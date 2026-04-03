package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/auth"
	"github.com/furkanbeydemir/orch/internal/config"
)

func TestAuthLoginAccountAndLogout(t *testing.T) {
	repoRoot := t.TempDir()
	t.Chdir(repoRoot)

	if err := config.EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}
	if err := config.Save(repoRoot, config.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	results := []auth.OAuthResult{
		{
			AccessToken:  "token-123",
			RefreshToken: "refresh-123",
			ExpiresAt:    time.Now().UTC().Add(1 * time.Hour),
			AccountID:    "acc-123",
			Email:        "oauth@example.com",
		},
		{
			AccessToken:  "token-456",
			RefreshToken: "refresh-456",
			ExpiresAt:    time.Now().UTC().Add(2 * time.Hour),
			AccountID:    "acc-456",
			Email:        "second@example.com",
		},
	}
	originalOAuthRunner := runOAuthLoginFlow
	runOAuthLoginFlow = func(flow string) (auth.OAuthResult, error) {
		result := results[0]
		results = results[1:]
		return result, nil
	}
	defer func() {
		runOAuthLoginFlow = originalOAuthRunner
	}()

	authModeFlag = "account"
	authMethodFlag = ""
	authFlowFlag = "headless"
	authProviderFlag = "openai"
	authEmailFlag = "user@example.com"
	authAPIKeyFlag = ""
	if err := runAuthLogin(nil, nil); err != nil {
		t.Fatalf("auth login account: %v", err)
	}

	state, err := auth.Load(repoRoot)
	if err != nil {
		t.Fatalf("load auth state: %v", err)
	}
	if state == nil || state.AccessToken != "token-123" {
		t.Fatalf("expected stored account token")
	}
	if state.RefreshToken != "refresh-123" {
		t.Fatalf("expected stored refresh token")
	}
	if state.AccountID != "acc-123" {
		t.Fatalf("expected stored account id")
	}

	authEmailFlag = "second@example.com"
	if err := runAuthLogin(nil, nil); err != nil {
		t.Fatalf("auth login second account: %v", err)
	}

	credentials, activeID, err := auth.List(repoRoot, "openai")
	if err != nil {
		t.Fatalf("list credentials: %v", err)
	}
	if len(credentials) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(credentials))
	}
	if activeID != "acc-456" {
		t.Fatalf("expected second account to become active, got %s", activeID)
	}

	if err := runAuthUse(nil, []string{"acc-123"}); err != nil {
		t.Fatalf("auth use: %v", err)
	}
	active, err := auth.Get(repoRoot, "openai")
	if err != nil {
		t.Fatalf("get active credential: %v", err)
	}
	if active == nil || active.ID != "acc-123" {
		t.Fatalf("expected acc-123 active, got %#v", active)
	}

	if err := runAuthRemove(nil, []string{"acc-456"}); err != nil {
		t.Fatalf("auth remove: %v", err)
	}
	credentials, activeID, err = auth.List(repoRoot, "openai")
	if err != nil {
		t.Fatalf("list credentials after remove: %v", err)
	}
	if len(credentials) != 1 {
		t.Fatalf("expected 1 credential after remove, got %d", len(credentials))
	}
	if activeID != "acc-123" {
		t.Fatalf("expected acc-123 to remain active, got %s", activeID)
	}

	if err := runAuthLogout(nil, nil); err != nil {
		t.Fatalf("auth logout: %v", err)
	}
	state, err = auth.Load(repoRoot)
	if err != nil {
		t.Fatalf("load auth state after logout: %v", err)
	}
	if state != nil {
		t.Fatalf("expected auth state to be removed")
	}
}

func TestAuthLoginAPIKeyMode(t *testing.T) {
	repoRoot := t.TempDir()
	t.Chdir(repoRoot)

	if err := config.EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}
	if err := config.Save(repoRoot, config.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	authModeFlag = ""
	authMethodFlag = "api"
	authFlowFlag = ""
	authProviderFlag = "openai"
	authEmailFlag = ""
	authAPIKeyFlag = "sk-test"
	if err := runAuthLogin(nil, nil); err != nil {
		t.Fatalf("auth login api_key: %v", err)
	}

	cfg, err := config.Load(repoRoot)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Provider.OpenAI.AuthMode != "api_key" {
		t.Fatalf("expected auth mode api_key, got %s", cfg.Provider.OpenAI.AuthMode)
	}
}

func TestAuthListShowsCredentialDetails(t *testing.T) {
	repoRoot := t.TempDir()
	t.Chdir(repoRoot)

	if err := config.EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}
	if err := auth.Set(repoRoot, "openai", auth.Credential{
		Type:          "oauth",
		AccessToken:   "token-123",
		RefreshToken:  "refresh-123",
		AccountID:     "acc-123",
		Email:         "user@example.com",
		ExpiresAt:     time.Now().UTC().Add(time.Hour),
		CooldownUntil: time.Now().UTC().Add(2 * time.Minute),
		LastError:     "provider_rate_limited: chat rate limited",
		LastUsedAt:    time.Now().UTC().Add(-5 * time.Minute),
	}); err != nil {
		t.Fatalf("set oauth credential: %v", err)
	}

	out := captureStdout(t, func() {
		if err := runAuthList(nil, nil); err != nil {
			t.Fatalf("auth list: %v", err)
		}
	})

	for _, expected := range []string{"status=active", "cooldown_until=", "last_used=", "last_error=provider_rate_limited", "expires_at="} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, out)
		}
	}
}
