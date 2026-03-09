package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRunStateStructuredArtifactsJSONRoundtrip(t *testing.T) {
	now := time.Now().UTC()
	state := RunState{
		ID:   "run-1",
		Task: Task{ID: "task-1", Description: "fix auth bug", CreatedAt: now},
		TaskBrief: &TaskBrief{
			TaskID:         "task-1",
			UserRequest:    "fix auth bug",
			NormalizedGoal: "Fix auth bug while preserving existing behavior.",
			TaskType:       TaskTypeBugfix,
			RiskLevel:      RiskHigh,
		},
		Plan: &Plan{
			TaskID:             "task-1",
			Summary:            "Fix auth bug while preserving existing behavior.",
			TaskType:           TaskTypeBugfix,
			RiskLevel:          RiskHigh,
			AcceptanceCriteria: []AcceptanceCriterion{{ID: "ac-1", Description: "Bug path no longer fails."}},
		},
		ExecutionContract: &ExecutionContract{
			TaskID:       "task-1",
			AllowedFiles: []string{"internal/auth/service.go"},
			PatchBudget:  PatchBudget{MaxFiles: 2, MaxChangedLines: 80},
		},
		ValidationResults: []ValidationResult{{
			Name:     "scope_compliance",
			Stage:    "validation",
			Status:   ValidationPass,
			Severity: SeverityLow,
			Summary:  "patch stayed inside allowed scope",
		}},
		RetryDirective: &RetryDirective{
			Stage:        "validation",
			Attempt:      1,
			FailedGates:  []string{"scope_compliance"},
			Instructions: []string{"Keep the patch inside allowed scope."},
		},
		ReviewScorecard: &ReviewScorecard{
			RequirementCoverage: 9,
			ScopeControl:        10,
			RegressionRisk:      8,
			Readability:         8,
			Maintainability:     8,
			TestAdequacy:        8,
			Decision:            ReviewAccept,
		},
		Confidence: &ConfidenceReport{
			Score:   0.88,
			Band:    "high",
			Reasons: []string{"all mandatory signals aligned"},
		},
		TestFailures: []TestFailure{{
			Code:    "test_assertion_failure",
			Summary: "expected 200 got 500",
			Details: []string{"expected 200 got 500"},
		}},
		Status:    StatusPlanning,
		StartedAt: now,
	}

	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded RunState
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.TaskBrief == nil || decoded.TaskBrief.TaskType != TaskTypeBugfix {
		t.Fatalf("task brief did not roundtrip")
	}
	if decoded.ExecutionContract == nil || len(decoded.ExecutionContract.AllowedFiles) != 1 {
		t.Fatalf("execution contract did not roundtrip")
	}
	if len(decoded.ValidationResults) != 1 || decoded.ValidationResults[0].Name != "scope_compliance" {
		t.Fatalf("validation results did not roundtrip")
	}
	if decoded.RetryDirective == nil || decoded.RetryDirective.Stage != "validation" {
		t.Fatalf("retry directive did not roundtrip")
	}
	if decoded.ReviewScorecard == nil || decoded.ReviewScorecard.Decision != ReviewAccept {
		t.Fatalf("review scorecard did not roundtrip")
	}
	if decoded.Confidence == nil || decoded.Confidence.Band != "high" {
		t.Fatalf("confidence report did not roundtrip")
	}
	if len(decoded.TestFailures) != 1 || decoded.TestFailures[0].Code != "test_assertion_failure" {
		t.Fatalf("test failures did not roundtrip")
	}
}
