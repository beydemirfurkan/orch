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
	Provider     string    `json:"provider"`
	Mode         string    `json:"mode"`
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
	AccountID    string    `json:"accountId,omitempty"`
	Email        string    `json:"email,omitempty"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Credential struct {
	ID            string    `json:"id,omitempty"`
	Label         string    `json:"label,omitempty"`
	Type          string    `json:"type"`
	Key           string    `json:"key,omitempty"`
	AccessToken   string    `json:"accessToken,omitempty"`
	RefreshToken  string    `json:"refreshToken,omitempty"`
	ExpiresAt     time.Time `json:"expiresAt,omitempty"`
	AccountID     string    `json:"accountId,omitempty"`
	Email         string    `json:"email,omitempty"`
	CooldownUntil time.Time `json:"cooldownUntil,omitempty"`
	LastError     string    `json:"lastError,omitempty"`
	LastUsedAt    time.Time `json:"lastUsedAt,omitempty"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type ProviderCredentials struct {
	ActiveID    string       `json:"activeId,omitempty"`
	Credentials []Credential `json:"credentials"`
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
		Provider:     "openai",
		UpdatedAt:    cred.UpdatedAt,
		RefreshToken: strings.TrimSpace(cred.RefreshToken),
		ExpiresAt:    cred.ExpiresAt,
		AccountID:    strings.TrimSpace(cred.AccountID),
		Email:        strings.TrimSpace(cred.Email),
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
	stores, err := readProviderStores(repoRoot)
	if err != nil {
		return nil, err
	}

	active := map[string]Credential{}
	for provider, store := range stores {
		cred, ok := activeCredential(store)
		if ok {
			active[provider] = cred
		}
	}
	return active, nil
}

func List(repoRoot, provider string) ([]Credential, string, error) {
	store, err := loadProviderCredentials(repoRoot, provider)
	if err != nil {
		return nil, "", err
	}
	if store == nil {
		return []Credential{}, "", nil
	}
	creds := append([]Credential(nil), store.Credentials...)
	return creds, strings.TrimSpace(store.ActiveID), nil
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
		Email:        strings.TrimSpace(state.Email),
		AccessToken:  strings.TrimSpace(state.AccessToken),
		RefreshToken: strings.TrimSpace(state.RefreshToken),
		ExpiresAt:    state.ExpiresAt,
		AccountID:    strings.TrimSpace(state.AccountID),
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
	store, err := loadProviderCredentials(repoRoot, provider)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, nil
	}
	cred, ok := activeCredential(*store)
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

	stores, err := readProviderStores(repoRoot)
	if err != nil {
		return err
	}

	store := stores[id]
	cred = normalizeCredential(cred)
	if err := validateCredential(cred); err != nil {
		return err
	}

	index := matchCredential(store.Credentials, cred)
	if index >= 0 {
		if cred.ID == "" {
			cred.ID = store.Credentials[index].ID
		}
		store.Credentials[index] = mergeCredential(store.Credentials[index], cred)
	} else {
		cred.ID = ensureCredentialID(store.Credentials, cred)
		store.Credentials = append(store.Credentials, cred)
	}
	store.ActiveID = cred.ID
	stores[id] = normalizeProviderStore(store)

	return writeProviderStores(repoRoot, stores)
}

func SetActive(repoRoot, provider, credentialID string) error {
	store, err := loadProviderCredentials(repoRoot, provider)
	if err != nil {
		return err
	}
	if store == nil {
		return fmt.Errorf("no stored credentials for provider %s", provider)
	}
	credentialID = strings.TrimSpace(credentialID)
	if credentialID == "" {
		return fmt.Errorf("credential id is required")
	}

	for _, cred := range store.Credentials {
		if cred.ID == credentialID {
			store.ActiveID = credentialID
			return saveProviderCredentials(repoRoot, provider, *store)
		}
	}

	return fmt.Errorf("credential %s not found for provider %s", credentialID, provider)
}

func Remove(repoRoot, provider string) error {
	stores, err := readProviderStores(repoRoot)
	if err != nil {
		return err
	}
	id := strings.ToLower(strings.TrimSpace(provider))
	if id == "" {
		return fmt.Errorf("provider is required")
	}
	if _, ok := stores[id]; !ok {
		return nil
	}
	delete(stores, id)

	if len(stores) == 0 {
		return Clear(repoRoot)
	}
	return writeProviderStores(repoRoot, stores)
}

func RemoveCredential(repoRoot, provider, credentialID string) error {
	store, err := loadProviderCredentials(repoRoot, provider)
	if err != nil {
		return err
	}
	if store == nil {
		return fmt.Errorf("no stored credentials for provider %s", provider)
	}
	credentialID = strings.TrimSpace(credentialID)
	if credentialID == "" {
		return fmt.Errorf("credential id is required")
	}

	updated := make([]Credential, 0, len(store.Credentials))
	removed := false
	for _, cred := range store.Credentials {
		if cred.ID == credentialID {
			removed = true
			continue
		}
		updated = append(updated, cred)
	}
	if !removed {
		return fmt.Errorf("credential %s not found for provider %s", credentialID, provider)
	}
	if len(updated) == 0 {
		return Remove(repoRoot, provider)
	}

	store.Credentials = updated
	if strings.TrimSpace(store.ActiveID) == credentialID {
		store.ActiveID = updated[0].ID
	}
	return saveProviderCredentials(repoRoot, provider, *store)
}

func loadProviderCredentials(repoRoot, provider string) (*ProviderCredentials, error) {
	stores, err := readProviderStores(repoRoot)
	if err != nil {
		return nil, err
	}
	id := strings.ToLower(strings.TrimSpace(provider))
	if id == "" {
		return nil, fmt.Errorf("provider is required")
	}
	store, ok := stores[id]
	if !ok {
		return nil, nil
	}
	store = normalizeProviderStore(store)
	return &store, nil
}

func saveProviderCredentials(repoRoot, provider string, store ProviderCredentials) error {
	stores, err := readProviderStores(repoRoot)
	if err != nil {
		return err
	}
	id := strings.ToLower(strings.TrimSpace(provider))
	if id == "" {
		return fmt.Errorf("provider is required")
	}
	stores[id] = normalizeProviderStore(store)
	return writeProviderStores(repoRoot, stores)
}

func mutateCredential(repoRoot, provider, credentialID string, mutator func(*Credential) error) error {
	store, err := loadProviderCredentials(repoRoot, provider)
	if err != nil {
		return err
	}
	if store == nil {
		return fmt.Errorf("no stored credentials for provider %s", provider)
	}
	credentialID = strings.TrimSpace(credentialID)
	if credentialID == "" {
		return fmt.Errorf("credential id is required")
	}

	for i := range store.Credentials {
		if store.Credentials[i].ID != credentialID {
			continue
		}
		if err := mutator(&store.Credentials[i]); err != nil {
			return err
		}
		return saveProviderCredentials(repoRoot, provider, *store)
	}

	return fmt.Errorf("credential %s not found for provider %s", credentialID, provider)
}

func readProviderStores(repoRoot string) (map[string]ProviderCredentials, error) {
	path := filepath.Join(repoRoot, config.OrchDir, authFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]ProviderCredentials{}, nil
		}
		return nil, fmt.Errorf("read auth state: %w", err)
	}

	stores := map[string]ProviderCredentials{}
	if err := json.Unmarshal(data, &stores); err == nil {
		stores = normalizeProviderStores(stores)
		if len(stores) > 0 {
			return stores, nil
		}
	}

	legacy, legacyErr := loadLegacyCredentials(data)
	if legacyErr == nil && len(legacy) > 0 {
		return normalizeProviderStores(legacy), nil
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
		RefreshToken: strings.TrimSpace(state.RefreshToken),
		ExpiresAt:    state.ExpiresAt,
		AccountID:    strings.TrimSpace(state.AccountID),
		Email:        strings.TrimSpace(state.Email),
		UpdatedAt:    state.UpdatedAt,
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
	if strings.TrimSpace(cred.Type) == "" {
		return map[string]ProviderCredentials{}, nil
	}

	return normalizeProviderStores(map[string]ProviderCredentials{provider: {Credentials: []Credential{cred}}}), nil
}

func loadLegacyCredentials(data []byte) (map[string]ProviderCredentials, error) {
	parsed := map[string]Credential{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}

	stores := map[string]ProviderCredentials{}
	for provider, cred := range parsed {
		id := strings.ToLower(strings.TrimSpace(provider))
		if id == "" {
			continue
		}
		cred = normalizeCredential(cred)
		if strings.TrimSpace(cred.Type) == "" {
			continue
		}
		stores[id] = ProviderCredentials{Credentials: []Credential{cred}}
	}
	return stores, nil
}

func writeProviderStores(repoRoot string, stores map[string]ProviderCredentials) error {
	if len(stores) == 0 {
		return Clear(repoRoot)
	}
	if err := config.EnsureOrchDir(repoRoot); err != nil {
		return err
	}

	stores = normalizeProviderStores(stores)
	data, err := json.MarshalIndent(stores, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize auth state: %w", err)
	}

	path := filepath.Join(repoRoot, config.OrchDir, authFile)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write auth state: %w", err)
	}
	return nil
}

func normalizeProviderStores(stores map[string]ProviderCredentials) map[string]ProviderCredentials {
	normalized := map[string]ProviderCredentials{}
	for provider, store := range stores {
		id := strings.ToLower(strings.TrimSpace(provider))
		if id == "" {
			continue
		}
		store = normalizeProviderStore(store)
		if len(store.Credentials) == 0 {
			continue
		}
		normalized[id] = store
	}
	return normalized
}

func normalizeProviderStore(store ProviderCredentials) ProviderCredentials {
	seen := map[string]struct{}{}
	creds := make([]Credential, 0, len(store.Credentials))
	for _, cred := range store.Credentials {
		cred = normalizeCredential(cred)
		if strings.TrimSpace(cred.Type) == "" {
			continue
		}
		cred.ID = ensureCredentialID(creds, cred)
		if _, ok := seen[cred.ID]; ok {
			continue
		}
		seen[cred.ID] = struct{}{}
		creds = append(creds, cred)
	}
	store.Credentials = creds
	store.ActiveID = strings.TrimSpace(store.ActiveID)
	if len(store.Credentials) == 0 {
		store.ActiveID = ""
		return store
	}
	if store.ActiveID == "" || indexOfCredential(store.Credentials, store.ActiveID) < 0 {
		store.ActiveID = store.Credentials[0].ID
	}
	return store
}

func normalizeCredential(cred Credential) Credential {
	cred.ID = strings.TrimSpace(cred.ID)
	cred.Label = strings.TrimSpace(cred.Label)
	cred.Type = strings.ToLower(strings.TrimSpace(cred.Type))
	cred.Key = strings.TrimSpace(cred.Key)
	cred.AccessToken = strings.TrimSpace(cred.AccessToken)
	cred.RefreshToken = strings.TrimSpace(cred.RefreshToken)
	cred.AccountID = strings.TrimSpace(cred.AccountID)
	cred.Email = strings.TrimSpace(cred.Email)
	cred.LastError = strings.TrimSpace(cred.LastError)
	if cred.Type == "api_key" {
		cred.Type = "api"
	}
	if cred.Type == "account" {
		cred.Type = "oauth"
	}
	if cred.Label == "" {
		switch {
		case cred.Email != "":
			cred.Label = cred.Email
		case cred.AccountID != "":
			cred.Label = cred.AccountID
		default:
			cred.Label = cred.Type
		}
	}
	if cred.UpdatedAt.IsZero() {
		cred.UpdatedAt = time.Now().UTC()
	}
	return cred
}

func validateCredential(cred Credential) error {
	if cred.Type != "api" && cred.Type != "oauth" && cred.Type != "wellknown" {
		return fmt.Errorf("unsupported credential type: %s", cred.Type)
	}
	if cred.Type == "api" && strings.TrimSpace(cred.Key) == "" {
		return fmt.Errorf("api credential key cannot be empty")
	}
	if cred.Type == "oauth" && strings.TrimSpace(cred.AccessToken) == "" {
		return fmt.Errorf("oauth access token cannot be empty")
	}
	return nil
}

func activeCredential(store ProviderCredentials) (Credential, bool) {
	store = normalizeProviderStore(store)
	if len(store.Credentials) == 0 {
		return Credential{}, false
	}
	index := indexOfCredential(store.Credentials, store.ActiveID)
	if index < 0 {
		index = 0
	}
	return store.Credentials[index], true
}

func indexOfCredential(creds []Credential, credentialID string) int {
	for i, cred := range creds {
		if cred.ID == credentialID {
			return i
		}
	}
	return -1
}

func matchCredential(creds []Credential, incoming Credential) int {
	if incoming.ID != "" {
		return indexOfCredential(creds, incoming.ID)
	}
	if incoming.Type == "oauth" {
		for i, cred := range creds {
			if cred.Type != "oauth" {
				continue
			}
			if incoming.AccountID != "" && incoming.AccountID == cred.AccountID {
				return i
			}
			if incoming.Email != "" && strings.EqualFold(incoming.Email, cred.Email) {
				return i
			}
		}
	}
	if incoming.Type == "api" {
		for i, cred := range creds {
			if cred.Type == "api" {
				return i
			}
		}
	}
	return -1
}

func mergeCredential(existing, incoming Credential) Credential {
	merged := existing
	merged.Type = incoming.Type
	merged.Label = incoming.Label
	if incoming.Key != "" {
		merged.Key = incoming.Key
	}
	if incoming.AccessToken != "" {
		merged.AccessToken = incoming.AccessToken
	}
	if incoming.RefreshToken != "" {
		merged.RefreshToken = incoming.RefreshToken
	}
	if !incoming.ExpiresAt.IsZero() {
		merged.ExpiresAt = incoming.ExpiresAt
	}
	if incoming.AccountID != "" {
		merged.AccountID = incoming.AccountID
	}
	if incoming.Email != "" {
		merged.Email = incoming.Email
	}
	if !incoming.CooldownUntil.IsZero() || !merged.CooldownUntil.IsZero() {
		merged.CooldownUntil = incoming.CooldownUntil
	}
	if incoming.LastError != "" || merged.LastError != "" {
		merged.LastError = incoming.LastError
	}
	if !incoming.LastUsedAt.IsZero() || !merged.LastUsedAt.IsZero() {
		merged.LastUsedAt = incoming.LastUsedAt
	}
	merged.UpdatedAt = incoming.UpdatedAt
	return normalizeCredential(merged)
}

func ensureCredentialID(existing []Credential, cred Credential) string {
	if cred.ID != "" && indexOfCredential(existing, cred.ID) < 0 {
		return cred.ID
	}
	base := credentialIDBase(cred)
	if base == "" {
		base = cred.Type
	}
	base = sanitizeCredentialID(base)
	if base == "" {
		base = "credential"
	}
	id := base
	for suffix := 2; indexOfCredential(existing, id) >= 0; suffix++ {
		id = fmt.Sprintf("%s-%d", base, suffix)
	}
	return id
}

func credentialIDBase(cred Credential) string {
	switch {
	case cred.AccountID != "":
		return cred.AccountID
	case cred.Email != "":
		return strings.ToLower(cred.Email)
	case cred.Label != "":
		return strings.ToLower(cred.Label)
	case !cred.UpdatedAt.IsZero():
		return fmt.Sprintf("%s-%d", cred.Type, cred.UpdatedAt.Unix())
	default:
		return cred.Type
	}
}

func sanitizeCredentialID(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		case r == '@':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-._")
}
