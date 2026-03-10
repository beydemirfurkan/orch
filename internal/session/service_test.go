package session

import (
	"strings"
	"testing"

	"github.com/furkanbeydemir/orch/internal/storage"
)

func TestAppendTextAndListMessagesWithParts(t *testing.T) {
	repoRoot := t.TempDir()
	store, err := storage.Open(repoRoot)
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()

	projectID, err := store.GetOrCreateProject()
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	sess, err := store.EnsureDefaultSession(projectID)
	if err != nil {
		t.Fatalf("ensure default session: %v", err)
	}

	svc := NewService(store)
	created, err := svc.AppendText(MessageInput{
		SessionID:  sess.ID,
		Role:       "user",
		ProviderID: "openai",
		ModelID:    "gpt-5.3-codex",
		Text:       "selam",
	})
	if err != nil {
		t.Fatalf("append text: %v", err)
	}
	if created == nil {
		t.Fatalf("expected created message")
	}
	if len(created.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(created.Parts))
	}
	if text := ExtractTextPart(created.Parts[0]); text != "selam" {
		t.Fatalf("unexpected text payload: %q", text)
	}

	messages, err := svc.ListMessagesWithParts(sess.ID, 10)
	if err != nil {
		t.Fatalf("list messages with parts: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected one message, got %d", len(messages))
	}
	if messages[0].Message.Role != "user" {
		t.Fatalf("unexpected role: %s", messages[0].Message.Role)
	}
	if got := ExtractTextPart(messages[0].Parts[0]); got != "selam" {
		t.Fatalf("unexpected extracted text: %q", got)
	}
}

func TestMaybeCompact(t *testing.T) {
	repoRoot := t.TempDir()
	store, err := storage.Open(repoRoot)
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()

	projectID, err := store.GetOrCreateProject()
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	sess, err := store.EnsureDefaultSession(projectID)
	if err != nil {
		t.Fatalf("ensure default session: %v", err)
	}

	svc := NewService(store)
	veryLong := strings.Repeat("a", 320000)
	if _, err := svc.AppendText(MessageInput{SessionID: sess.ID, Role: "user", Text: veryLong}); err != nil {
		t.Fatalf("append large text: %v", err)
	}
	if _, err := svc.AppendText(MessageInput{SessionID: sess.ID, Role: "assistant", Text: "Updated file internal/session/service.go and docs/SESSION_ONLY_FULLSTACK_PLAN.md"}); err != nil {
		t.Fatalf("append path hint text: %v", err)
	}

	compacted, note, err := svc.MaybeCompact(sess.ID, "unknown-model")
	if err != nil {
		t.Fatalf("maybe compact: %v", err)
	}
	if !compacted {
		t.Fatalf("expected compaction to trigger")
	}
	if note == "" {
		t.Fatalf("expected compaction note")
	}

	summary, err := store.GetSessionSummary(sess.ID)
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}
	if summary == nil || strings.TrimSpace(summary.SummaryText) == "" {
		t.Fatalf("expected persisted summary")
	}
	if !strings.Contains(summary.SummaryText, "## Instructions") {
		t.Fatalf("expected instructions section in summary")
	}
	if !strings.Contains(summary.SummaryText, "## Relevant files/directories") {
		t.Fatalf("expected relevant files section in summary")
	}
}
