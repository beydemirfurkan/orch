package execution

import (
	"testing"

	"github.com/furkanbeydemir/orch/internal/models"
)

func TestPlanComplianceFailsWhenRequiredFilesAreMissing(t *testing.T) {
	guard := NewPlanComplianceGuard()
	plan := &models.Plan{
		FilesToModify: []string{"internal/auth/service.go", "internal/auth/store.go"},
		AcceptanceCriteria: []models.AcceptanceCriterion{{
			ID:          "ac-1",
			Description: "Race condition no longer occurs.",
		}},
	}
	contract := &models.ExecutionContract{AllowedFiles: []string{"internal/auth/service.go", "internal/auth/store.go"}}
	patch := &models.Patch{Files: []models.PatchFile{{Path: "internal/auth/service.go"}}}

	result := guard.Validate(plan, contract, patch)
	if result.Status != models.ValidationFail {
		t.Fatalf("expected failure, got %s", result.Status)
	}
	if len(result.Details) != 1 || result.Details[0] != "internal/auth/store.go" {
		t.Fatalf("unexpected missing files: %#v", result.Details)
	}
}

func TestPlanComplianceFailsOnForbiddenConfigChange(t *testing.T) {
	guard := NewPlanComplianceGuard()
	plan := &models.Plan{
		FilesToModify:      []string{"internal/auth/service.go"},
		AcceptanceCriteria: []models.AcceptanceCriterion{{ID: "ac-1", Description: "Behavior remains stable."}},
		ForbiddenChanges:   []string{"Do not change config files."},
	}
	patch := &models.Patch{Files: []models.PatchFile{{Path: "config/app.yaml"}, {Path: "internal/auth/service.go"}}}

	result := guard.Validate(plan, nil, patch)
	if result.Status != models.ValidationFail {
		t.Fatalf("expected failure, got %s", result.Status)
	}
}

func TestRetryDirectiveBuilderUsesValidationFailures(t *testing.T) {
	builder := NewRetryDirectiveBuilder()
	state := &models.RunState{
		ValidationResults: []models.ValidationResult{{
			Name:            "plan_compliance",
			Stage:           "validation",
			Status:          models.ValidationFail,
			Severity:        models.SeverityHigh,
			Summary:         "missing required file",
			ActionableItems: []string{"Modify the required file."},
		}},
	}

	directive := builder.FromValidation(state, 2)
	if directive == nil {
		t.Fatalf("expected directive")
	}
	if directive.Attempt != 2 || directive.Stage != "validation" {
		t.Fatalf("unexpected directive: %#v", directive)
	}
	if len(directive.FailedGates) != 1 || directive.FailedGates[0] != "plan_compliance" {
		t.Fatalf("unexpected failed gates: %#v", directive.FailedGates)
	}
	if len(directive.Instructions) == 0 {
		t.Fatalf("expected actionable retry instructions")
	}
}
