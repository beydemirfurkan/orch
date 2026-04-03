// Package ollama implements the providers.Provider interface for a local Ollama instance.
// Ollama exposes an OpenAI-compatible /api/chat endpoint — no auth required.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/furkanbeydemir/orch/internal/providers"
)

const (
	defaultBaseURL = "http://localhost:11434"
	defaultTimeout = 180 * time.Second
)

// Config holds Ollama-specific provider configuration.
type Config struct {
	// BaseURL defaults to http://localhost:11434
	BaseURL        string
	TimeoutSeconds int
}

// Client implements providers.Provider for a local Ollama instance.
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
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) Name() string { return "ollama" }

func (c *Client) Validate(ctx context.Context) error {
	// Probe the Ollama API to verify it's reachable.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("ollama: build probe request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama: server unreachable at %s: %w", c.cfg.BaseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama: unexpected status %d from probe", resp.StatusCode)
	}
	return nil
}

func (c *Client) Chat(ctx context.Context, req providers.ChatRequest) (providers.ChatResponse, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = "llama3"
	}

	msgs := []ollamaMessage{
		{Role: "system", Content: req.SystemPrompt},
		{Role: "user", Content: req.UserPrompt},
	}

	body := ollamaRequest{
		Model:    model,
		Messages: msgs,
		Stream:   false,
		Options:  ollamaOptions{NumPredict: req.MaxTokens},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return providers.ChatResponse{}, fmt.Errorf("ollama: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return providers.ChatResponse{}, fmt.Errorf("ollama: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return providers.ChatResponse{}, fmt.Errorf("ollama: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return providers.ChatResponse{}, fmt.Errorf("ollama: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return providers.ChatResponse{}, fmt.Errorf("ollama: HTTP %d: %s", resp.StatusCode, string(respBody))
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

// ---- request/response types ----

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	NumPredict int `json:"num_predict,omitempty"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  ollamaOptions   `json:"options,omitempty"`
}

type ollamaResponse struct {
	Model   string        `json:"model"`
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
	// Ollama returns token counts in eval_count / prompt_eval_count.
	PromptEvalCount int `json:"prompt_eval_count"`
	EvalCount       int `json:"eval_count"`
}

func parseResponse(data []byte) (providers.ChatResponse, error) {
	var rb ollamaResponse
	if err := json.Unmarshal(data, &rb); err != nil {
		return providers.ChatResponse{}, fmt.Errorf("ollama: parse response: %w", err)
	}

	total := rb.PromptEvalCount + rb.EvalCount
	return providers.ChatResponse{
		Text: rb.Message.Content,
		Usage: providers.Usage{
			InputTokens:  rb.PromptEvalCount,
			OutputTokens: rb.EvalCount,
			TotalTokens:  total,
		},
	}, nil
}
