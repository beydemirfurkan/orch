package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
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

type inspectDoer struct {
	fn func(req *http.Request) (*http.Response, error)
}

func (d *inspectDoer) Do(req *http.Request) (*http.Response, error) {
	if d.fn == nil {
		return nil, fmt.Errorf("no inspect fn configured")
	}
	return d.fn(req)
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

func TestChatAccountModeFailsOverToSecondToken(t *testing.T) {
	client := New(config.OpenAIProviderConfig{
		AuthMode:        "account",
		BaseURL:         "https://api.openai.com/v1",
		AccountTokenEnv: "OPENAI_ACCOUNT_TOKEN",
		MaxRetries:      2,
		Models:          config.ProviderRoleModels{Coder: "gpt-5.3-codex"},
	})
	firstToken := testAccountToken("acc-1")
	secondToken := testAccountToken("acc-2")
	client.SetTokenResolver(func(ctx context.Context) (string, error) {
		return firstToken, nil
	})
	client.SetAccountFailoverHandler(func(ctx context.Context, err error) (string, bool, error) {
		return secondToken, true, nil
	})
	markedSuccess := false
	client.SetAccountSuccessHandler(func(ctx context.Context) {
		markedSuccess = true
	})

	attempts := 0
	doer := &inspectDoer{fn: func(req *http.Request) (*http.Response, error) {
		attempts++
		switch attempts {
		case 1:
			if got := req.Header.Get("ChatGPT-Account-Id"); got != "acc-1" {
				return nil, fmt.Errorf("expected first account header acc-1, got %s", got)
			}
			return response(http.StatusTooManyRequests, `{"error":"rate"}`), nil
		case 2:
			if got := req.Header.Get("ChatGPT-Account-Id"); got != "acc-2" {
				return nil, fmt.Errorf("expected second account header acc-2, got %s", got)
			}
			return response(http.StatusOK, `{"output_text":"done","status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`), nil
		default:
			return nil, fmt.Errorf("unexpected attempt %d", attempts)
		}
	}}

	out, err := client.chatWithDoer(context.Background(), providers.ChatRequest{Role: providers.RoleCoder}, doer)
	if err != nil {
		t.Fatalf("chat should succeed after failover: %v", err)
	}
	if out.ProviderMetadata["account_id"] != "acc-2" {
		t.Fatalf("expected account_id metadata acc-2, got %q", out.ProviderMetadata["account_id"])
	}
	if !markedSuccess {
		t.Fatalf("expected account success handler to be called")
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

func TestChatAccountModeUsesCodexEndpointAccountHeaderAndInstructions(t *testing.T) {
	client := New(config.OpenAIProviderConfig{
		AuthMode:        "account",
		BaseURL:         "https://api.openai.com/v1",
		AccountTokenEnv: "OPENAI_ACCOUNT_TOKEN",
		Models: config.ProviderRoleModels{
			Coder: "gpt-5.3-codex",
		},
	})
	client.SetTokenResolver(func(ctx context.Context) (string, error) {
		return testAccountToken("acc-123"), nil
	})

	doer := &inspectDoer{fn: func(req *http.Request) (*http.Response, error) {
		if got := req.URL.String(); got != "https://chatgpt.com/backend-api/codex/responses" {
			return nil, fmt.Errorf("unexpected request url: %s", got)
		}
		if got := req.Header.Get("ChatGPT-Account-Id"); got != "acc-123" {
			return nil, fmt.Errorf("unexpected account header: %s", got)
		}
		if got := req.Header.Get("Accept"); got != "text/event-stream" {
			return nil, fmt.Errorf("unexpected accept header: %s", got)
		}
		if got := req.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
			return nil, fmt.Errorf("missing auth header")
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("read request body: %w", err)
		}
		payload := map[string]any{}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("parse request body: %w", err)
		}
		if got := payload["instructions"]; got != "You are a constrained coding agent." {
			return nil, fmt.Errorf("unexpected instructions: %#v", got)
		}
		input, ok := payload["input"].([]any)
		if !ok {
			return nil, fmt.Errorf("expected account mode input list, got %#v", payload["input"])
		}
		if len(input) != 1 {
			return nil, fmt.Errorf("expected single input item, got %d", len(input))
		}
		message, ok := input[0].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected input item object, got %#v", input[0])
		}
		if got := message["type"]; got != "message" {
			return nil, fmt.Errorf("unexpected input item type: %#v", got)
		}
		if got := message["role"]; got != "user" {
			return nil, fmt.Errorf("unexpected input item role: %#v", got)
		}
		if got := message["content"]; got != "Return a diff." {
			return nil, fmt.Errorf("unexpected input content: %#v", got)
		}
		if got := payload["store"]; got != false {
			return nil, fmt.Errorf("unexpected store flag: %#v", got)
		}
		if got := payload["stream"]; got != true {
			return nil, fmt.Errorf("unexpected stream flag: %#v", got)
		}
		return sseResponse(http.StatusOK, strings.Join([]string{
			`data: {"type":"response.output_text.delta","delta":"done"}`,
			`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2},"output":[{"type":"message","content":[{"type":"output_text","text":"done"}]}]}}`,
			`data: [DONE]`,
		}, "\n")), nil
	}}

	out, err := client.chatWithDoer(context.Background(), providers.ChatRequest{
		Role:         providers.RoleCoder,
		SystemPrompt: "You are a constrained coding agent.",
		UserPrompt:   "Return a diff.",
	}, doer)
	if err != nil {
		t.Fatalf("chat error: %v", err)
	}
	if strings.TrimSpace(out.Text) != "done" {
		t.Fatalf("unexpected text: %q", out.Text)
	}
}

func TestStreamAccountModeEmitsTokenEvents(t *testing.T) {
	client := New(config.OpenAIProviderConfig{
		AuthMode:        "account",
		BaseURL:         "https://api.openai.com/v1",
		AccountTokenEnv: "OPENAI_ACCOUNT_TOKEN",
		Models: config.ProviderRoleModels{
			Coder: "gpt-5.3-codex",
		},
	})
	client.SetTokenResolver(func(ctx context.Context) (string, error) {
		return testAccountToken("acc-123"), nil
	})

	events, errCh := client.streamWithDoer(context.Background(), providers.ChatRequest{
		Role:         providers.RoleCoder,
		SystemPrompt: "You are a constrained coding agent.",
		UserPrompt:   "Say hello.",
	}, &inspectDoer{fn: func(req *http.Request) (*http.Response, error) {
		return sseResponse(http.StatusOK, strings.Join([]string{
			`event: response.output_text.delta`,
			`data: {"delta":"Hel"}`,
			``,
			`event: response.output_text.delta`,
			`data: {"delta":"lo"}`,
			``,
			`event: response.completed`,
			`data: {"response":{"status":"completed"}}`,
			``,
		}, "\n")), nil
	}})

	var chunks []string
	for event := range events {
		if event.Type == "token" {
			chunks = append(chunks, event.Text)
		}
	}
	for err := range errCh {
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
	}
	if strings.Join(chunks, "") != "Hello" {
		t.Fatalf("unexpected chunks: %#v", chunks)
	}
}

func TestParseProviderResponseTreatsEventPrefixedBodyAsSSE(t *testing.T) {
	resp := response(http.StatusOK, strings.Join([]string{
		`event: response.output_text.delta`,
		`data: {"delta":"Merhaba"}`,
		``,
		`event: response.completed`,
		`data: {"response":{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"Merhaba"}]}]}}`,
		``,
	}, "\n"))

	out, err := parseProviderResponse(resp, mustReadBody(t, resp), "account")
	if err != nil {
		t.Fatalf("parseProviderResponse error: %v", err)
	}
	if out.Text != "Merhaba" {
		t.Fatalf("unexpected text: %q", out.Text)
	}
}

func TestValidateAccountModeRejectsNonJWTToken(t *testing.T) {
	client := New(config.OpenAIProviderConfig{
		AuthMode:        "account",
		AccountTokenEnv: "OPENAI_ACCOUNT_TOKEN",
	})
	client.SetTokenResolver(func(ctx context.Context) (string, error) {
		return "not-a-jwt", nil
	})

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

func testAccountToken(accountID string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := fmt.Sprintf(`{"https://api.openai.com/auth":{"chatgpt_account_id":"%s"}}`, accountID)
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return header + "." + body + ".sig"
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func sseResponse(status int, body string) *http.Response {
	resp := response(status, body)
	resp.Header.Set("Content-Type", "text/event-stream")
	return resp
}

func mustReadBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	resp.Body = io.NopCloser(strings.NewReader(string(body)))
	return body
}
