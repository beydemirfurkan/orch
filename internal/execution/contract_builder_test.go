package execution

import (
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
)

func TestContractBuilderBuildsStructuredContract(t *testing.T) {
	builder := NewContractBuilder(config.DefaultConfig())
	task := &models.Task{ID: "task-1", Description: "fix race condition in auth service", CreatedAt: time.Now()}
	brief := &models.TaskBrief{TaskID: "task-1", TaskType: models.TaskTypeBugfix, RiskLevel: models.RiskHigh}
	plan := &models.Plan{
		TaskID:         "task-1",
		FilesToModify:  []string{"internal/auth/service.go"},
		FilesToInspect: []string{"internal/auth/service.go", "internal/auth/service_test.go"},
		AcceptanceCriteria: []models.AcceptanceCriterion{{
			ID:          "ac-1",
			Description: "Race condition is no longer reproducible.",
		}},
		Invariants:       []string{"Public API remains unchanged."},
		ForbiddenChanges: []string{"Do not change config files."},
		Steps: []models.PlanStep{{
			Order:       1,
			Description: "Protect auth state mutation.",
		}},
	}
	ctx := &models.ContextResult{
		SelectedFiles: []string{"internal/auth/service.go"},
		RelatedTests:  []string{"internal/auth/service_test.go"},
	}

	contract := builder.Build(task, brief, plan, ctx)
	if contract == nil {
		t.Fatalf("expected contract")
	}
	if len(contract.AllowedFiles) != 1 || contract.AllowedFiles[0] != "internal/auth/service.go" {
		t.Fatalf("unexpected allowed files: %#v", contract.AllowedFiles)
	}
	if len(contract.InspectFiles) < 2 {
		t.Fatalf("expected inspect files to include plan and test context")
	}
	if len(contract.RequiredEdits) == 0 {
		t.Fatalf("expected required edits")
	}
	if len(contract.ProhibitedActions) == 0 {
		t.Fatalf("expected prohibited actions")
	}
	if contract.PatchBudget.MaxFiles <= 0 || contract.PatchBudget.MaxChangedLines <= 0 {
		t.Fatalf("expected patch budget from config")
	}
}

func TestScopeGuardRejectsOutOfScopePatch(t *testing.T) {
	guard := NewScopeGuard()
	contract := &models.ExecutionContract{AllowedFiles: []string{"internal/auth/service.go"}}
	patch := &models.Patch{Files: []models.PatchFile{{Path: "internal/auth/service.go"}, {Path: "internal/auth/config.go"}}}

	result := guard.Validate(contract, patch)
	if result.Status != models.ValidationFail {
		t.Fatalf("expected validation failure, got %s", result.Status)
	}
	if len(result.Details) != 1 || result.Details[0] != "internal/auth/config.go" {
		t.Fatalf("unexpected scope violation details: %#v", result.Details)
	}
}
