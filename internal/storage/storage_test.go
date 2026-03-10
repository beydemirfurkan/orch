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
		TaskBrief: &models.TaskBrief{
			TaskID:         "task-1",
			UserRequest:    "save run state",
			NormalizedGoal: "Address task: save run state",
			TaskType:       models.TaskTypeChore,
			RiskLevel:      models.RiskLow,
		},
		Plan: &models.Plan{
			TaskID:    "task-1",
			Summary:   "Address task: save run state",
			TaskType:  models.TaskTypeChore,
			RiskLevel: models.RiskLow,
			Steps:     []models.PlanStep{{Order: 1, Description: "Persist the run state."}},
		},
		ExecutionContract: &models.ExecutionContract{
			TaskID:       "task-1",
			AllowedFiles: []string{"internal/storage/storage.go"},
			PatchBudget:  models.PatchBudget{MaxFiles: 1, MaxChangedLines: 20},
		},
		Patch: &models.Patch{
			TaskID: "task-1",
			Files: []models.PatchFile{{
				Path:   "README.md",
				Status: "modified",
				Diff:   "@@ -1 +1 @@\n-old\n+new\n",
			}},
			RawDiff: "diff --git a/README.md b/README.md\nindex 1111111..2222222 100644\n--- a/README.md\n+++ b/README.md\n@@ -1 +1 @@\n-old\n+new\n",
		},
		ValidationResults: []models.ValidationResult{{
			Name:     "task_brief_valid",
			Stage:    "planning",
			Status:   models.ValidationPass,
			Severity: models.SeverityLow,
			Summary:  "task brief persisted",
		}},
		RetryDirective: &models.RetryDirective{
			Stage:        "validation",
			Attempt:      1,
			FailedGates:  []string{"plan_compliance"},
			Instructions: []string{"Update the required file."},
		},
		Confidence: &models.ConfidenceReport{
			Score:   0.74,
			Band:    "medium",
			Reasons: []string{"validation and planning artifacts are present"},
		},
		TestFailures: []models.TestFailure{{
			Code:    "test_assertion_failure",
			Summary: "expected 200 got 500",
		}},
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

	latestState, err := store.GetLatestRunStateBySession(session.ID)
	if err != nil {
		t.Fatalf("get latest run state by session: %v", err)
	}
	if latestState == nil || latestState.ID != state.ID {
		t.Fatalf("unexpected latest run state: %+v", latestState)
	}

	loadedState, err := store.GetRunState(projectID, state.ID)
	if err != nil {
		t.Fatalf("get run state by id: %v", err)
	}
	if loadedState == nil || loadedState.Task.Description != "save run state" {
		t.Fatalf("unexpected loaded run state: %+v", loadedState)
	}

	projectStates, err := store.ListRunStatesByProject(projectID, 10)
	if err != nil {
		t.Fatalf("list run states by project: %v", err)
	}
	if len(projectStates) != 1 {
		t.Fatalf("expected one project state, got %d", len(projectStates))
	}

	patchText, err := store.LoadLatestPatchBySession(session.ID)
	if err != nil {
		t.Fatalf("load latest patch by session: %v", err)
	}
	if patchText != state.Patch.RawDiff {
		t.Fatalf("unexpected patch text loaded")
	}
}

func TestSessionMessagePartLifecycle(t *testing.T) {
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

	createdMsg, createdParts, err := store.CreateMessageWithParts(SessionMessage{
		SessionID:  session.ID,
		Role:       "user",
		ProviderID: "openai",
		ModelID:    "gpt-5.3-codex",
	}, []SessionPart{{
		Type:    "text",
		Payload: `{"text":"selam"}`,
	}})
	if err != nil {
		t.Fatalf("create message with parts: %v", err)
	}
	if createdMsg.ID == "" {
		t.Fatalf("expected message id")
	}
	if len(createdParts) != 1 {
		t.Fatalf("expected one part, got %d", len(createdParts))
	}
	if createdParts[0].MessageID != createdMsg.ID {
		t.Fatalf("unexpected part message id: got=%s want=%s", createdParts[0].MessageID, createdMsg.ID)
	}

	messages, err := store.ListSessionMessages(session.ID, 10)
	if err != nil {
		t.Fatalf("list session messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected one message, got %d", len(messages))
	}
	if messages[0].Role != "user" {
		t.Fatalf("unexpected message role: %s", messages[0].Role)
	}

	parts, err := store.ListSessionParts(createdMsg.ID)
	if err != nil {
		t.Fatalf("list session parts: %v", err)
	}
	if len(parts) != 1 {
		t.Fatalf("expected one part, got %d", len(parts))
	}
	if parts[0].Type != "text" {
		t.Fatalf("unexpected part type: %s", parts[0].Type)
	}
	if parts[0].Payload == "" {
		t.Fatalf("expected payload content")
	}
}

func TestSessionSummaryAndMetrics(t *testing.T) {
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

	if err := store.UpsertSessionSummary(session.ID, "## Goal\nShip session-only runtime"); err != nil {
		t.Fatalf("upsert session summary: %v", err)
	}
	summary, err := store.GetSessionSummary(session.ID)
	if err != nil {
		t.Fatalf("get session summary: %v", err)
	}
	if summary == nil {
		t.Fatalf("expected session summary")
	}
	if summary.SummaryText == "" {
		t.Fatalf("expected summary text")
	}

	err = store.UpsertSessionMetrics(SessionMetrics{
		SessionID:     session.ID,
		InputTokens:   120,
		OutputTokens:  35,
		TotalCost:     0.014,
		TurnCount:     2,
		LastMessageID: "msg-123",
	})
	if err != nil {
		t.Fatalf("upsert session metrics: %v", err)
	}
	metrics, err := store.GetSessionMetrics(session.ID)
	if err != nil {
		t.Fatalf("get session metrics: %v", err)
	}
	if metrics == nil {
		t.Fatalf("expected session metrics")
	}
	if metrics.InputTokens != 120 || metrics.OutputTokens != 35 || metrics.TurnCount != 2 {
		t.Fatalf("unexpected metrics payload: %+v", metrics)
	}
}

func TestCompactSessionParts(t *testing.T) {
	repoRoot := t.TempDir()

	store, err := Open(repoRoot)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	projectID, err := store.GetOrCreateProject()
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	session, err := store.EnsureDefaultSession(projectID)
	if err != nil {
		t.Fatalf("ensure default session: %v", err)
	}

	for i := 0; i < 4; i++ {
		msg, _, createErr := store.CreateMessageWithParts(SessionMessage{
			SessionID: session.ID,
			Role:      "user",
		}, []SessionPart{{Type: "text", Payload: `{"text":"payload"}`}})
		if createErr != nil {
			t.Fatalf("create message %d: %v", i, createErr)
		}
		parts, listErr := store.ListSessionParts(msg.ID)
		if listErr != nil || len(parts) != 1 {
			t.Fatalf("list parts for %s: %v", msg.ID, listErr)
		}
	}

	affected, err := store.CompactSessionParts(session.ID, 1)
	if err != nil {
		t.Fatalf("compact session parts: %v", err)
	}
	if affected == 0 {
		t.Fatalf("expected compacted rows")
	}

	messages, err := store.ListSessionMessages(session.ID, 10)
	if err != nil {
		t.Fatalf("list session messages: %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(messages))
	}

	compactedCount := 0
	for _, message := range messages {
		parts, partErr := store.ListSessionParts(message.ID)
		if partErr != nil {
			t.Fatalf("list parts: %v", partErr)
		}
		for _, part := range parts {
			if part.Compacted {
				compactedCount++
			}
		}
	}
	if compactedCount == 0 {
		t.Fatalf("expected some compacted parts")
	}
}
