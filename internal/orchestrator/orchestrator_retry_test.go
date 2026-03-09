package orchestrator

import (
	"strings"
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/agents"
	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
)

type agentStub struct {
	name    string
	execute func(input *agents.Input) (*agents.Output, error)
}

func (a agentStub) Name() string { return a.name }

func (a agentStub) Execute(input *agents.Input) (*agents.Output, error) {
	return a.execute(input)
}

func TestRunEnforcesTestRetryLimit(t *testing.T) {
	repoRoot := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.Commands.Test = "false"
	cfg.Safety.FeatureFlags.RetryLimits = true
	cfg.Safety.Retry.TestMax = 2

	orch := New(cfg, repoRoot, false)
	orch.planner = agentStub{name: "planner", execute: func(input *agents.Input) (*agents.Output, error) {
		return &agents.Output{Plan: &models.Plan{
			TaskID:             input.Task.ID,
			Summary:            "retry test plan",
			TaskType:           models.TaskTypeBugfix,
			RiskLevel:          models.RiskMedium,
			FilesToModify:      []string{"demo.go"},
			FilesToInspect:     []string{"demo.go"},
			AcceptanceCriteria: []models.AcceptanceCriterion{{ID: "ac-1", Description: "Patch updates demo.go."}},
			TestRequirements:   []string{"Run configured test command."},
			Steps:              []models.PlanStep{{Order: 1, Description: "Modify demo.go."}},
		}}, nil
	}}
	orch.coder = agentStub{name: "coder", execute: func(input *agents.Input) (*agents.Output, error) {
		return &agents.Output{Patch: &models.Patch{TaskID: input.Task.ID, RawDiff: "diff --git a/demo.go b/demo.go\n--- a/demo.go\n+++ b/demo.go\n@@ -1 +1 @@\n-old\n+new\n"}}, nil
	}}
	task := &models.Task{ID: "task-1", Description: "retry test", CreatedAt: time.Now()}

	state, err := orch.Run(task)
	if err == nil {
		t.Fatalf("expected run to fail when test command always fails")
	}
	if state == nil {
		t.Fatalf("expected run state")
	}

	if state.Retries.Testing != 2 {
		t.Fatalf("unexpected testing retries. got=%d want=%d", state.Retries.Testing, 2)
	}

	if len(state.UnresolvedFailures) == 0 {
		t.Fatalf("expected unresolved failure summary to be recorded")
	}
}

func TestRunLogsToolPolicyDecisions(t *testing.T) {
	repoRoot := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.Commands.Test = "printf ok"

	orch := New(cfg, repoRoot, false)
	orch.planner = agentStub{name: "planner", execute: func(input *agents.Input) (*agents.Output, error) {
		return &agents.Output{Plan: &models.Plan{
			TaskID:             input.Task.ID,
			Summary:            "policy log test plan",
			TaskType:           models.TaskTypeBugfix,
			RiskLevel:          models.RiskMedium,
			FilesToModify:      []string{"demo.go"},
			FilesToInspect:     []string{"demo.go"},
			AcceptanceCriteria: []models.AcceptanceCriterion{{ID: "ac-1", Description: "Patch updates demo.go."}},
			TestRequirements:   []string{"Run configured test command."},
			Steps:              []models.PlanStep{{Order: 1, Description: "Modify demo.go."}},
		}}, nil
	}}
	orch.coder = agentStub{name: "coder", execute: func(input *agents.Input) (*agents.Output, error) {
		return &agents.Output{Patch: &models.Patch{TaskID: input.Task.ID, RawDiff: "diff --git a/demo.go b/demo.go\n--- a/demo.go\n+++ b/demo.go\n@@ -1 +1 @@\n-old\n+new\n"}}, nil
	}}
	task := &models.Task{ID: "task-2", Description: "policy log test", CreatedAt: time.Now()}

	state, err := orch.Run(task)
	if err != nil {
		t.Fatalf("expected run to complete: %v", err)
	}

	found := false
	for _, entry := range state.Logs {
		if entry.Actor == "policy" && entry.Step == "decision" && strings.Contains(entry.Message, "tool=run_tests") {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected policy decision log for run_tests tool")
	}

	if state.Context == nil {
		t.Fatalf("expected context to be built and attached to run state")
	}
	if state.TaskBrief == nil {
		t.Fatalf("expected task brief to be attached to run state")
	}
	if state.Plan == nil || len(state.Plan.AcceptanceCriteria) == 0 {
		t.Fatalf("expected structured plan acceptance criteria to be attached to run state")
	}
	if state.ExecutionContract == nil || len(state.ExecutionContract.AllowedFiles) == 0 {
		t.Fatalf("expected execution contract to be attached to run state")
	}
	if len(state.ValidationResults) == 0 {
		t.Fatalf("expected validation results to be attached to run state")
	}
	foundPlanCompliance := false
	for _, result := range state.ValidationResults {
		if result.Name == "plan_compliance" {
			foundPlanCompliance = true
			break
		}
	}
	if !foundPlanCompliance {
		t.Fatalf("expected plan_compliance validation result")
	}
}
