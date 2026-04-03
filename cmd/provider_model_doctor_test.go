package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/furkanbeydemir/orch/internal/config"
)

func TestProviderAndModelCommands(t *testing.T) {
	repoRoot := t.TempDir()
	t.Chdir(repoRoot)

	if err := config.EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}
	if err := config.Save(repoRoot, config.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if err := runProviderSet(nil, []string{"openai"}); err != nil {
		t.Fatalf("provider set: %v", err)
	}

	if err := runModelSet(nil, []string{"coder", "gpt-5.3-codex"}); err != nil {
		t.Fatalf("model set: %v", err)
	}

	if err := runProviderShow(nil, nil); err != nil {
		t.Fatalf("provider show: %v", err)
	}
	if err := runModelShow(nil, nil); err != nil {
		t.Fatalf("model show: %v", err)
	}
}

func TestDoctorFailsWithoutAPIKey(t *testing.T) {
	repoRoot := t.TempDir()
	t.Chdir(repoRoot)

	if err := config.EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}
	if err := config.Save(repoRoot, config.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	t.Setenv("OPENAI_API_KEY", "")
	err := runDoctor(nil, nil)
	if err == nil {
		t.Fatalf("expected doctor failure without API key")
	}
	if !strings.Contains(err.Error(), "doctor failed") {
		t.Fatalf("unexpected doctor error: %v", err)
	}
}

func TestDoctorProbeAccountModeSucceeds(t *testing.T) {
	repoRoot := t.TempDir()
	t.Chdir(repoRoot)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/codex/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("ChatGPT-Account-Id"); got != "acc-123" {
			t.Fatalf("unexpected account header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"OK","status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
	}))
	defer server.Close()

	if err := config.EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Provider.OpenAI.AuthMode = "account"
	cfg.Provider.OpenAI.BaseURL = server.URL
	cfg.Provider.OpenAI.TimeoutSeconds = 5
	if err := config.Save(repoRoot, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	t.Setenv("OPENAI_ACCOUNT_TOKEN", testDoctorAccountToken("acc-123"))
	doctorProbeFlag = true
	defer func() { doctorProbeFlag = false }()

	if err := runDoctor(nil, nil); err != nil {
		t.Fatalf("expected doctor probe to succeed: %v", err)
	}
}

func TestDoctorProbeAccountModeFailsWhenProviderRejects(t *testing.T) {
	repoRoot := t.TempDir()
	t.Chdir(repoRoot)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	if err := config.EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Provider.OpenAI.AuthMode = "account"
	cfg.Provider.OpenAI.BaseURL = server.URL
	cfg.Provider.OpenAI.TimeoutSeconds = 5
	if err := config.Save(repoRoot, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	t.Setenv("OPENAI_ACCOUNT_TOKEN", testDoctorAccountToken("acc-123"))
	doctorProbeFlag = true
	defer func() { doctorProbeFlag = false }()

	err := runDoctor(nil, nil)
	if err == nil {
		t.Fatalf("expected doctor probe failure")
	}
	if !strings.Contains(err.Error(), "doctor failed") {
		t.Fatalf("unexpected doctor error: %v", err)
	}
}

func TestProviderListJSONOutput(t *testing.T) {
	repoRoot := t.TempDir()
	t.Chdir(repoRoot)

	if err := config.EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}
	if err := config.Save(repoRoot, config.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	providerJSONFlag = true
	defer func() { providerJSONFlag = false }()

	out := captureStdout(t, func() {
		if err := runProviderList(nil, nil); err != nil {
			t.Fatalf("provider list: %v", err)
		}
	})

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("invalid json output: %v\noutput=%s", err, out)
	}

	all, ok := payload["all"].([]any)
	if !ok || len(all) == 0 {
		t.Fatalf("expected non-empty all providers, got: %#v", payload["all"])
	}
	if all[0] != "openai" {
		t.Fatalf("expected openai in all providers, got: %#v", all)
	}
}

func testDoctorAccountToken(accountID string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := fmt.Sprintf(`{"https://api.openai.com/auth":{"chatgpt_account_id":"%s"}}`, accountID)
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return header + "." + body + ".sig"
}
