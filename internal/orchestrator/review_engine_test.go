package orchestrator

import (
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/agents"
	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
)

func TestRunAttachesReviewScorecard(t *testing.T) {
	repoRoot := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.Commands.Test = "printf ok"

	orch := New(cfg, repoRoot, false)
	orch.planner = agentStub{name: "planner", execute: func(input *agents.Input) (*agents.Output, error) {
		return &agents.Output{Plan: &models.Plan{
			TaskID:             input.Task.ID,
			Summary:            "health endpoint plan",
			TaskType:           models.TaskTypeFeature,
			RiskLevel:          models.RiskMedium,
			FilesToModify:      []string{"health.go"},
			FilesToInspect:     []string{"health.go"},
			AcceptanceCriteria: []models.AcceptanceCriterion{{ID: "ac-1", Description: "Health endpoint is implemented."}},
			TestRequirements:   []string{"Run configured test command."},
			Steps:              []models.PlanStep{{Order: 1, Description: "Modify health.go."}},
		}}, nil
	}}
	orch.coder = agentStub{name: "coder", execute: func(input *agents.Input) (*agents.Output, error) {
		return &agents.Output{Patch: &models.Patch{TaskID: input.Task.ID, RawDiff: "diff --git a/health.go b/health.go\n--- a/health.go\n+++ b/health.go\n@@ -1 +1 @@\n-old\n+new\n"}}, nil
	}}
	task := &models.Task{ID: "task-review-1", Description: "add health endpoint", CreatedAt: time.Now()}

	state, err := orch.Run(task)
	if err != nil {
		t.Fatalf("expected run to complete: %v", err)
	}
	if state.ReviewScorecard == nil {
		t.Fatalf("expected review scorecard to be attached to run state")
	}
	if state.Review == nil {
		t.Fatalf("expected final review result")
	}
	if state.Confidence == nil {
		t.Fatalf("expected confidence report to be attached to run state")
	}
}
