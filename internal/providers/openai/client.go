package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/providers"
)

type Client struct {
	cfg             config.OpenAIProviderConfig
	httpClient      *http.Client
	rand            *rand.Rand
	resolveToken    func(context.Context) (string, error)
	accountFailover func(context.Context, error) (string, bool, error)
	accountSuccess  func(context.Context)
}

type requester interface {
	Do(req *http.Request) (*http.Response, error)
}

const (
	defaultAPIBaseURL   = "https://api.openai.com/v1"
	defaultCodexBaseURL = "https://chatgpt.com/backend-api"
)

func New(cfg config.OpenAIProviderConfig) *Client {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 90 * time.Second
	}

	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
		resolveToken: func(ctx context.Context) (string, error) {
			_ = ctx
			return "", nil
		},
		accountFailover: func(ctx context.Context, err error) (string, bool, error) {
			_, _ = ctx, err
			return "", false, nil
		},
		accountSuccess: func(ctx context.Context) {
			_ = ctx
		},
	}
}

func (c *Client) SetTokenResolver(resolver func(context.Context) (string, error)) {
	if resolver == nil {
		return
	}
	c.resolveToken = resolver
}

func (c *Client) SetAccountFailoverHandler(handler func(context.Context, error) (string, bool, error)) {
	if handler == nil {
		return
	}
	c.accountFailover = handler
}

func (c *Client) SetAccountSuccessHandler(handler func(context.Context)) {
	if handler == nil {
		return
	}
	c.accountSuccess = handler
}

func (c *Client) Name() string {
	return "openai"
}

func (c *Client) Validate(ctx context.Context) error {
	key, err := c.resolveAuthToken(ctx)
	if err != nil {
		return err
	}
	mode := c.authMode()
	if mode == "account" {
		if _, accountErr := extractAccountID(key); accountErr != nil {
			return &providers.Error{Code: providers.ErrAuthError, Message: "invalid account token", Cause: accountErr}
		}
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.modelsURL(mode), nil)
	if err != nil {
		return err
	}
	if err := c.applyAuthHeaders(req, mode, key); err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return mapHTTPError(err, 0, "validate")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return mapStatusError(resp.StatusCode, string(body), "validate")
	}

	return nil
}

func (c *Client) Chat(ctx context.Context, req providers.ChatRequest) (providers.ChatResponse, error) {
	return c.chatWithDoer(ctx, req, c.httpClient)
}

func (c *Client) Stream(ctx context.Context, req providers.ChatRequest) (<-chan providers.StreamEvent, <-chan error) {
	if c.authMode() == "account" {
		return c.streamWithDoer(ctx, req, c.httpClient)
	}

	stream := make(chan providers.StreamEvent, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(stream)
		defer close(errCh)

		resp, err := c.Chat(ctx, req)
		if err != nil {
			errCh <- err
			return
		}

		stream <- providers.StreamEvent{Type: "token", Text: resp.Text}
		stream <- providers.StreamEvent{Type: "done", Metadata: map[string]string{"finish_reason": resp.FinishReason}}
	}()

	return stream, errCh
}

func (c *Client) streamWithDoer(ctx context.Context, req providers.ChatRequest, doer requester) (<-chan providers.StreamEvent, <-chan error) {
	stream := make(chan providers.StreamEvent, 32)
	errCh := make(chan error, 1)

	go func() {
		defer close(stream)
		defer close(errCh)

		mode := c.authMode()
		key, err := c.resolveAuthToken(ctx)
		if err != nil {
			errCh <- err
			return
		}

		model := strings.TrimSpace(req.Model)
		if model == "" {
			model = c.defaultModel(req.Role)
		}
		if model == "" {
			errCh <- &providers.Error{Code: providers.ErrModelUnavailable, Message: fmt.Sprintf("model not configured for role %s", req.Role)}
			return
		}

		payload := map[string]any{
			"model": model,
			"input": []map[string]any{{
				"type":    "message",
				"role":    "user",
				"content": req.UserPrompt,
			}},
			"store":  false,
			"stream": true,
		}
		if strings.TrimSpace(req.SystemPrompt) != "" {
			payload["instructions"] = req.SystemPrompt
		}
		if effort := strings.TrimSpace(req.ReasoningEffort); effort != "" {
			payload["reasoning"] = map[string]any{"effort": effort}
		} else if effort := strings.TrimSpace(c.cfg.ReasoningEffort); effort != "" {
			payload["reasoning"] = map[string]any{"effort": effort}
		}

		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			errCh <- err
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.responsesURL(mode), bytes.NewReader(bodyBytes))
		if err != nil {
			errCh <- err
			return
		}
		if err := c.applyAuthHeaders(httpReq, mode, key); err != nil {
			errCh <- err
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")

		httpResp, err := doer.Do(httpReq)
		if err != nil {
			errCh <- mapHTTPError(err, 0, "chat")
			return
		}
		defer httpResp.Body.Close()
		if httpResp.StatusCode >= 300 {
			data, _ := io.ReadAll(httpResp.Body)
			errCh <- mapStatusError(httpResp.StatusCode, string(data), "chat")
			return
		}

		if err := streamSSEResponse(httpResp.Body, func(eventType, payload string) error {
			return emitSSEEvent(stream, eventType, payload)
		}); err != nil {
			errCh <- &providers.Error{Code: providers.ErrInvalidResponse, Message: "failed to parse provider response", Cause: err}
			return
		}
	}()

	return stream, errCh
}

func (c *Client) chatWithDoer(ctx context.Context, req providers.ChatRequest, doer requester) (providers.ChatResponse, error) {
	mode := c.authMode()
	key, err := c.resolveAuthToken(ctx)
	if err != nil {
		return providers.ChatResponse{}, err
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = c.defaultModel(req.Role)
	}
	if model == "" {
		return providers.ChatResponse{}, &providers.Error{Code: providers.ErrModelUnavailable, Message: fmt.Sprintf("model not configured for role %s", req.Role)}
	}

	payload := map[string]any{
		"model": model,
	}
	if mode == "account" {
		payload["input"] = []map[string]any{{
			"type":    "message",
			"role":    "user",
			"content": req.UserPrompt,
		}}
		payload["store"] = false
		payload["stream"] = true
	} else {
		payload["input"] = req.UserPrompt
	}
	if strings.TrimSpace(req.SystemPrompt) != "" {
		payload["instructions"] = req.SystemPrompt
	}
	if effort := strings.TrimSpace(req.ReasoningEffort); effort != "" {
		payload["reasoning"] = map[string]any{"effort": effort}
	} else if effort := strings.TrimSpace(c.cfg.ReasoningEffort); effort != "" {
		payload["reasoning"] = map[string]any{"effort": effort}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return providers.ChatResponse{}, err
	}

	attempts := c.cfg.MaxRetries
	if attempts <= 0 {
		attempts = 3
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		httpReq, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, c.responsesURL(mode), bytes.NewReader(bodyBytes))
		if reqErr != nil {
			return providers.ChatResponse{}, reqErr
		}
		if err := c.applyAuthHeaders(httpReq, mode, key); err != nil {
			return providers.ChatResponse{}, err
		}
		httpReq.Header.Set("Content-Type", "application/json")

		httpResp, doErr := doer.Do(httpReq)
		if doErr != nil {
			mapped := mapHTTPError(doErr, 0, "chat")
			lastErr = mapped
			if nextToken, switched, switchErr := c.maybeFailoverAccount(ctx, mode, mapped); switchErr != nil {
				return providers.ChatResponse{}, switchErr
			} else if switched {
				key = nextToken
				continue
			}
			if isRetryable(mapped) && attempt < attempts {
				c.sleepBackoff(attempt)
				continue
			}
			return providers.ChatResponse{}, mapped
		}

		data, readErr := io.ReadAll(httpResp.Body)
		_ = httpResp.Body.Close()
		if readErr != nil {
			lastErr = &providers.Error{Code: providers.ErrInvalidResponse, Message: "failed to read provider response", Cause: readErr}
			if attempt < attempts {
				c.sleepBackoff(attempt)
				continue
			}
			return providers.ChatResponse{}, lastErr
		}

		if httpResp.StatusCode >= 300 {
			mapped := mapStatusError(httpResp.StatusCode, string(data), "chat")
			lastErr = mapped
			if nextToken, switched, switchErr := c.maybeFailoverAccount(ctx, mode, mapped); switchErr != nil {
				return providers.ChatResponse{}, switchErr
			} else if switched {
				key = nextToken
				continue
			}
			if isRetryable(mapped) && attempt < attempts {
				c.sleepBackoff(attempt)
				continue
			}
			return providers.ChatResponse{}, mapped
		}

		parsed, parseErr := parseProviderResponse(httpResp, data, mode)
		if parseErr != nil {
			lastErr = &providers.Error{Code: providers.ErrInvalidResponse, Message: "failed to parse provider response", Cause: parseErr}
			if attempt < attempts {
				c.sleepBackoff(attempt)
				continue
			}
			return providers.ChatResponse{}, lastErr
		}

		parsed.ProviderMetadata = map[string]string{
			"provider": "openai",
			"model":    model,
		}
		if mode == "account" {
			if accountID, accountErr := extractAccountID(key); accountErr == nil {
				parsed.ProviderMetadata["account_id"] = accountID
			}
			c.accountSuccess(ctx)
		}
		return parsed, nil
	}

	if lastErr == nil {
		lastErr = &providers.Error{Code: providers.ErrTransient, Message: "provider call failed"}
	}

	return providers.ChatResponse{}, lastErr
}

func (c *Client) maybeFailoverAccount(ctx context.Context, mode string, err error) (string, bool, error) {
	if mode != "account" || !isAccountFailoverEligible(err) {
		return "", false, nil
	}
	return c.accountFailover(ctx, err)
}

func isAccountFailoverEligible(err error) bool {
	pe, ok := err.(*providers.Error)
	if !ok {
		return false
	}
	switch pe.Code {
	case providers.ErrRateLimited, providers.ErrAuthError, providers.ErrModelUnavailable:
		return true
	default:
		return false
	}
}

func AccountFailoverCooldown(err error) time.Duration {
	pe, ok := err.(*providers.Error)
	if !ok {
		return 0
	}
	switch pe.Code {
	case providers.ErrRateLimited:
		return 2 * time.Minute
	case providers.ErrAuthError:
		return 15 * time.Minute
	case providers.ErrModelUnavailable:
		return 10 * time.Minute
	default:
		return 0
	}
}

func (c *Client) defaultModel(role providers.Role) string {
	switch role {
	case providers.RolePlanner:
		return c.cfg.Models.Planner
	case providers.RoleCoder:
		return c.cfg.Models.Coder
	case providers.RoleReviewer:
		return c.cfg.Models.Reviewer
	default:
		return ""
	}
}

type responsesAPI struct {
	Output []struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	OutputText string `json:"output_text"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Status string `json:"status"`
}

func parseResponse(data []byte) (providers.ChatResponse, error) {
	var raw responsesAPI
	if err := json.Unmarshal(data, &raw); err != nil {
		return providers.ChatResponse{}, err
	}

	text := strings.TrimSpace(raw.OutputText)
	if text == "" {
		var b strings.Builder
		for _, out := range raw.Output {
			for _, c := range out.Content {
				if strings.TrimSpace(c.Text) != "" {
					if b.Len() > 0 {
						b.WriteString("\n")
					}
					b.WriteString(c.Text)
				}
			}
		}
		text = strings.TrimSpace(b.String())
	}

	if text == "" {
		return providers.ChatResponse{}, fmt.Errorf("empty output text")
	}

	return providers.ChatResponse{
		Text:         text,
		FinishReason: raw.Status,
		Usage: providers.Usage{
			InputTokens:  raw.Usage.InputTokens,
			OutputTokens: raw.Usage.OutputTokens,
			TotalTokens:  raw.Usage.TotalTokens,
		},
	}, nil
}

func parseProviderResponse(resp *http.Response, data []byte, mode string) (providers.ChatResponse, error) {
	contentType := ""
	if resp != nil {
		contentType = strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	}
	trimmed := bytes.TrimSpace(data)
	if mode == "account" && (strings.Contains(contentType, "text/event-stream") || bytes.HasPrefix(trimmed, []byte("data:")) || bytes.HasPrefix(trimmed, []byte("event:"))) {
		return parseSSEResponse(trimmed)
	}
	return parseResponse(data)
}

func parseSSEResponse(data []byte) (providers.ChatResponse, error) {
	var textBuilder strings.Builder
	var usage providers.Usage
	finishReason := "completed"
	finalText := ""
	currentEvent := ""
	dataLines := []string{}

	processEvent := func(eventName string, payload string) error {
		payload = strings.TrimSpace(payload)
		if payload == "" || payload == "[DONE]" {
			return nil
		}
		if !strings.HasPrefix(payload, "{") {
			return nil
		}

		event := map[string]any{}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return nil
		}
		eventType, _ := event["type"].(string)
		if eventType == "" {
			eventType = eventName
		}

		switch eventType {
		case "response.output_text.delta":
			if delta, ok := event["delta"].(string); ok {
				textBuilder.WriteString(delta)
			}
		case "response.completed", "response.done":
			response, _ := event["response"].(map[string]any)
			if response != nil {
				if parsedUsage, ok := parseUsageMap(response["usage"]); ok {
					usage = parsedUsage
				}
				if status, ok := response["status"].(string); ok && strings.TrimSpace(status) != "" {
					finishReason = status
				}
				if text := extractResponseOutputText(response["output"]); strings.TrimSpace(text) != "" {
					finalText = text
				}
			}
		case "response.output_item.done":
			item, _ := event["item"].(map[string]any)
			if item != nil {
				if itemText := extractResponseOutputText([]any{item}); strings.TrimSpace(itemText) != "" {
					finalText = itemText
				}
			}
		}
		return nil
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if err := processEvent(currentEvent, strings.Join(dataLines, "\n")); err != nil {
				return providers.ChatResponse{}, err
			}
			currentEvent = ""
			dataLines = dataLines[:0]
			continue
		}
		if strings.HasPrefix(trimmed, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
			continue
		}
		if strings.HasPrefix(trimmed, "data:") {
			dataLine := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			if currentEvent == "" {
				if err := processEvent("", dataLine); err != nil {
					return providers.ChatResponse{}, err
				}
				continue
			}
			dataLines = append(dataLines, dataLine)
			continue
		}
	}
	if err := processEvent(currentEvent, strings.Join(dataLines, "\n")); err != nil {
		return providers.ChatResponse{}, err
	}

	text := strings.TrimSpace(finalText)
	if text == "" {
		text = strings.TrimSpace(textBuilder.String())
	}
	if text == "" {
		return providers.ChatResponse{}, fmt.Errorf("empty output text")
	}

	return providers.ChatResponse{
		Text:         text,
		FinishReason: finishReason,
		Usage:        usage,
	}, nil
}

func streamSSEResponse(body io.Reader, handler func(eventType, payload string) error) error {
	scanner := bufio.NewScanner(body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	currentEvent := ""
	dataLines := []string{}
	flush := func() error {
		payload := strings.TrimSpace(strings.Join(dataLines, "\n"))
		dataLines = dataLines[:0]
		if payload == "" {
			currentEvent = ""
			return nil
		}
		if err := handler(currentEvent, payload); err != nil {
			return err
		}
		currentEvent = ""
		return nil
	}
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(trimmed, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
			continue
		}
		if strings.HasPrefix(trimmed, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(trimmed, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}

func emitSSEEvent(stream chan<- providers.StreamEvent, eventType, payload string) error {
	payload = strings.TrimSpace(payload)
	if payload == "" || payload == "[DONE]" {
		return nil
	}
	if !strings.HasPrefix(payload, "{") {
		return nil
	}
	parsed := map[string]any{}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return nil
	}
	if t, _ := parsed["type"].(string); t != "" {
		eventType = t
	}
	switch eventType {
	case "response.output_text.delta":
		if delta, ok := parsed["delta"].(string); ok && delta != "" {
			stream <- providers.StreamEvent{Type: "token", Text: delta}
		}
	case "response.completed", "response.done":
		response, _ := parsed["response"].(map[string]any)
		metadata := map[string]string{}
		if response != nil {
			if status, _ := response["status"].(string); strings.TrimSpace(status) != "" {
				metadata["finish_reason"] = strings.TrimSpace(status)
			}
		}
		stream <- providers.StreamEvent{Type: "done", Metadata: metadata}
	}
	return nil
}

func parseUsageMap(value any) (providers.Usage, bool) {
	usageMap, ok := value.(map[string]any)
	if !ok {
		return providers.Usage{}, false
	}
	inputTokens := intFromAny(usageMap["input_tokens"])
	outputTokens := intFromAny(usageMap["output_tokens"])
	totalTokens := intFromAny(usageMap["total_tokens"])
	if totalTokens == 0 {
		totalTokens = inputTokens + outputTokens
	}
	return providers.Usage{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  totalTokens,
	}, true
}

func extractResponseOutputText(value any) string {
	items, ok := value.([]any)
	if !ok {
		return ""
	}
	var b strings.Builder
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if itemType, _ := itemMap["type"].(string); itemType != "message" {
			continue
		}
		content, ok := itemMap["content"].([]any)
		if !ok {
			continue
		}
		for _, part := range content {
			partMap, ok := part.(map[string]any)
			if !ok {
				continue
			}
			partType, _ := partMap["type"].(string)
			if partType != "output_text" && partType != "text" {
				continue
			}
			if text, _ := partMap["text"].(string); strings.TrimSpace(text) != "" {
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString(text)
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func (c *Client) sleepBackoff(attempt int) {
	base := time.Duration(250*(1<<(attempt-1))) * time.Millisecond
	jitter := time.Duration(c.rand.Intn(200)) * time.Millisecond
	time.Sleep(base + jitter)
}

func mapHTTPError(err error, status int, op string) error {
	if strings.Contains(strings.ToLower(err.Error()), "timeout") {
		return &providers.Error{Code: providers.ErrTimeout, Message: fmt.Sprintf("%s timeout", op), Cause: err}
	}
	if status >= 500 {
		return &providers.Error{Code: providers.ErrTransient, Message: fmt.Sprintf("%s transient failure", op), Cause: err}
	}
	return &providers.Error{Code: providers.ErrTransient, Message: fmt.Sprintf("%s request failed", op), Cause: err}
}

func mapStatusError(status int, body, op string) error {
	trimmed := strings.TrimSpace(body)
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return &providers.Error{Code: providers.ErrAuthError, Message: fmt.Sprintf("%s unauthorized", op), Cause: fmt.Errorf("status=%d body=%s", status, trimmed)}
	case status == http.StatusTooManyRequests:
		return &providers.Error{Code: providers.ErrRateLimited, Message: fmt.Sprintf("%s rate limited", op), Cause: fmt.Errorf("status=%d body=%s", status, trimmed)}
	case status == http.StatusNotFound:
		return &providers.Error{Code: providers.ErrModelUnavailable, Message: fmt.Sprintf("%s model unavailable", op), Cause: fmt.Errorf("status=%d body=%s", status, trimmed)}
	case status >= 500:
		return &providers.Error{Code: providers.ErrTransient, Message: fmt.Sprintf("%s transient error", op), Cause: fmt.Errorf("status=%d body=%s", status, trimmed)}
	default:
		return &providers.Error{Code: providers.ErrInvalidResponse, Message: fmt.Sprintf("%s invalid response", op), Cause: fmt.Errorf("status=%d body=%s", status, trimmed)}
	}
}

func isRetryable(err error) bool {
	pe, ok := err.(*providers.Error)
	if !ok {
		return false
	}
	switch pe.Code {
	case providers.ErrRateLimited, providers.ErrTimeout, providers.ErrTransient:
		return true
	default:
		return false
	}
}

func (c *Client) resolveAuthToken(ctx context.Context) (string, error) {
	mode := c.authMode()
	if mode == "" {
		mode = "api_key"
	}

	switch mode {
	case "api_key":
		key := strings.TrimSpace(os.Getenv(c.cfg.APIKeyEnv))
		if key == "" && c.resolveToken != nil {
			resolved, err := c.resolveToken(ctx)
			if err != nil {
				return "", &providers.Error{Code: providers.ErrAuthError, Message: "failed to resolve api key from local auth state", Cause: err}
			}
			key = strings.TrimSpace(resolved)
		}
		if key == "" {
			return "", &providers.Error{Code: providers.ErrAuthError, Message: fmt.Sprintf("missing API key in env var %s and local auth state", c.cfg.APIKeyEnv)}
		}
		return key, nil
	case "account":
		if env := strings.TrimSpace(os.Getenv(c.cfg.AccountTokenEnv)); env != "" {
			return env, nil
		}
		if c.resolveToken != nil {
			token, err := c.resolveToken(ctx)
			if err != nil {
				return "", &providers.Error{Code: providers.ErrAuthError, Message: "failed to resolve account token", Cause: err}
			}
			if strings.TrimSpace(token) != "" {
				return strings.TrimSpace(token), nil
			}
		}
		return "", &providers.Error{Code: providers.ErrAuthError, Message: fmt.Sprintf("missing account token in env var %s and local auth state", c.cfg.AccountTokenEnv)}
	default:
		return "", &providers.Error{Code: providers.ErrAuthError, Message: fmt.Sprintf("unsupported auth mode: %s", mode)}
	}
}

func (c *Client) authMode() string {
	mode := strings.ToLower(strings.TrimSpace(c.cfg.AuthMode))
	if mode == "" {
		return "api_key"
	}
	return mode
}

func (c *Client) baseURLForMode(mode string) string {
	base := strings.TrimSpace(c.cfg.BaseURL)
	base = strings.TrimRight(base, "/")
	if mode == "account" {
		if base == "" || strings.EqualFold(base, strings.TrimRight(defaultAPIBaseURL, "/")) {
			return defaultCodexBaseURL
		}
		return base
	}
	if base == "" {
		return defaultAPIBaseURL
	}
	return base
}

func (c *Client) modelsURL(mode string) string {
	base := c.baseURLForMode(mode)
	if strings.HasSuffix(base, "/models") {
		return base
	}
	return base + "/models"
}

func (c *Client) responsesURL(mode string) string {
	base := c.baseURLForMode(mode)
	if mode == "account" {
		switch {
		case strings.HasSuffix(base, "/codex/responses"):
			return base
		case strings.HasSuffix(base, "/codex"):
			return base + "/responses"
		default:
			return base + "/codex/responses"
		}
	}
	if strings.HasSuffix(base, "/responses") {
		return base
	}
	return base + "/responses"
}

func (c *Client) applyAuthHeaders(req *http.Request, mode, token string) error {
	req.Header.Set("Authorization", "Bearer "+token)
	if mode != "account" {
		return nil
	}

	accountID, err := extractAccountID(token)
	if err != nil {
		return &providers.Error{Code: providers.ErrAuthError, Message: "failed to extract account id from oauth token", Cause: err}
	}
	req.Header.Set("ChatGPT-Account-Id", accountID)
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("originator", "orch")
	req.Header.Set("Accept", "text/event-stream")
	return nil
}

func extractAccountID(token string) (string, error) {
	token = strings.TrimSpace(token)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("token is not a jwt")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return "", fmt.Errorf("failed to decode jwt payload: %w", err)
		}
	}

	claims := map[string]any{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("failed to parse jwt payload: %w", err)
	}

	if id, ok := claims["chatgpt_account_id"].(string); ok && strings.TrimSpace(id) != "" {
		return strings.TrimSpace(id), nil
	}
	if nested, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
		if id, ok := nested["chatgpt_account_id"].(string); ok && strings.TrimSpace(id) != "" {
			return strings.TrimSpace(id), nil
		}
	}
	if organizations, ok := claims["organizations"].([]any); ok && len(organizations) > 0 {
		if org, ok := organizations[0].(map[string]any); ok {
			if id, ok := org["id"].(string); ok && strings.TrimSpace(id) != "" {
				return strings.TrimSpace(id), nil
			}
		}
	}

	return "", fmt.Errorf("chatgpt_account_id claim not found")
}
