package confidence

import (
	"testing"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
)

func TestPolicyPassesWhenConfidenceMeetsThreshold(t *testing.T) {
	policy := NewPolicy(config.DefaultConfig())
	state := &models.RunState{
		Review:          &models.ReviewResult{Decision: models.ReviewAccept},
		ReviewScorecard: &models.ReviewScorecard{Decision: models.ReviewAccept},
		Confidence:      &models.ConfidenceReport{Score: 0.82, Band: "medium"},
	}

	if err := policy.Apply(state); err != nil {
		t.Fatalf("expected policy pass, got error: %v", err)
	}
	if state.Review.Decision != models.ReviewAccept {
		t.Fatalf("expected accept decision to remain")
	}
	if !hasGate(state.ValidationResults, "review_decision_threshold_met", models.ValidationPass) {
		t.Fatalf("expected passing review threshold gate")
	}
}

func TestPolicyRevisesWhenConfidenceBelowCompletionThreshold(t *testing.T) {
	policy := NewPolicy(config.DefaultConfig())
	state := &models.RunState{
		Review:          &models.ReviewResult{Decision: models.ReviewAccept},
		ReviewScorecard: &models.ReviewScorecard{Decision: models.ReviewAccept},
		Confidence:      &models.ConfidenceReport{Score: 0.61, Band: "low"},
	}

	if err := policy.Apply(state); err != nil {
		t.Fatalf("expected revise path without hard error, got: %v", err)
	}
	if state.Review.Decision != models.ReviewRevise {
		t.Fatalf("expected review decision to downgrade to revise")
	}
	if !hasGate(state.ValidationResults, "review_decision_threshold_met", models.ValidationFail) {
		t.Fatalf("expected failing review threshold gate")
	}
}

func TestPolicyFailsWhenConfidenceBelowFailThreshold(t *testing.T) {
	policy := NewPolicy(config.DefaultConfig())
	state := &models.RunState{
		Review:          &models.ReviewResult{Decision: models.ReviewAccept},
		ReviewScorecard: &models.ReviewScorecard{Decision: models.ReviewAccept},
		Confidence:      &models.ConfidenceReport{Score: 0.30, Band: "very_low"},
	}

	if err := policy.Apply(state); err == nil {
		t.Fatalf("expected hard failure for very low confidence")
	}
	if state.Review.Decision != models.ReviewRevise {
		t.Fatalf("expected decision to downgrade to revise before failure")
	}
}

func hasGate(results []models.ValidationResult, name string, status models.ValidationStatus) bool {
	for _, result := range results {
		if result.Name == name && result.Status == status {
			return true
		}
	}
	return false
}
