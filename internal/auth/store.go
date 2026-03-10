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

type Credential struct {
	Type        string    `json:"type"`
	Key         string    `json:"key,omitempty"`
	AccessToken string    `json:"accessToken,omitempty"`
	Email       string    `json:"email,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func Load(repoRoot string) (*State, error) {
	cred, err := Get(repoRoot, "openai")
	if err != nil {
		return nil, err
	}
	if cred == nil {
		return nil, nil
	}

	state := &State{
		Provider:  "openai",
		UpdatedAt: cred.UpdatedAt,
		Email:     strings.TrimSpace(cred.Email),
	}

	switch strings.TrimSpace(strings.ToLower(cred.Type)) {
	case "oauth", "account":
		state.Mode = "account"
		state.AccessToken = strings.TrimSpace(cred.AccessToken)
	case "api", "api_key":
		state.Mode = "api_key"
	default:
		state.Mode = strings.TrimSpace(cred.Type)
	}

	return state, nil
}

func LoadAll(repoRoot string) (map[string]Credential, error) {
	path := filepath.Join(repoRoot, config.OrchDir, authFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Credential{}, nil
		}
		return nil, fmt.Errorf("read auth state: %w", err)
	}

	parsed := map[string]Credential{}
	if err := json.Unmarshal(data, &parsed); err == nil {
		clean := map[string]Credential{}
		for provider, cred := range parsed {
			id := strings.ToLower(strings.TrimSpace(provider))
			if id == "" {
				continue
			}

			cred.Type = strings.ToLower(strings.TrimSpace(cred.Type))
			cred.Key = strings.TrimSpace(cred.Key)
			cred.AccessToken = strings.TrimSpace(cred.AccessToken)
			cred.Email = strings.TrimSpace(cred.Email)

			if cred.Type == "" {
				continue
			}
			if cred.Type == "api_key" {
				cred.Type = "api"
			}
			if cred.Type == "account" {
				cred.Type = "oauth"
			}
			clean[id] = cred
		}
		if len(clean) > 0 {
			return clean, nil
		}
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse auth state: %w", err)
	}

	provider := strings.ToLower(strings.TrimSpace(state.Provider))
	if provider == "" {
		provider = "openai"
	}

	mode := strings.ToLower(strings.TrimSpace(state.Mode))
	cred := Credential{
		Email:     strings.TrimSpace(state.Email),
		UpdatedAt: state.UpdatedAt,
	}
	if mode == "account" {
		cred.Type = "oauth"
		cred.AccessToken = strings.TrimSpace(state.AccessToken)
	}
	if mode == "api_key" {
		cred.Type = "api"
		cred.Key = strings.TrimSpace(state.AccessToken)
	}
	if cred.Type == "" {
		cred.Type = mode
	}

	if cred.Type == "" {
		return map[string]Credential{}, nil
	}

	return map[string]Credential{provider: cred}, nil
}

func Save(repoRoot string, state *State) error {
	if state == nil {
		return fmt.Errorf("auth state cannot be nil")
	}
	provider := strings.ToLower(strings.TrimSpace(state.Provider))
	if provider == "" {
		provider = "openai"
	}

	mode := strings.ToLower(strings.TrimSpace(state.Mode))
	cred := Credential{
		Email:       strings.TrimSpace(state.Email),
		AccessToken: strings.TrimSpace(state.AccessToken),
	}
	if mode == "account" {
		cred.Type = "oauth"
	}
	if mode == "api_key" {
		cred.Type = "api"
		cred.Key = strings.TrimSpace(state.AccessToken)
		cred.AccessToken = ""
	}
	if cred.Type == "" {
		return fmt.Errorf("unsupported auth mode: %s", state.Mode)
	}

	return Set(repoRoot, provider, cred)
}

func Clear(repoRoot string) error {
	path := filepath.Join(repoRoot, config.OrchDir, authFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove auth state: %w", err)
	}
	return nil
}

func Get(repoRoot, provider string) (*Credential, error) {
	all, err := LoadAll(repoRoot)
	if err != nil {
		return nil, err
	}
	id := strings.ToLower(strings.TrimSpace(provider))
	if id == "" {
		return nil, nil
	}
	cred, ok := all[id]
	if !ok {
		return nil, nil
	}
	return &cred, nil
}

func Set(repoRoot, provider string, cred Credential) error {
	id := strings.ToLower(strings.TrimSpace(provider))
	if id == "" {
		return fmt.Errorf("provider is required")
	}

	kind := strings.ToLower(strings.TrimSpace(cred.Type))
	if kind == "api_key" {
		kind = "api"
	}
	if kind == "account" {
		kind = "oauth"
	}
	if kind != "api" && kind != "oauth" && kind != "wellknown" {
		return fmt.Errorf("unsupported credential type: %s", cred.Type)
	}

	if err := config.EnsureOrchDir(repoRoot); err != nil {
		return err
	}

	all, err := LoadAll(repoRoot)
	if err != nil {
		return err
	}

	cred.Type = kind
	cred.Key = strings.TrimSpace(cred.Key)
	cred.AccessToken = strings.TrimSpace(cred.AccessToken)
	cred.Email = strings.TrimSpace(cred.Email)
	cred.UpdatedAt = time.Now().UTC()

	all[id] = cred

	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize auth state: %w", err)
	}

	path := filepath.Join(repoRoot, config.OrchDir, authFile)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write auth state: %w", err)
	}

	return nil
}

func Remove(repoRoot, provider string) error {
	all, err := LoadAll(repoRoot)
	if err != nil {
		return err
	}
	id := strings.ToLower(strings.TrimSpace(provider))
	if id == "" {
		return fmt.Errorf("provider is required")
	}
	if _, ok := all[id]; !ok {
		return nil
	}
	delete(all, id)

	if len(all) == 0 {
		return Clear(repoRoot)
	}

	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize auth state: %w", err)
	}

	path := filepath.Join(repoRoot, config.OrchDir, authFile)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write auth state: %w", err)
	}
	return nil
}
