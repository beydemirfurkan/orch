package providers

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/furkanbeydemir/orch/internal/auth"
	"github.com/furkanbeydemir/orch/internal/config"
)

type OpenAIState struct {
	Enabled   bool
	Connected bool
	Mode      string
	Source    string
	Reason    string
}

type ProviderState struct {
	All       []string
	Default   map[string]string
	Connected []string
	OpenAI    OpenAIState
}

func ReadState(repoRoot string) (ProviderState, error) {
	cfg, err := config.Load(repoRoot)
	if err != nil {
		return ProviderState{}, fmt.Errorf("failed to load config: %w", err)
	}

	state := ProviderState{
		All:       []string{},
		Default:   map[string]string{},
		Connected: []string{},
		OpenAI: OpenAIState{
			Enabled: cfg.Provider.Flags.OpenAIEnabled,
			Mode:    strings.ToLower(strings.TrimSpace(cfg.Provider.OpenAI.AuthMode)),
		},
	}

	if state.OpenAI.Mode == "" {
		state.OpenAI.Mode = "api_key"
	}

	if cfg.Provider.Flags.OpenAIEnabled {
		state.All = append(state.All, "openai")
		state.Default["openai"] = strings.TrimSpace(cfg.Provider.OpenAI.Models.Coder)
	}

	if !cfg.Provider.Flags.OpenAIEnabled {
		state.OpenAI.Reason = "provider disabled"
		return state, nil
	}

	var connected bool
	var source string

	switch state.OpenAI.Mode {
	case "api_key":
		if strings.TrimSpace(os.Getenv(cfg.Provider.OpenAI.APIKeyEnv)) != "" {
			connected = true
			source = "env"
		} else {
			cred, credErr := auth.Get(repoRoot, "openai")
			if credErr == nil && cred != nil && strings.TrimSpace(strings.ToLower(cred.Type)) == "api" && strings.TrimSpace(cred.Key) != "" {
				connected = true
				source = "local"
			}
		}
		if !connected {
			state.OpenAI.Reason = fmt.Sprintf("missing API key (%s or local auth)", cfg.Provider.OpenAI.APIKeyEnv)
		}
	case "account":
		if strings.TrimSpace(os.Getenv(cfg.Provider.OpenAI.AccountTokenEnv)) != "" {
			connected = true
			source = "env"
		} else {
			cred, credErr := auth.Get(repoRoot, "openai")
			if credErr == nil && cred != nil && strings.TrimSpace(strings.ToLower(cred.Type)) == "oauth" && strings.TrimSpace(cred.AccessToken) != "" {
				connected = true
				source = "local"
			}
		}
		if !connected {
			state.OpenAI.Reason = fmt.Sprintf("missing account token (%s or local auth)", cfg.Provider.OpenAI.AccountTokenEnv)
		}
	default:
		state.OpenAI.Reason = fmt.Sprintf("invalid auth mode (%s)", state.OpenAI.Mode)
	}

	state.OpenAI.Connected = connected
	state.OpenAI.Source = source
	if connected {
		state.Connected = append(state.Connected, "openai")
		state.OpenAI.Reason = ""
	}

	sort.Strings(state.All)
	sort.Strings(state.Connected)

	return state, nil
}
