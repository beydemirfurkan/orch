package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/furkanbeydemir/orch/internal/config"
)

const authFile = "auth.json"

type State struct {
	Provider    string    `json:"provider"`
	Mode        string    `json:"mode"`
	AccessToken string    `json:"accessToken"`
	Email       string    `json:"email,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func Load(repoRoot string) (*State, error) {
	path := filepath.Join(repoRoot, config.OrchDir, authFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read auth state: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse auth state: %w", err)
	}

	state.Provider = strings.TrimSpace(state.Provider)
	state.Mode = strings.TrimSpace(state.Mode)
	state.AccessToken = strings.TrimSpace(state.AccessToken)

	return &state, nil
}

func Save(repoRoot string, state *State) error {
	if state == nil {
		return fmt.Errorf("auth state cannot be nil")
	}
	if err := config.EnsureOrchDir(repoRoot); err != nil {
		return err
	}

	state.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize auth state: %w", err)
	}

	path := filepath.Join(repoRoot, config.OrchDir, authFile)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write auth state: %w", err)
	}

	return nil
}

func Clear(repoRoot string) error {
	path := filepath.Join(repoRoot, config.OrchDir, authFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove auth state: %w", err)
	}
	return nil
}
