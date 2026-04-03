package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/config"
)

func TestLoadMigratesLegacySingleCredentialMap(t *testing.T) {
	repoRoot := t.TempDir()
	if err := config.EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}

	legacy := map[string]Credential{
		"openai": {
			Type:         "oauth",
			AccessToken:  "token-123",
			RefreshToken: "refresh-123",
			AccountID:    "acc-123",
			Email:        "user@example.com",
			UpdatedAt:    time.Now().UTC(),
		},
	}
	data, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy auth: %v", err)
	}
	path := filepath.Join(repoRoot, config.OrchDir, authFile)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write legacy auth: %v", err)
	}

	creds, activeID, err := List(repoRoot, "openai")
	if err != nil {
		t.Fatalf("list credentials: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}
	if activeID != "acc-123" {
		t.Fatalf("expected acc-123 active, got %s", activeID)
	}
	if creds[0].ID != "acc-123" {
		t.Fatalf("expected migrated id acc-123, got %s", creds[0].ID)
	}
}

func TestSetActiveSwitchesReturnedCredential(t *testing.T) {
	repoRoot := t.TempDir()
	if err := Set(repoRoot, "openai", Credential{Type: "oauth", AccessToken: "token-1", RefreshToken: "refresh-1", AccountID: "acc-1", Email: "one@example.com"}); err != nil {
		t.Fatalf("set first credential: %v", err)
	}
	if err := Set(repoRoot, "openai", Credential{Type: "oauth", AccessToken: "token-2", RefreshToken: "refresh-2", AccountID: "acc-2", Email: "two@example.com"}); err != nil {
		t.Fatalf("set second credential: %v", err)
	}

	if err := SetActive(repoRoot, "openai", "acc-1"); err != nil {
		t.Fatalf("set active: %v", err)
	}

	active, err := Get(repoRoot, "openai")
	if err != nil {
		t.Fatalf("get active credential: %v", err)
	}
	if active == nil || active.ID != "acc-1" {
		t.Fatalf("expected acc-1 active, got %#v", active)
	}
}
