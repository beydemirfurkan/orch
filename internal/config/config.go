package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	OrchDir     = ".orch"
	ConfigFile  = "config.json"
	RepoMapFile = "repo-map.json"
	RunsDir     = "runs"
)

type Config struct {
	Version  int           `json:"version"`
	Models   ModelConfig   `json:"models"`
	Commands CommandConfig `json:"commands"`
	Patch    PatchConfig   `json:"patch"`
	Safety   SafetyConfig  `json:"safety"`
}

type ModelConfig struct {
	Planner  string `json:"planner"`
	Coder    string `json:"coder"`
	Reviewer string `json:"reviewer"`
}

type CommandConfig struct {
	Test string `json:"test"`
	Lint string `json:"lint"`
}

type PatchConfig struct {
	MaxFiles int `json:"maxFiles"`
	MaxLines int `json:"maxLines"`
}

type SafetyConfig struct {
	DryRun                     bool               `json:"dryRun"`
	RequireDestructiveApproval bool               `json:"requireDestructiveApproval"`
	LockStaleAfterSeconds      int                `json:"lockStaleAfterSeconds"`
	Retry                      RetryPolicyConfig  `json:"retry"`
	FeatureFlags               SafetyFeatureFlags `json:"featureFlags"`
}

type RetryPolicyConfig struct {
	ValidationMax int `json:"validationMax"`
	TestMax       int `json:"testMax"`
	ReviewMax     int `json:"reviewMax"`
}

type SafetyFeatureFlags struct {
	PermissionMode         bool `json:"permissionMode"`
	RepoLock               bool `json:"repoLock"`
	RetryLimits            bool `json:"retryLimits"`
	PatchConflictReporting bool `json:"patchConflictReporting"`
}

func DefaultConfig() *Config {
	return &Config{
		Version: 1,
		Models: ModelConfig{
			Planner:  "openai:gpt-4o-mini",
			Coder:    "anthropic:claude-sonnet",
			Reviewer: "openai:gpt-4o-mini",
		},
		Commands: CommandConfig{
			Test: "go test ./...",
			Lint: "go vet ./...",
		},
		Patch: PatchConfig{
			MaxFiles: 10,
			MaxLines: 800,
		},
		Safety: SafetyConfig{
			DryRun:                     true,
			RequireDestructiveApproval: true,
			LockStaleAfterSeconds:      3600,
			Retry: RetryPolicyConfig{
				ValidationMax: 2,
				TestMax:       2,
				ReviewMax:     2,
			},
			FeatureFlags: SafetyFeatureFlags{
				PermissionMode:         true,
				RepoLock:               true,
				RetryLimits:            true,
				PatchConflictReporting: true,
			},
		},
	}
}

func Load(repoRoot string) (*Config, error) {
	configPath := filepath.Join(repoRoot, OrchDir, ConfigFile)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	applyDefaults(&cfg, data)

	return &cfg, nil
}

func applyDefaults(cfg *Config, rawJSON []byte) {
	defaults := DefaultConfig()
	presence := parsePresence(rawJSON)

	if cfg.Safety.LockStaleAfterSeconds <= 0 {
		cfg.Safety.LockStaleAfterSeconds = defaults.Safety.LockStaleAfterSeconds
	}

	if cfg.Safety.Retry.ValidationMax <= 0 {
		cfg.Safety.Retry.ValidationMax = defaults.Safety.Retry.ValidationMax
	}
	if cfg.Safety.Retry.TestMax <= 0 {
		cfg.Safety.Retry.TestMax = defaults.Safety.Retry.TestMax
	}
	if cfg.Safety.Retry.ReviewMax <= 0 {
		cfg.Safety.Retry.ReviewMax = defaults.Safety.Retry.ReviewMax
	}

	if !presence.featureFlagsPresent {
		cfg.Safety.FeatureFlags = defaults.Safety.FeatureFlags
	}

	if !presence.requireDestructiveApprovalPresent {
		cfg.Safety.RequireDestructiveApproval = defaults.Safety.RequireDestructiveApproval
	}

}

type fieldPresence struct {
	featureFlagsPresent               bool
	requireDestructiveApprovalPresent bool
}

func parsePresence(rawJSON []byte) fieldPresence {
	result := fieldPresence{}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(rawJSON, &root); err != nil {
		return result
	}

	safetyRaw, ok := root["safety"]
	if !ok {
		return result
	}

	var safety map[string]json.RawMessage
	if err := json.Unmarshal(safetyRaw, &safety); err != nil {
		return result
	}

	_, result.featureFlagsPresent = safety["featureFlags"]
	_, result.requireDestructiveApprovalPresent = safety["requireDestructiveApproval"]

	return result

}

func Save(repoRoot string, cfg *Config) error {
	orchDir := filepath.Join(repoRoot, OrchDir)
	if err := os.MkdirAll(orchDir, 0o755); err != nil {
		return fmt.Errorf("failed to create .orch directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	configPath := filepath.Join(orchDir, ConfigFile)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func EnsureOrchDir(repoRoot string) error {
	dirs := []string{
		filepath.Join(repoRoot, OrchDir),
		filepath.Join(repoRoot, OrchDir, RunsDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}
