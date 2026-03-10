package cmd

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/storage"
)

func TestSessionRunsJSONOutput(t *testing.T) {
	repoRoot := t.TempDir()
	t.Chdir(repoRoot)

	store, err := storage.Open(repoRoot)
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()

	projectID, err := store.GetOrCreateProject()
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	sess, err := store.EnsureDefaultSession(projectID)
	if err != nil {
		t.Fatalf("default session: %v", err)
	}

	err = store.SaveRunState(&models.RunState{
		ID:        "run-json-1",
		ProjectID: projectID,
		SessionID: sess.ID,
		Task: models.Task{
			ID:          "task-json-1",
			Description: "verify runs json output",
			CreatedAt:   time.Now(),
		},
		Status:    models.StatusCompleted,
		Retries:   models.RetryState{},
		StartedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("save run state: %v", err)
	}

	sessionRunsLimit = 10
	sessionRunsJSON = true
	t.Cleanup(func() { sessionRunsJSON = false })

	output := captureStdout(t, func() {
		if err := runSessionRuns(nil, nil); err != nil {
			t.Fatalf("run session runs: %v", err)
		}
	})

	var payload []map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("invalid json output: %v\noutput=%s", err, output)
	}
	if len(payload) == 0 {
		t.Fatalf("expected at least one run in json output")
	}
	if payload[0]["id"] != "run-json-1" {
		t.Fatalf("unexpected run id in output: %v", payload[0]["id"])
	}
}
