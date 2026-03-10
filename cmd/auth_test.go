package cmd

import (
	"testing"

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

	authModeFlag = "account"
	authMethodFlag = ""
	authProviderFlag = "openai"
	authTokenFlag = "token-123"
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
	authProviderFlag = "openai"
	authTokenFlag = ""
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
