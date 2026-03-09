package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/agents"
	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
)

func TestRunClassifiesTestFailure(t *testing.T) {
	repoRoot := t.TempDir()

	scriptPath := filepath.Join(repoRoot, "testfail.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho '--- FAIL: TestAuth' >&2\necho 'expected 200 got 500' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write test script: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Commands.Test = "sh testfail.sh"
	cfg.Safety.FeatureFlags.RetryLimits = false

	orch := New(cfg, repoRoot, false)
	orch.planner = agentStub{name: "planner", execute: func(input *agents.Input) (*agents.Output, error) {
		return &agents.Output{Plan: &models.Plan{
			TaskID:             input.Task.ID,
			Summary:            "classify test failure plan",
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

	state, err := orch.Run(&models.Task{ID: "task-test-classifier", Description: "classify failing tests", CreatedAt: time.Now()})
	if err == nil {
		t.Fatalf("expected run to fail on test command")
	}
	if state == nil {
		t.Fatalf("expected run state")
	}
	if len(state.TestFailures) == 0 {
		t.Fatalf("expected classified test failures")
	}
	if state.TestFailures[0].Code != "test_assertion_failure" {
		t.Fatalf("unexpected test failure code: %s", state.TestFailures[0].Code)
	}
	found := false
	for _, result := range state.ValidationResults {
		if result.Name == "required_tests_passed" && result.Status == models.ValidationFail && strings.Contains(result.Summary, "test_assertion_failure") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected test gate failure to be recorded")
	}
}
