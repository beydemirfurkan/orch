package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPreservesExplicitFalseSafetyValues(t *testing.T) {
	repoRoot := t.TempDir()
	if err := EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}

	raw := `{
  "version": 1,
  "models": {"planner":"p","coder":"c","reviewer":"r"},
  "commands": {"test":"go test ./...","lint":"go vet ./..."},
  "patch": {"maxFiles":10,"maxLines":800},
  "safety": {
    "dryRun": true,
    "requireDestructiveApproval": false,
    "lockStaleAfterSeconds": 60,
    "retry": {"validationMax": 1, "testMax": 1, "reviewMax": 1},
    "confidence": {"completeMin": 0.8, "failBelow": 0.4},
    "featureFlags": {
      "permissionMode": false,
      "repoLock": false,
      "retryLimits": false,
      "patchConflictReporting": false,
      "confidenceEnforcement": false
    }
  }
}`

	configPath := filepath.Join(repoRoot, OrchDir, ConfigFile)
	if err := os.WriteFile(configPath, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(repoRoot)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Safety.RequireDestructiveApproval {
		t.Fatalf("expected explicit false requireDestructiveApproval to be preserved")
	}
	if cfg.Safety.FeatureFlags.PermissionMode || cfg.Safety.FeatureFlags.RepoLock || cfg.Safety.FeatureFlags.RetryLimits || cfg.Safety.FeatureFlags.PatchConflictReporting || cfg.Safety.FeatureFlags.ConfidenceEnforcement {
		t.Fatalf("expected explicit false feature flags to be preserved")
	}
	if cfg.Safety.Confidence.CompleteMin != 0.8 || cfg.Safety.Confidence.FailBelow != 0.4 {
		t.Fatalf("expected explicit confidence policy values to be preserved")
	}
}

func TestLoadBackfillsMissingSafetyFields(t *testing.T) {
	repoRoot := t.TempDir()
	if err := EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}

	raw := `{
  "version": 1,
  "models": {"planner":"p","coder":"c","reviewer":"r"},
  "commands": {"test":"go test ./...","lint":"go vet ./..."},
  "patch": {"maxFiles":10,"maxLines":800},
  "safety": {"dryRun": true}
}`

	configPath := filepath.Join(repoRoot, OrchDir, ConfigFile)
	if err := os.WriteFile(configPath, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(repoRoot)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if !cfg.Safety.RequireDestructiveApproval {
		t.Fatalf("expected missing requireDestructiveApproval to be defaulted true")
	}
	if !cfg.Safety.FeatureFlags.PermissionMode || !cfg.Safety.FeatureFlags.RepoLock || !cfg.Safety.FeatureFlags.RetryLimits || !cfg.Safety.FeatureFlags.PatchConflictReporting || !cfg.Safety.FeatureFlags.ConfidenceEnforcement {
		t.Fatalf("expected missing featureFlags to be defaulted true")
	}
	if cfg.Safety.Confidence.CompleteMin <= 0 || cfg.Safety.Confidence.FailBelow <= 0 {
		t.Fatalf("expected missing confidence policy to be backfilled")
	}

	if cfg.Provider.Default != "openai" {
		t.Fatalf("expected default provider to be openai, got=%s", cfg.Provider.Default)
	}
	if cfg.Provider.OpenAI.Models.Coder == "" {
		t.Fatalf("expected default openai coder model to be backfilled")
	}
}

func TestLoadPreservesExplicitProviderFlags(t *testing.T) {
	repoRoot := t.TempDir()
	if err := EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}

	raw := `{
  "version": 1,
  "models": {"planner":"p","coder":"c","reviewer":"r"},
  "commands": {"test":"go test ./...","lint":"go vet ./..."},
  "patch": {"maxFiles":10,"maxLines":800},
  "safety": {"dryRun": true},
  "provider": {
    "default": "openai",
    "openai": {
      "apiKeyEnv": "OPENAI_API_KEY",
      "baseURL": "https://api.openai.com/v1",
      "reasoningEffort": "medium",
      "timeoutSeconds": 30,
      "maxRetries": 1,
      "models": {"planner":"a","coder":"b","reviewer":"c"}
    },
    "flags": {"openaiEnabled": false}
  }
}`

	configPath := filepath.Join(repoRoot, OrchDir, ConfigFile)
	if err := os.WriteFile(configPath, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(repoRoot)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Provider.Flags.OpenAIEnabled {
		t.Fatalf("expected explicit openaiEnabled=false to be preserved")
	}
}
