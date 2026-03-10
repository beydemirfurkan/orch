package openai

import (
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
	cfg          config.OpenAIProviderConfig
	httpClient   *http.Client
	rand         *rand.Rand
	resolveToken func(context.Context) (string, error)
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
	}
}

func (c *Client) SetTokenResolver(resolver func(context.Context) (string, error)) {
	if resolver == nil {
		return
	}
	c.resolveToken = resolver
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

func (c *Client) chatWithDoer(ctx context.Context, req providers.ChatRequest, doer requester) (providers.ChatResponse, error) {
	key, err := c.resolveAuthToken(ctx)
	if err != nil {
		return providers.ChatResponse{}, err
	}
	mode := c.authMode()

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = c.defaultModel(req.Role)
	}
	if model == "" {
		return providers.ChatResponse{}, &providers.Error{Code: providers.ErrModelUnavailable, Message: fmt.Sprintf("model not configured for role %s", req.Role)}
	}

	payload := map[string]any{
		"model": model,
		"input": buildInput(req.SystemPrompt, req.UserPrompt),
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
			if isRetryable(mapped) && attempt < attempts {
				c.sleepBackoff(attempt)
				continue
			}
			return providers.ChatResponse{}, mapped
		}

		parsed, parseErr := parseResponse(data)
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
		return parsed, nil
	}

	if lastErr == nil {
		lastErr = &providers.Error{Code: providers.ErrTransient, Message: "provider call failed"}
	}

	return providers.ChatResponse{}, lastErr
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

func buildInput(system, user string) []map[string]any {
	parts := make([]map[string]any, 0, 2)
	if strings.TrimSpace(system) != "" {
		parts = append(parts, map[string]any{
			"role": "system",
			"content": []map[string]string{
				{"type": "input_text", "text": system},
			},
		})
	}
	parts = append(parts, map[string]any{
		"role": "user",
		"content": []map[string]string{
			{"type": "input_text", "text": user},
		},
	})
	return parts
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
