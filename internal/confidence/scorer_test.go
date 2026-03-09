package confidence

import (
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/models"
)

func TestScoreHighConfidenceRun(t *testing.T) {
	scorer := New()
	state := &models.RunState{
		Task: models.Task{ID: "task-1", Description: "fix auth bug", CreatedAt: time.Now()},
		Plan: &models.Plan{
			Summary:            "Fix auth bug",
			FilesToInspect:     []string{"auth.go"},
			FilesToModify:      []string{"auth.go"},
			AcceptanceCriteria: []models.AcceptanceCriterion{{ID: "ac-1", Description: "Bug fixed"}},
			TestRequirements:   []string{"Run go test ./..."},
		},
		ValidationResults: []models.ValidationResult{
			{Name: "scope_compliance", Status: models.ValidationPass, Severity: models.SeverityLow},
			{Name: "patch_hygiene", Status: models.ValidationPass, Severity: models.SeverityLow},
			{Name: "plan_compliance", Status: models.ValidationPass, Severity: models.SeverityLow},
		},
		ReviewScorecard: &models.ReviewScorecard{
			RequirementCoverage: 9,
			ScopeControl:        9,
			RegressionRisk:      8,
			Readability:         8,
			Maintainability:     8,
			TestAdequacy:        9,
			Decision:            models.ReviewAccept,
		},
		TestResults: "ok   auth",
	}

	report := scorer.Score(state)
	if report == nil {
		t.Fatalf("expected confidence report")
	}
	if report.Band != "high" && report.Band != "medium" {
		t.Fatalf("expected healthy confidence band, got %s", report.Band)
	}
	if report.Score <= 0.69 {
		t.Fatalf("expected higher score, got %f", report.Score)
	}
}

func TestScoreLowConfidenceRun(t *testing.T) {
	scorer := New()
	state := &models.RunState{
		Task: models.Task{ID: "task-2", Description: "feature", CreatedAt: time.Now()},
		Plan: &models.Plan{},
		ValidationResults: []models.ValidationResult{
			{Name: "scope_compliance", Status: models.ValidationFail, Severity: models.SeverityHigh, Summary: "scope fail"},
		},
		ReviewScorecard: &models.ReviewScorecard{
			RequirementCoverage: 4,
			ScopeControl:        3,
			RegressionRisk:      4,
			Readability:         5,
			Maintainability:     5,
			TestAdequacy:        2,
			Decision:            models.ReviewRevise,
		},
		Retries: models.RetryState{Validation: 2, Testing: 1},
	}

	report := scorer.Score(state)
	if report == nil {
		t.Fatalf("expected confidence report")
	}
	if report.Band != "low" && report.Band != "very_low" {
		t.Fatalf("expected low confidence band, got %s", report.Band)
	}
	if report.Score >= 0.70 {
		t.Fatalf("expected low score, got %f", report.Score)
	}
	if len(report.Warnings) == 0 {
		t.Fatalf("expected warnings for low confidence")
	}
}
