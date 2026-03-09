package review

import (
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/models"
)

func TestEvaluateAcceptsHealthyRun(t *testing.T) {
	engine := NewEngine()
	state := &models.RunState{
		Task:      models.Task{ID: "task-1", Description: "fix auth bug", CreatedAt: time.Now()},
		TaskBrief: &models.TaskBrief{TaskID: "task-1", TaskType: models.TaskTypeBugfix, RiskLevel: models.RiskMedium},
		Plan: &models.Plan{
			TaskID:             "task-1",
			AcceptanceCriteria: []models.AcceptanceCriterion{{ID: "ac-1", Description: "Bug no longer occurs."}},
			TestRequirements:   []string{"Run go test ./..."},
		},
		ExecutionContract: &models.ExecutionContract{AllowedFiles: []string{"internal/auth/service.go"}},
		Patch: &models.Patch{
			RawDiff: "diff --git a/internal/auth/service.go b/internal/auth/service.go\n--- a/internal/auth/service.go\n+++ b/internal/auth/service.go\n@@ -1 +1 @@\n-old\n+new\n",
			Files:   []models.PatchFile{{Path: "internal/auth/service.go"}},
		},
		ValidationResults: []models.ValidationResult{
			{Name: "patch_hygiene", Status: models.ValidationPass},
			{Name: "scope_compliance", Status: models.ValidationPass},
			{Name: "plan_compliance", Status: models.ValidationPass},
		},
		TestResults: "ok   github.com/example/project/auth 0.100s",
	}

	scorecard, review := engine.Evaluate(state, nil)
	if scorecard == nil || review == nil {
		t.Fatalf("expected scorecard and review")
	}
	if scorecard.Decision != models.ReviewAccept {
		t.Fatalf("expected accept decision, got %s", scorecard.Decision)
	}
	if review.Decision != models.ReviewAccept {
		t.Fatalf("expected accept review, got %s", review.Decision)
	}
}

func TestEvaluateRevisesWhenScopeFails(t *testing.T) {
	engine := NewEngine()
	state := &models.RunState{
		Task:      models.Task{ID: "task-2", Description: "feature task", CreatedAt: time.Now()},
		TaskBrief: &models.TaskBrief{TaskID: "task-2", TaskType: models.TaskTypeFeature, RiskLevel: models.RiskMedium},
		Plan: &models.Plan{
			TaskID:             "task-2",
			AcceptanceCriteria: []models.AcceptanceCriterion{{ID: "ac-1", Description: "Feature works."}},
			TestRequirements:   []string{"Run tests"},
		},
		Patch: &models.Patch{Files: []models.PatchFile{{Path: "internal/feature/service.go"}}},
		ValidationResults: []models.ValidationResult{
			{Name: "scope_compliance", Status: models.ValidationFail, Summary: "out of scope"},
			{Name: "plan_compliance", Status: models.ValidationFail, Summary: "missing required file"},
		},
		TestResults: "ok",
	}

	scorecard, review := engine.Evaluate(state, nil)
	if scorecard.Decision != models.ReviewRevise {
		t.Fatalf("expected revise scorecard decision, got %s", scorecard.Decision)
	}
	if review.Decision != models.ReviewRevise {
		t.Fatalf("expected revise review decision, got %s", review.Decision)
	}
	if len(scorecard.Findings) == 0 {
		t.Fatalf("expected findings for revise decision")
	}
}

func TestEvaluateRespectsProviderReviseSignal(t *testing.T) {
	engine := NewEngine()
	state := &models.RunState{
		Task:      models.Task{ID: "task-3", Description: "review provider", CreatedAt: time.Now()},
		TaskBrief: &models.TaskBrief{TaskID: "task-3", TaskType: models.TaskTypeFeature, RiskLevel: models.RiskLow},
		Plan: &models.Plan{
			TaskID:             "task-3",
			AcceptanceCriteria: []models.AcceptanceCriterion{{ID: "ac-1", Description: "Feature works."}},
			TestRequirements:   []string{"Run tests"},
		},
		ExecutionContract: &models.ExecutionContract{AllowedFiles: []string{"internal/feature/service.go"}},
		Patch:             &models.Patch{Files: []models.PatchFile{{Path: "internal/feature/service.go"}}},
		ValidationResults: []models.ValidationResult{
			{Name: "patch_hygiene", Status: models.ValidationPass},
			{Name: "scope_compliance", Status: models.ValidationPass},
			{Name: "plan_compliance", Status: models.ValidationPass},
		},
		TestResults: "ok",
	}

	providerReview := &models.ReviewResult{Decision: models.ReviewRevise, Comments: []string{"revise: missing edge case"}}
	scorecard, review := engine.Evaluate(state, providerReview)
	if scorecard.Decision != models.ReviewRevise {
		t.Fatalf("expected provider revise to force revise")
	}
	if review.Decision != models.ReviewRevise {
		t.Fatalf("expected final review revise")
	}
}
