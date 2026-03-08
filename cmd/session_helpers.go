package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/furkanbeydemir/orch/internal/storage"
)

type sessionContext struct {
	Store     *storage.Store
	ProjectID string
	Session   storage.Session
}

func (c *sessionContext) ExecutionRoot(defaultRoot string) string {
	if c == nil {
		return defaultRoot
	}
	worktree := strings.TrimSpace(c.Session.Worktree)
	if worktree == "" {
		return defaultRoot
	}
	if filepath.IsAbs(worktree) {
		return worktree
	}
	return filepath.Join(defaultRoot, worktree)
}

func loadSessionContext(repoRoot string) (*sessionContext, error) {
	store, err := storage.Open(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to open session storage: %w", err)
	}

	projectID, err := store.GetOrCreateProject()
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("failed to resolve project: %w", err)
	}

	session, err := store.EnsureDefaultSession(projectID)
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("failed to resolve active session: %w", err)
	}

	return &sessionContext{
		Store:     store,
		ProjectID: projectID,
		Session:   session,
	}, nil
}
