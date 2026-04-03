// Package anthropic implements the providers.Provider interface for the Anthropic Messages API.
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/furkanbeydemir/orch/internal/providers"
)

const (
	defaultBaseURL    = "https://api.anthropic.com/v1"
	anthropicVersion  = "2023-06-01"
	defaultTimeout    = 120 * time.Second
	defaultMaxRetries = 3
)

// Config holds Anthropic-specific provider configuration.
type Config struct {
	APIKeyEnv      string
	BaseURL        string
	TimeoutSeconds int
	MaxRetries     int
}

// Client implements providers.Provider for the Anthropic Messages API.
type Client struct {
	cfg        Config
	httpClient *http.Client
}

func New(cfg Config) *Client {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.APIKeyEnv == "" {
		cfg.APIKeyEnv = "ANTHROPIC_API_KEY"
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = defaultMaxRetries
	}
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) Name() string { return "anthropic" }

func (c *Client) Validate(ctx context.Context) error {
	key := c.apiKey()
	if key == "" {
		return fmt.Errorf("anthropic: %s is not set", c.cfg.APIKeyEnv)
	}
	return nil
}

func (c *Client) Chat(ctx context.Context, req providers.ChatRequest) (providers.ChatResponse, error) {
	key := c.apiKey()
	if key == "" {
		return providers.ChatResponse{}, fmt.Errorf("anthropic: %s is not set", c.cfg.APIKeyEnv)
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = "claude-sonnet-4-5"
	}

	body := c.buildRequestBody(req, model)
	data, err := json.Marshal(body)
	if err != nil {
		return providers.ChatResponse{}, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/messages", bytes.NewReader(data))
	if err != nil {
		return providers.ChatResponse{}, fmt.Errorf("anthropic: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", key)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.doWithRetry(httpReq, data)
	if err != nil {
		return providers.ChatResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return providers.ChatResponse{}, fmt.Errorf("anthropic: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return providers.ChatResponse{}, fmt.Errorf("anthropic: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return parseResponse(respBody)
}

func (c *Client) Stream(ctx context.Context, req providers.ChatRequest) (<-chan providers.StreamEvent, <-chan error) {
	events := make(chan providers.StreamEvent, 1)
	errs := make(chan error, 1)
	go func() {
		defer close(events)
		defer close(errs)
		resp, err := c.Chat(ctx, req)
		if err != nil {
			errs <- err
			return
		}
		events <- providers.StreamEvent{Type: "text", Text: resp.Text}
	}()
	return events, errs
}

// ---- internal helpers ----

func (c *Client) apiKey() string {
	env := c.cfg.APIKeyEnv
	if env == "" {
		env = "ANTHROPIC_API_KEY"
	}
	return strings.TrimSpace(os.Getenv(env))
}

type requestBody struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseBody struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Role    string          `json:"role"`
	Content []contentBlock  `json:"content"`
	Model   string          `json:"model"`
	Usage   responseUsage   `json:"usage"`
	StopReason string       `json:"stop_reason"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type responseUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func (c *Client) buildRequestBody(req providers.ChatRequest, model string) requestBody {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	msgs := []message{
		{Role: "user", Content: req.UserPrompt},
	}

	return requestBody{
		Model:     model,
		MaxTokens: maxTokens,
		System:    req.SystemPrompt,
		Messages:  msgs,
	}
}

func parseResponse(data []byte) (providers.ChatResponse, error) {
	var rb responseBody
	if err := json.Unmarshal(data, &rb); err != nil {
		return providers.ChatResponse{}, fmt.Errorf("anthropic: parse response: %w", err)
	}

	var text string
	for _, block := range rb.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}

	return providers.ChatResponse{
		Text:         text,
		FinishReason: rb.StopReason,
		Usage: providers.Usage{
			InputTokens:  rb.Usage.InputTokens,
			OutputTokens: rb.Usage.OutputTokens,
			TotalTokens:  rb.Usage.InputTokens + rb.Usage.OutputTokens,
		},
	}, nil
}

func (c *Client) doWithRetry(req *http.Request, body []byte) (*http.Response, error) {
	maxRetries := c.cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Clone the request with a fresh body for retries.
			clone := req.Clone(req.Context())
			clone.Body = io.NopCloser(bytes.NewReader(body))
			req = clone
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Retry on 429 or 5xx.
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}
	return nil, fmt.Errorf("anthropic: all retries exhausted: %w", lastErr)
}
