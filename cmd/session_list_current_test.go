package cmd

import (
	"encoding/json"
	"testing"

	"github.com/furkanbeydemir/orch/internal/storage"
)

func TestSessionListJSONOutput(t *testing.T) {
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
	if _, err := store.EnsureDefaultSession(projectID); err != nil {
		t.Fatalf("default session: %v", err)
	}

	sessionListJSON = true
	t.Cleanup(func() { sessionListJSON = false })

	output := captureStdout(t, func() {
		if err := runSessionList(nil, nil); err != nil {
			t.Fatalf("run session list: %v", err)
		}
	})

	var payload []map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("invalid json output: %v\noutput=%s", err, output)
	}
	if len(payload) == 0 {
		t.Fatalf("expected at least one session")
	}
	if payload[0]["name"] == "" {
		t.Fatalf("expected name in first session payload")
	}
}

func TestSessionCurrentJSONOutput(t *testing.T) {
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

	sessionCurrentJSON = true
	t.Cleanup(func() { sessionCurrentJSON = false })

	output := captureStdout(t, func() {
		if err := runSessionCurrent(nil, nil); err != nil {
			t.Fatalf("run session current: %v", err)
		}
	})

	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("invalid json output: %v\noutput=%s", err, output)
	}
	if payload["id"] != sess.ID {
		t.Fatalf("expected id %s, got %v", sess.ID, payload["id"])
	}
	if payload["is_active"] != true {
		t.Fatalf("expected is_active=true, got %v", payload["is_active"])
	}
}
