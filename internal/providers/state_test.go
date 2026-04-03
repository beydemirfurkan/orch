package providers

import (
	"testing"

	"github.com/furkanbeydemir/orch/internal/auth"
	"github.com/furkanbeydemir/orch/internal/config"
)

func TestReadStateDisconnectedWithoutCredentials(t *testing.T) {
	repoRoot := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Provider.Default = "openai"
	cfg.Provider.OpenAI.AuthMode = "api_key"
	if err := config.Save(repoRoot, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	t.Setenv(cfg.Provider.OpenAI.APIKeyEnv, "")

	state, err := ReadState(repoRoot)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if len(state.All) != 1 || state.All[0] != "openai" {
		t.Fatalf("unexpected all providers: %+v", state.All)
	}
	if state.OpenAI.Connected {
		t.Fatalf("expected disconnected openai state")
	}
	if len(state.Connected) != 0 {
		t.Fatalf("expected no connected providers, got: %+v", state.Connected)
	}
}

func TestReadStateConnectedWithStoredAPIKey(t *testing.T) {
	repoRoot := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Provider.Default = "openai"
	cfg.Provider.OpenAI.AuthMode = "api_key"
	if err := config.Save(repoRoot, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if err := auth.Set(repoRoot, "openai", auth.Credential{Type: "api", Key: "sk-test"}); err != nil {
		t.Fatalf("save auth: %v", err)
	}

	t.Setenv(cfg.Provider.OpenAI.APIKeyEnv, "")

	state, err := ReadState(repoRoot)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if !state.OpenAI.Connected {
		t.Fatalf("expected connected openai state")
	}
	if state.OpenAI.Source != "local" {
		t.Fatalf("expected local source, got: %s", state.OpenAI.Source)
	}
	if len(state.Connected) != 1 || state.Connected[0] != "openai" {
		t.Fatalf("unexpected connected providers: %+v", state.Connected)
	}
}

func TestReadStateUsesActiveOAuthCredential(t *testing.T) {
	repoRoot := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Provider.Default = "openai"
	cfg.Provider.OpenAI.AuthMode = "account"
	if err := config.Save(repoRoot, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if err := auth.Set(repoRoot, "openai", auth.Credential{Type: "oauth", AccessToken: testAccountToken("acc-1"), RefreshToken: "refresh-1", AccountID: "acc-1", Email: "one@example.com"}); err != nil {
		t.Fatalf("save first oauth: %v", err)
	}
	if err := auth.Set(repoRoot, "openai", auth.Credential{Type: "oauth", AccessToken: testAccountToken("acc-2"), RefreshToken: "refresh-2", AccountID: "acc-2", Email: "two@example.com"}); err != nil {
		t.Fatalf("save second oauth: %v", err)
	}
	if err := auth.SetActive(repoRoot, "openai", "acc-1"); err != nil {
		t.Fatalf("set active oauth: %v", err)
	}

	state, err := ReadState(repoRoot)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if !state.OpenAI.Connected {
		t.Fatalf("expected connected openai state")
	}
	if state.OpenAI.Source != "local" {
		t.Fatalf("expected local source, got: %s", state.OpenAI.Source)
	}
	active, err := auth.Get(repoRoot, "openai")
	if err != nil {
		t.Fatalf("get active oauth: %v", err)
	}
	if active == nil || active.ID != "acc-1" {
		t.Fatalf("expected acc-1 active, got %#v", active)
	}
}

func testAccountToken(accountID string) string {
	return "eyJhbGciOiJub25lIn0.eyJodHRwczovL2FwaS5vcGVuYWkuY29tL2F1dGgiOnsiY2hhdGdwdF9hY2NvdW50X2lkIjoi" + accountID + "In19.sig"
}
