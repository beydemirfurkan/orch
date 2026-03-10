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
