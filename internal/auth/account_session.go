package auth

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type AccountSession struct {
	repoRoot  string
	provider  string
	currentID string
	excluded  map[string]struct{}
	notice    string
}

func NewAccountSession(repoRoot, provider string) *AccountSession {
	return &AccountSession{
		repoRoot: strings.TrimSpace(repoRoot),
		provider: strings.ToLower(strings.TrimSpace(provider)),
		excluded: map[string]struct{}{},
	}
}

func (s *AccountSession) ResolveToken(ctx context.Context) (string, error) {
	_ = ctx
	cred, err := s.pickCredential()
	if err != nil {
		return "", err
	}
	if cred == nil {
		return "", fmt.Errorf("no active oauth credential available for provider %s", s.provider)
	}
	s.currentID = cred.ID
	return strings.TrimSpace(cred.AccessToken), nil
}

func (s *AccountSession) Failover(ctx context.Context, cooldown time.Duration, reason string) (string, bool, error) {
	_ = ctx
	if strings.TrimSpace(s.currentID) == "" {
		return "", false, nil
	}
	fromID := s.currentID
	if err := mutateCredential(s.repoRoot, s.provider, s.currentID, func(cred *Credential) error {
		if cooldown > 0 {
			cred.CooldownUntil = time.Now().UTC().Add(cooldown)
		}
		cred.LastError = strings.TrimSpace(reason)
		cred.UpdatedAt = time.Now().UTC()
		return nil
	}); err != nil {
		return "", false, err
	}
	s.excluded[s.currentID] = struct{}{}

	cred, err := s.pickCredential()
	if err != nil {
		return "", false, err
	}
	if cred == nil {
		return "", false, nil
	}
	s.currentID = cred.ID
	s.notice = buildFailoverNotice(fromID, cred.ID, reason)
	return strings.TrimSpace(cred.AccessToken), true, nil
}

func (s *AccountSession) MarkSuccess(ctx context.Context) {
	_ = ctx
	if strings.TrimSpace(s.currentID) == "" {
		return
	}
	_ = mutateCredential(s.repoRoot, s.provider, s.currentID, func(cred *Credential) error {
		cred.LastError = ""
		cred.CooldownUntil = time.Time{}
		cred.LastUsedAt = time.Now().UTC()
		cred.UpdatedAt = time.Now().UTC()
		return nil
	})
	s.excluded = map[string]struct{}{}
}

func (s *AccountSession) ConsumeNotice() string {
	notice := strings.TrimSpace(s.notice)
	s.notice = ""
	return notice
}

func (s *AccountSession) pickCredential() (*Credential, error) {
	credentials, activeID, err := List(s.repoRoot, s.provider)
	if err != nil {
		return nil, err
	}
	if len(credentials) == 0 {
		return nil, nil
	}

	now := time.Now().UTC()
	ordered := orderCredentials(credentials, activeID, s.currentID)
	for _, candidate := range ordered {
		if candidate.Type != "oauth" {
			continue
		}
		if _, skip := s.excluded[candidate.ID]; skip {
			continue
		}
		if !candidate.CooldownUntil.IsZero() && candidate.CooldownUntil.After(now) {
			continue
		}
		if candidate.ID != activeID {
			if err := SetActive(s.repoRoot, s.provider, candidate.ID); err != nil {
				return nil, err
			}
		}
		resolved, err := resolveAccountCredentialByID(s.repoRoot, s.provider, candidate.ID)
		if err == nil {
			return resolved, nil
		}
		_ = mutateCredential(s.repoRoot, s.provider, candidate.ID, func(cred *Credential) error {
			cred.LastError = strings.TrimSpace(err.Error())
			cred.CooldownUntil = now.Add(5 * time.Minute)
			cred.UpdatedAt = now
			return nil
		})
		s.excluded[candidate.ID] = struct{}{}
	}

	return nil, nil
}

func orderCredentials(credentials []Credential, activeID, currentID string) []Credential {
	ordered := make([]Credential, 0, len(credentials))
	appendByID := func(id string) {
		if strings.TrimSpace(id) == "" {
			return
		}
		for _, cred := range credentials {
			if cred.ID == id && !containsCredential(ordered, id) {
				ordered = append(ordered, cred)
				return
			}
		}
	}
	appendByID(currentID)
	appendByID(activeID)
	for _, cred := range credentials {
		if containsCredential(ordered, cred.ID) {
			continue
		}
		ordered = append(ordered, cred)
	}
	return ordered
}

func containsCredential(credentials []Credential, credentialID string) bool {
	for _, cred := range credentials {
		if cred.ID == credentialID {
			return true
		}
	}
	return false
}

func buildFailoverNotice(fromID, toID, reason string) string {
	reason = strings.TrimSpace(reason)
	message := fmt.Sprintf("OpenAI account failover: switched from %s to %s", fromID, toID)
	if reason != "" {
		message += " after " + reason
	}
	return message
}
