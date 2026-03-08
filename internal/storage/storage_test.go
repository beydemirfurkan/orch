package storage

import (
	"errors"
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/models"
)

func TestProjectAndDefaultSessionBootstrap(t *testing.T) {
	repoRoot := t.TempDir()

	store, err := Open(repoRoot)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	projectID, err := store.GetOrCreateProject()
	if err != nil {
		t.Fatalf("get or create project: %v", err)
	}

	session, err := store.EnsureDefaultSession(projectID)
	if err != nil {
		t.Fatalf("ensure default session: %v", err)
	}
	if session.Name != "default" {
		t.Fatalf("unexpected default session name: %s", session.Name)
	}

	active, err := store.GetActiveSession(projectID)
	if err != nil {
		t.Fatalf("get active session: %v", err)
	}
	if active.ID != session.ID {
		t.Fatalf("active session mismatch: got=%s want=%s", active.ID, session.ID)
	}
}

func TestSessionLifecycle(t *testing.T) {
	repoRoot := t.TempDir()

	store, err := Open(repoRoot)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	projectID, err := store.GetOrCreateProject()
	if err != nil {
		t.Fatalf("get or create project: %v", err)
	}

	_, err = store.EnsureDefaultSession(projectID)
	if err != nil {
		t.Fatalf("ensure default session: %v", err)
	}

	feature, err := store.CreateSession(projectID, "feature-x")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	selected, err := store.SelectSession(projectID, "feature-x")
	if err != nil {
		t.Fatalf("select session: %v", err)
	}
	if selected.ID != feature.ID {
		t.Fatalf("selected session mismatch: got=%s want=%s", selected.ID, feature.ID)
	}

	if err := store.CloseSession(projectID, "feature-x"); err != nil {
		t.Fatalf("close session: %v", err)
	}

	_, err = store.SelectSession(projectID, "feature-x")
	if !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("expected closed session error, got: %v", err)
	}
}

func TestSaveRunState(t *testing.T) {
	repoRoot := t.TempDir()

	store, err := Open(repoRoot)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	projectID, err := store.GetOrCreateProject()
	if err != nil {
		t.Fatalf("get or create project: %v", err)
	}
	session, err := store.EnsureDefaultSession(projectID)
	if err != nil {
		t.Fatalf("ensure default session: %v", err)
	}

	state := &models.RunState{
		ID:        "run-test-1",
		ProjectID: projectID,
		SessionID: session.ID,
		Task: models.Task{
			ID:          "task-1",
			Description: "save run state",
			CreatedAt:   time.Now(),
		},
		Status:    models.StatusCompleted,
		StartedAt: time.Now(),
	}

	if err := store.SaveRunState(state); err != nil {
		t.Fatalf("save run state: %v", err)
	}

	runs, err := store.ListRunsBySession(session.ID, 10)
	if err != nil {
		t.Fatalf("list runs by session: %v", err)
	}
	if len(runs) == 0 {
		t.Fatalf("expected at least one run record")
	}
	if runs[0].ID != state.ID {
		t.Fatalf("unexpected run id: got=%s want=%s", runs[0].ID, state.ID)
	}

	filteredByStatus, err := store.ListRunsBySessionFiltered(session.ID, 10, string(models.StatusCompleted), "")
	if err != nil {
		t.Fatalf("filter runs by status: %v", err)
	}
	if len(filteredByStatus) != 1 {
		t.Fatalf("expected one completed run, got=%d", len(filteredByStatus))
	}

	filteredByText, err := store.ListRunsBySessionFiltered(session.ID, 10, "", "save run")
	if err != nil {
		t.Fatalf("filter runs by task text: %v", err)
	}
	if len(filteredByText) != 1 {
		t.Fatalf("expected one text-matched run, got=%d", len(filteredByText))
	}
}
