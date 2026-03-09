package runstore

import (
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/models"
)

func TestListRunStatesSortedAndLimited(t *testing.T) {
	repoRoot := t.TempDir()
	now := time.Now().UTC()

	states := []*models.RunState{
		{ID: "run-older", Task: models.Task{ID: "task-1", Description: "older", CreatedAt: now}, Status: models.StatusCompleted, StartedAt: now.Add(-2 * time.Hour)},
		{ID: "run-newer", Task: models.Task{ID: "task-2", Description: "newer", CreatedAt: now}, Status: models.StatusFailed, StartedAt: now.Add(-1 * time.Hour)},
	}
	for _, state := range states {
		if err := SaveRunState(repoRoot, state); err != nil {
			t.Fatalf("save run state %s: %v", state.ID, err)
		}
	}

	loaded, err := ListRunStates(repoRoot, 1)
	if err != nil {
		t.Fatalf("list run states: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 run, got %d", len(loaded))
	}
	if loaded[0].ID != "run-newer" {
		t.Fatalf("expected newest run first, got %s", loaded[0].ID)
	}
}

func TestLoadRunState(t *testing.T) {
	repoRoot := t.TempDir()
	state := &models.RunState{
		ID:         "run-1",
		Task:       models.Task{ID: "task-1", Description: "demo", CreatedAt: time.Now()},
		Status:     models.StatusCompleted,
		StartedAt:  time.Now(),
		Confidence: &models.ConfidenceReport{Score: 0.88, Band: "high"},
	}
	if err := SaveRunState(repoRoot, state); err != nil {
		t.Fatalf("save run state: %v", err)
	}

	loaded, err := LoadRunState(repoRoot, state.ID)
	if err != nil {
		t.Fatalf("load run state: %v", err)
	}
	if loaded.ID != state.ID {
		t.Fatalf("unexpected run id: got=%s want=%s", loaded.ID, state.ID)
	}
	if loaded.Confidence == nil || loaded.Confidence.Band != "high" {
		t.Fatalf("expected confidence report to roundtrip")
	}
}
