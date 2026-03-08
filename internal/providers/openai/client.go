package openai

import (
	"bytes"
	"context"
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
	cfg        config.OpenAIProviderConfig
	httpClient *http.Client
	rand       *rand.Rand
}

type requester interface {
	Do(req *http.Request) (*http.Response, error)
}

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
	}
}

func (c *Client) Name() string {
	return "openai"
}

func (c *Client) Validate(ctx context.Context) error {
	key := strings.TrimSpace(os.Getenv(c.cfg.APIKeyEnv))
	if key == "" {
		return &providers.Error{Code: providers.ErrAuthError, Message: fmt.Sprintf("missing API key in env var %s", c.cfg.APIKeyEnv)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.cfg.BaseURL, "/")+"/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)

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
	key := strings.TrimSpace(os.Getenv(c.cfg.APIKeyEnv))
	if key == "" {
		return providers.ChatResponse{}, &providers.Error{Code: providers.ErrAuthError, Message: fmt.Sprintf("missing API key in env var %s", c.cfg.APIKeyEnv)}
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
		httpReq, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.BaseURL, "/")+"/responses", bytes.NewReader(bodyBytes))
		if reqErr != nil {
			return providers.ChatResponse{}, reqErr
		}
		httpReq.Header.Set("Authorization", "Bearer "+key)
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
