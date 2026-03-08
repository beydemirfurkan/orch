package openai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/providers"
)

type sequenceDoer struct {
	responses []*http.Response
	index     int
}

func (d *sequenceDoer) Do(req *http.Request) (*http.Response, error) {
	if d.index >= len(d.responses) {
		return nil, fmt.Errorf("no response configured")
	}
	resp := d.responses[d.index]
	d.index++
	return resp, nil
}

func TestChatRetriesOnRateLimit(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	client := New(config.OpenAIProviderConfig{
		APIKeyEnv:      "OPENAI_API_KEY",
		BaseURL:        "https://example.test/v1",
		TimeoutSeconds: 5,
		MaxRetries:     2,
		Models: config.ProviderRoleModels{
			Coder: "gpt-5.3-codex",
		},
	})

	doer := &sequenceDoer{responses: []*http.Response{
		response(http.StatusTooManyRequests, `{"error":"rate"}`),
		response(http.StatusOK, `{"output_text":"done","status":"completed","usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`),
	}}

	out, err := client.chatWithDoer(context.Background(), providers.ChatRequest{Role: providers.RoleCoder}, doer)
	if err != nil {
		t.Fatalf("chat should succeed after retry: %v", err)
	}
	if strings.TrimSpace(out.Text) != "done" {
		t.Fatalf("unexpected text: %q", out.Text)
	}
	if doer.index != 2 {
		t.Fatalf("expected 2 attempts, got %d", doer.index)
	}
}

func TestValidateMissingKey(t *testing.T) {
	_ = os.Unsetenv("OPENAI_API_KEY")
	client := New(config.OpenAIProviderConfig{APIKeyEnv: "OPENAI_API_KEY", BaseURL: "https://example.test/v1"})
	err := client.Validate(context.Background())
	if err == nil {
		t.Fatalf("expected validate error")
	}
	perr, ok := err.(*providers.Error)
	if !ok {
		t.Fatalf("expected provider error type")
	}
	if perr.Code != providers.ErrAuthError {
		t.Fatalf("unexpected error code: %s", perr.Code)
	}
}

func TestMapStatusError(t *testing.T) {
	err := mapStatusError(http.StatusUnauthorized, "bad", "chat")
	perr, ok := err.(*providers.Error)
	if !ok {
		t.Fatalf("expected provider error")
	}
	if perr.Code != providers.ErrAuthError {
		t.Fatalf("unexpected code: %s", perr.Code)
	}
}

func TestResolveAuthTokenAccountModeWithResolver(t *testing.T) {
	client := New(config.OpenAIProviderConfig{
		AuthMode:        "account",
		AccountTokenEnv: "OPENAI_ACCOUNT_TOKEN",
	})
	client.SetTokenResolver(func(ctx context.Context) (string, error) {
		return "account-token", nil
	})

	token, err := client.resolveAuthToken(context.Background())
	if err != nil {
		t.Fatalf("resolve token: %v", err)
	}
	if token != "account-token" {
		t.Fatalf("unexpected token: %s", token)
	}
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
