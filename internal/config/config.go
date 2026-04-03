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
	Version  int            `json:"version"`
	Models   ModelConfig    `json:"models"`
	Commands CommandConfig  `json:"commands"`
	Patch    PatchConfig    `json:"patch"`
	Safety   SafetyConfig   `json:"safety"`
	Budget   BudgetConfig   `json:"budget"`
	Provider ProviderConfig `json:"provider"`
	Skills   SkillsConfig   `json:"skills,omitempty"`
	MCP      MCPConfig      `json:"mcp,omitempty"`
}

// SkillsConfig controls which skills are enabled globally or per agent.
type SkillsConfig struct {
	// GlobalSkills are enabled for all agents.
	GlobalSkills []string `json:"globalSkills,omitempty"`
	// AgentSkills maps agent names to their additional skill list.
	AgentSkills map[string][]string `json:"agentSkills,omitempty"`
}

// MCPConfig lists external MCP server definitions.
type MCPConfig struct {
	Servers []MCPServerConfig `json:"servers,omitempty"`
}

// MCPServerConfig describes a single MCP server.
type MCPServerConfig struct {
	// Name is used as the tool name prefix (e.g. "context7" → tool "context7_query").
	Name string `json:"name"`
	// Command is a stdio command (e.g. "npx -y @upstash/context7-mcp").
	Command string `json:"command,omitempty"`
	// URL is an HTTP endpoint for HTTP-based MCP servers.
	URL string `json:"url,omitempty"`
	// Env holds additional environment variables for stdio servers.
	Env map[string]string `json:"env,omitempty"`
	// Skills lists which skill names this server's tools satisfy.
	Skills []string `json:"skills,omitempty"`
}

type BudgetConfig struct {
	PlannerMaxTokens  int `json:"plannerMaxTokens"`
	CoderMaxTokens    int `json:"coderMaxTokens"`
	ReviewerMaxTokens int `json:"reviewerMaxTokens"`
	FixerMaxTokens    int `json:"fixerMaxTokens"`
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
	DryRun                     bool                   `json:"dryRun"`
	RequireDestructiveApproval bool                   `json:"requireDestructiveApproval"`
	LockStaleAfterSeconds      int                    `json:"lockStaleAfterSeconds"`
	Retry                      RetryPolicyConfig      `json:"retry"`
	Confidence                 ConfidencePolicyConfig `json:"confidence"`
	FeatureFlags               SafetyFeatureFlags     `json:"featureFlags"`
}

type RetryPolicyConfig struct {
	ValidationMax int `json:"validationMax"`
	TestMax       int `json:"testMax"`
	ReviewMax     int `json:"reviewMax"`
}

type ConfidencePolicyConfig struct {
	CompleteMin float64 `json:"completeMin"`
	FailBelow   float64 `json:"failBelow"`
}

type SafetyFeatureFlags struct {
	PermissionMode         bool `json:"permissionMode"`
	RepoLock               bool `json:"repoLock"`
	RetryLimits            bool `json:"retryLimits"`
	PatchConflictReporting bool `json:"patchConflictReporting"`
	ConfidenceEnforcement  bool `json:"confidenceEnforcement"`
	// Pantheon agent flags — all disabled by default to preserve existing behaviour.
	ExplorerEnabled bool `json:"explorerEnabled"`
	OracleEnabled   bool `json:"oracleEnabled"`
	FixerEnabled    bool `json:"fixerEnabled"`
}

type ProviderConfig struct {
	Default   string                  `json:"default"`
	OpenAI    OpenAIProviderConfig    `json:"openai"`
	Anthropic AnthropicProviderConfig `json:"anthropic,omitempty"`
	Ollama    OllamaProviderConfig    `json:"ollama,omitempty"`
	Flags     ProviderFeatureFlags    `json:"flags"`
	// RoleAssignments maps role names to "providerName:modelID" strings.
	// When set, takes precedence over the per-provider Models config.
	// Example: {"planner": "anthropic:claude-opus-4-5", "coder": "openai:o3"}
	RoleAssignments map[string]string `json:"roleAssignments,omitempty"`
}

type AnthropicProviderConfig struct {
	APIKeyEnv      string `json:"apiKeyEnv"`
	BaseURL        string `json:"baseURL"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	MaxRetries     int    `json:"maxRetries"`
}

type OllamaProviderConfig struct {
	BaseURL        string `json:"baseURL"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	Enabled        bool   `json:"enabled"`
}

type OpenAIProviderConfig struct {
	APIKeyEnv       string             `json:"apiKeyEnv"`
	AccountTokenEnv string             `json:"accountTokenEnv"`
	AuthMode        string             `json:"authMode"`
	BaseURL         string             `json:"baseURL"`
	ReasoningEffort string             `json:"reasoningEffort"`
	TimeoutSeconds  int                `json:"timeoutSeconds"`
	MaxRetries      int                `json:"maxRetries"`
	Models          ProviderRoleModels `json:"models"`
}

type ProviderRoleModels struct {
	Planner  string `json:"planner"`
	Coder    string `json:"coder"`
	Reviewer string `json:"reviewer"`
	Explorer string `json:"explorer,omitempty"`
	Oracle   string `json:"oracle,omitempty"`
	Fixer    string `json:"fixer,omitempty"`
}

type ProviderFeatureFlags struct {
	OpenAIEnabled bool `json:"openaiEnabled"`
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
		Budget: BudgetConfig{
			PlannerMaxTokens:  4096,
			CoderMaxTokens:    8192,
			ReviewerMaxTokens: 2048,
			FixerMaxTokens:    4096,
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
			Confidence: ConfidencePolicyConfig{
				CompleteMin: 0.70,
				FailBelow:   0.50,
			},
			FeatureFlags: SafetyFeatureFlags{
				PermissionMode:         true,
				RepoLock:               true,
				RetryLimits:            true,
				PatchConflictReporting: true,
				ConfidenceEnforcement:  true,
			},
		},
		Provider: ProviderConfig{
			Default: "openai",
			Anthropic: AnthropicProviderConfig{
				APIKeyEnv:      "ANTHROPIC_API_KEY",
				BaseURL:        "https://api.anthropic.com/v1",
				TimeoutSeconds: 120,
				MaxRetries:     3,
			},
			Ollama: OllamaProviderConfig{
				BaseURL:        "http://localhost:11434",
				TimeoutSeconds: 180,
				Enabled:        false,
			},
			OpenAI: OpenAIProviderConfig{
				APIKeyEnv:       "OPENAI_API_KEY",
				AccountTokenEnv: "OPENAI_ACCOUNT_TOKEN",
				AuthMode:        "api_key",
				BaseURL:         "https://api.openai.com/v1",
				ReasoningEffort: "medium",
				TimeoutSeconds:  90,
				MaxRetries:      3,
				Models: ProviderRoleModels{
					Planner:  "gpt-5.3-codex",
					Coder:    "gpt-5.3-codex",
					Reviewer: "gpt-5.3-codex",
				},
			},
			Flags: ProviderFeatureFlags{
				OpenAIEnabled: true,
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

	if cfg.Budget.PlannerMaxTokens <= 0 {
		cfg.Budget.PlannerMaxTokens = defaults.Budget.PlannerMaxTokens
	}
	if cfg.Budget.CoderMaxTokens <= 0 {
		cfg.Budget.CoderMaxTokens = defaults.Budget.CoderMaxTokens
	}
	if cfg.Budget.ReviewerMaxTokens <= 0 {
		cfg.Budget.ReviewerMaxTokens = defaults.Budget.ReviewerMaxTokens
	}
	if cfg.Budget.FixerMaxTokens <= 0 {
		cfg.Budget.FixerMaxTokens = defaults.Budget.FixerMaxTokens
	}

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
	if cfg.Safety.Confidence.CompleteMin <= 0 {
		cfg.Safety.Confidence.CompleteMin = defaults.Safety.Confidence.CompleteMin
	}
	if cfg.Safety.Confidence.FailBelow <= 0 {
		cfg.Safety.Confidence.FailBelow = defaults.Safety.Confidence.FailBelow
	}

	if !presence.featureFlagsPresent {
		cfg.Safety.FeatureFlags = defaults.Safety.FeatureFlags
	}
	if !presence.confidenceEnforcementPresent {
		cfg.Safety.FeatureFlags.ConfidenceEnforcement = defaults.Safety.FeatureFlags.ConfidenceEnforcement
	}

	if !presence.requireDestructiveApprovalPresent {
		cfg.Safety.RequireDestructiveApproval = defaults.Safety.RequireDestructiveApproval
	}

	if !presence.providerPresent {
		cfg.Provider = defaults.Provider
	} else {
		if cfg.Provider.Default == "" {
			cfg.Provider.Default = defaults.Provider.Default
		}
		if cfg.Provider.OpenAI.APIKeyEnv == "" {
			cfg.Provider.OpenAI.APIKeyEnv = defaults.Provider.OpenAI.APIKeyEnv
		}
		if cfg.Provider.OpenAI.AccountTokenEnv == "" {
			cfg.Provider.OpenAI.AccountTokenEnv = defaults.Provider.OpenAI.AccountTokenEnv
		}
		if cfg.Provider.OpenAI.AuthMode == "" {
			cfg.Provider.OpenAI.AuthMode = defaults.Provider.OpenAI.AuthMode
		}
		if cfg.Provider.OpenAI.BaseURL == "" {
			cfg.Provider.OpenAI.BaseURL = defaults.Provider.OpenAI.BaseURL
		}
		if cfg.Provider.OpenAI.ReasoningEffort == "" {
			cfg.Provider.OpenAI.ReasoningEffort = defaults.Provider.OpenAI.ReasoningEffort
		}
		if cfg.Provider.OpenAI.TimeoutSeconds <= 0 {
			cfg.Provider.OpenAI.TimeoutSeconds = defaults.Provider.OpenAI.TimeoutSeconds
		}
		if cfg.Provider.OpenAI.MaxRetries <= 0 {
			cfg.Provider.OpenAI.MaxRetries = defaults.Provider.OpenAI.MaxRetries
		}
		if cfg.Provider.OpenAI.Models.Planner == "" {
			cfg.Provider.OpenAI.Models.Planner = defaults.Provider.OpenAI.Models.Planner
		}
		if cfg.Provider.OpenAI.Models.Coder == "" {
			cfg.Provider.OpenAI.Models.Coder = defaults.Provider.OpenAI.Models.Coder
		}
		if cfg.Provider.OpenAI.Models.Reviewer == "" {
			cfg.Provider.OpenAI.Models.Reviewer = defaults.Provider.OpenAI.Models.Reviewer
		}
		if !presence.providerFlagsPresent {
			cfg.Provider.Flags = defaults.Provider.Flags
		}
	}
}

type fieldPresence struct {
	featureFlagsPresent               bool
	confidenceEnforcementPresent      bool
	requireDestructiveApprovalPresent bool
	providerPresent                   bool
	providerFlagsPresent              bool
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

	if featureFlagsRaw, ok := safety["featureFlags"]; ok {
		result.featureFlagsPresent = true
		var featureFlags map[string]json.RawMessage
		if err := json.Unmarshal(featureFlagsRaw, &featureFlags); err == nil {
			_, result.confidenceEnforcementPresent = featureFlags["confidenceEnforcement"]
		}
	}
	_, result.requireDestructiveApprovalPresent = safety["requireDestructiveApproval"]

	providerRaw, ok := root["provider"]
	if ok {
		result.providerPresent = true
		var provider map[string]json.RawMessage
		if err := json.Unmarshal(providerRaw, &provider); err == nil {
			_, result.providerFlagsPresent = provider["flags"]
		}
	}

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
