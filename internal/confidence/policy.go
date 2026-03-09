package confidence

import (
	"fmt"
	"strings"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
)

type Policy struct {
	enabled     bool
	completeMin float64
	failBelow   float64
}

func NewPolicy(cfg *config.Config) *Policy {
	defaults := config.DefaultConfig()
	policy := &Policy{
		enabled:     defaults.Safety.FeatureFlags.ConfidenceEnforcement,
		completeMin: defaults.Safety.Confidence.CompleteMin,
		failBelow:   defaults.Safety.Confidence.FailBelow,
	}
	if cfg == nil {
		return policy
	}
	policy.enabled = cfg.Safety.FeatureFlags.ConfidenceEnforcement
	if cfg.Safety.Confidence.CompleteMin > 0 {
		policy.completeMin = cfg.Safety.Confidence.CompleteMin
	}
	if cfg.Safety.Confidence.FailBelow > 0 {
		policy.failBelow = cfg.Safety.Confidence.FailBelow
	}
	return policy
}

func (p *Policy) Apply(state *models.RunState) error {
	if state == nil {
		return nil
	}

	appendOrReplaceReviewGate(state, models.ValidationResult{
		Name:     "review_scorecard_valid",
		Stage:    "review",
		Status:   models.ValidationPass,
		Severity: models.SeverityLow,
		Summary:  "review scorecard produced successfully",
	})

	if state.ReviewScorecard == nil {
		appendOrReplaceReviewGate(state, models.ValidationResult{
			Name:     "review_scorecard_valid",
			Stage:    "review",
			Status:   models.ValidationFail,
			Severity: models.SeverityHigh,
			Summary:  "review scorecard is missing",
		})
		return fmt.Errorf("review scorecard is missing")
	}
	if state.Review == nil {
		appendOrReplaceReviewGate(state, models.ValidationResult{
			Name:     "review_decision_threshold_met",
			Stage:    "review",
			Status:   models.ValidationFail,
			Severity: models.SeverityHigh,
			Summary:  "review result is missing",
		})
		return fmt.Errorf("review result is missing")
	}
	if state.Confidence == nil {
		appendOrReplaceReviewGate(state, models.ValidationResult{
			Name:     "review_decision_threshold_met",
			Stage:    "review",
			Status:   models.ValidationFail,
			Severity: models.SeverityHigh,
			Summary:  "confidence report is missing",
		})
		return fmt.Errorf("confidence report is missing")
	}

	if !p.enabled {
		appendOrReplaceReviewGate(state, models.ValidationResult{
			Name:     "review_decision_threshold_met",
			Stage:    "review",
			Status:   models.ValidationPass,
			Severity: models.SeverityLow,
			Summary:  "confidence enforcement disabled; review threshold considered satisfied",
		})
		return nil
	}

	score := state.Confidence.Score
	if score < p.failBelow {
		message := fmt.Sprintf("confidence %.2f is below fail threshold %.2f", score, p.failBelow)
		markReviewRevise(state, message)
		appendOrReplaceReviewGate(state, models.ValidationResult{
			Name:            "review_decision_threshold_met",
			Stage:           "review",
			Status:          models.ValidationFail,
			Severity:        models.SeverityHigh,
			Summary:         message,
			ActionableItems: []string{"Do not complete the run; inspect the confidence warnings and regenerate the patch with tighter scope and stronger verification."},
		})
		return fmt.Errorf("%s", message)
	}

	if score < p.completeMin {
		message := fmt.Sprintf("confidence %.2f is below completion threshold %.2f", score, p.completeMin)
		markReviewRevise(state, message)
		appendOrReplaceReviewGate(state, models.ValidationResult{
			Name:            "review_decision_threshold_met",
			Stage:           "review",
			Status:          models.ValidationFail,
			Severity:        models.SeverityMedium,
			Summary:         message,
			ActionableItems: []string{"Retry the patch until confidence reaches the completion threshold or reduce uncertainty in validation/test signals."},
		})
		return nil
	}

	appendOrReplaceReviewGate(state, models.ValidationResult{
		Name:     "review_decision_threshold_met",
		Stage:    "review",
		Status:   models.ValidationPass,
		Severity: models.SeverityLow,
		Summary:  fmt.Sprintf("confidence %.2f meets completion threshold %.2f", score, p.completeMin),
	})
	return nil
}

func appendOrReplaceReviewGate(state *models.RunState, gate models.ValidationResult) {
	if state == nil {
		return
	}
	for i, result := range state.ValidationResults {
		if result.Name == gate.Name && strings.EqualFold(result.Stage, gate.Stage) {
			state.ValidationResults[i] = gate
			return
		}
	}
	state.ValidationResults = append(state.ValidationResults, gate)
}

func markReviewRevise(state *models.RunState, message string) {
	if state == nil {
		return
	}
	if state.Review != nil {
		state.Review.Decision = models.ReviewRevise
		state.Review.Comments = uniqueNonEmptyPolicy(append(state.Review.Comments, message))
		state.Review.Suggestions = uniqueNonEmptyPolicy(append(state.Review.Suggestions, "Increase confidence by improving validation, tests, or review findings before completion."))
	}
	if state.ReviewScorecard != nil {
		state.ReviewScorecard.Decision = models.ReviewRevise
		state.ReviewScorecard.Findings = uniqueNonEmptyPolicy(append(state.ReviewScorecard.Findings, message))
	}
}

func uniqueNonEmptyPolicy(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
