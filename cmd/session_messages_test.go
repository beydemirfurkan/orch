package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/furkanbeydemir/orch/internal/session"
	"github.com/furkanbeydemir/orch/internal/storage"
)

func TestSessionMessagesCommand(t *testing.T) {
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

	svc := session.NewService(store)
	if _, err := svc.AppendText(session.MessageInput{
		SessionID:  sess.ID,
		Role:       "user",
		ProviderID: "openai",
		ModelID:    "gpt-5.3-codex",
		Text:       "hello session",
	}); err != nil {
		t.Fatalf("append text: %v", err)
	}

	sessionMessagesLimit = 10
	sessionMessagesJSON = false
	output := captureStdout(t, func() {
		if err := runSessionMessages(nil, nil); err != nil {
			t.Fatalf("run session messages: %v", err)
		}
	})

	if !strings.Contains(output, "role=user") {
		t.Fatalf("expected role in output, got: %s", output)
	}
	if !strings.Contains(output, "hello session") {
		t.Fatalf("expected message text in output, got: %s", output)
	}
}

func TestSessionMessagesCommandJSONOutput(t *testing.T) {
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

	svc := session.NewService(store)
	if _, err := svc.AppendText(session.MessageInput{SessionID: sess.ID, Role: "user", Text: "json output test"}); err != nil {
		t.Fatalf("append text: %v", err)
	}

	sessionMessagesLimit = 10
	sessionMessagesJSON = true
	t.Cleanup(func() { sessionMessagesJSON = false })

	output := captureStdout(t, func() {
		if err := runSessionMessages(nil, nil); err != nil {
			t.Fatalf("run session messages: %v", err)
		}
	})

	var payload []map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("invalid json output: %v\noutput=%s", err, output)
	}
	if len(payload) == 0 {
		t.Fatalf("expected at least one message in json output")
	}
	if payload[0]["role"] != "user" {
		t.Fatalf("expected first message role=user, got: %v", payload[0]["role"])
	}
	parts, ok := payload[0]["parts"].([]any)
	if !ok || len(parts) == 0 {
		t.Fatalf("expected parts in json output, got: %#v", payload[0]["parts"])
	}
}

func TestSessionMessagesCommandRendersStageAndCompaction(t *testing.T) {
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

	svc := session.NewService(store)
	stagePayload, _ := json.Marshal(map[string]any{
		"actor":     "planner",
		"step":      "plan",
		"message":   "Generating plan...",
		"timestamp": "2026-03-09T00:00:00Z",
	})
	compactionPayload, _ := json.Marshal(map[string]any{
		"estimated_tokens": 70000,
		"usable_input":     56000,
		"summary":          "Compaction summary content.",
	})
	if _, err := svc.AppendMessage(session.MessageInput{SessionID: sess.ID, Role: "assistant"}, []storage.SessionPart{
		{Type: "stage", Payload: string(stagePayload)},
		{Type: "compaction", Payload: string(compactionPayload)},
		{Type: "stage", Payload: "{not-json"},
	}); err != nil {
		t.Fatalf("append stage/compaction parts: %v", err)
	}

	sessionMessagesLimit = 10
	output := captureStdout(t, func() {
		if err := runSessionMessages(nil, nil); err != nil {
			t.Fatalf("run session messages: %v", err)
		}
	})

	if !strings.Contains(output, "part=stage actor=\"planner\" step=\"plan\"") {
		t.Fatalf("expected structured stage render, got: %s", output)
	}
	if !strings.Contains(output, "part=compaction estimated_tokens=70000") {
		t.Fatalf("expected structured compaction render, got: %s", output)
	}
	if !strings.Contains(output, "part=stage payload=") {
		t.Fatalf("expected malformed stage fallback render, got: %s", output)
	}
}
