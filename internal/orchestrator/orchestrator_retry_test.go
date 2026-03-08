package orchestrator

import (
	"strings"
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
)

func TestRunEnforcesTestRetryLimit(t *testing.T) {
	repoRoot := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.Commands.Test = "false"
	cfg.Safety.FeatureFlags.RetryLimits = true
	cfg.Safety.Retry.TestMax = 2

	orch := New(cfg, repoRoot, false)
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
	cfg.Commands.Test = "true"

	orch := New(cfg, repoRoot, false)
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
}
