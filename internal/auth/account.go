package auth

import (
	"fmt"
	"strings"
	"time"
)

const refreshSkew = 30 * time.Second

func ResolveAccountCredential(repoRoot, provider string) (*Credential, error) {
	return resolveAccountCredentialByID(repoRoot, provider, "")
}

func resolveAccountCredentialByID(repoRoot, provider, credentialID string) (*Credential, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return nil, fmt.Errorf("provider is required")
	}

	var (
		cred *Credential
		err  error
	)
	if strings.TrimSpace(credentialID) == "" {
		cred, err = Get(repoRoot, provider)
	} else {
		credentials, _, listErr := List(repoRoot, provider)
		if listErr != nil {
			return nil, listErr
		}
		for i := range credentials {
			if credentials[i].ID == credentialID {
				copy := credentials[i]
				cred = &copy
				break
			}
		}
	}
	if err != nil {
		return nil, err
	}
	if cred == nil {
		return nil, fmt.Errorf("no stored credential for provider %s", provider)
	}
	if strings.ToLower(strings.TrimSpace(cred.Type)) != "oauth" {
		return nil, fmt.Errorf("stored credential for %s is not oauth", provider)
	}
	if strings.TrimSpace(cred.AccessToken) == "" {
		return nil, fmt.Errorf("stored oauth access token is empty for %s", provider)
	}

	if !shouldRefresh(cred) {
		return cred, nil
	}
	if strings.TrimSpace(cred.RefreshToken) == "" {
		return nil, fmt.Errorf("oauth access token expired and no refresh token is available")
	}

	refreshed, err := RefreshOAuthToken(cred.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh oauth token: %w", err)
	}

	cred.AccessToken = strings.TrimSpace(refreshed.AccessToken)
	if strings.TrimSpace(refreshed.RefreshToken) != "" {
		cred.RefreshToken = strings.TrimSpace(refreshed.RefreshToken)
	}
	cred.ExpiresAt = refreshed.ExpiresAt
	if strings.TrimSpace(refreshed.AccountID) != "" {
		cred.AccountID = strings.TrimSpace(refreshed.AccountID)
	}
	if strings.TrimSpace(refreshed.Email) != "" {
		cred.Email = strings.TrimSpace(refreshed.Email)
	}

	if err := Set(repoRoot, provider, *cred); err != nil {
		return nil, err
	}

	if strings.TrimSpace(credentialID) == "" {
		return Get(repoRoot, provider)
	}
	return resolveAccountCredentialByID(repoRoot, provider, credentialID)
}

func ResolveAccountAccessToken(repoRoot, provider string) (string, error) {
	cred, err := ResolveAccountCredential(repoRoot, provider)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(cred.AccessToken), nil
}

func shouldRefresh(cred *Credential) bool {
	if cred == nil {
		return false
	}
	if cred.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().UTC().After(cred.ExpiresAt.Add(-refreshSkew))
}
