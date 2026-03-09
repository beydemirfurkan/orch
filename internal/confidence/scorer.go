package confidence

import (
	"fmt"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
)

type Scorer struct{}

func New() *Scorer {
	return &Scorer{}
}

func (s *Scorer) Score(state *models.RunState) *models.ConfidenceReport {
	if state == nil {
		return nil
	}

	reasons := []string{}
	warnings := []string{}

	planScore := s.planCompleteness(state, &reasons, &warnings)
	scopeScore := s.scopeCompliance(state, &reasons, &warnings)
	validationScore := s.validationQuality(state, &reasons, &warnings)
	testScore := s.testQuality(state, &reasons, &warnings)
	reviewScore := s.reviewQuality(state, &reasons, &warnings)
	retryPenalty := s.retryPenalty(state, &warnings)

	score := planScore*0.10 + scopeScore*0.20 + validationScore*0.20 + testScore*0.25 + reviewScore*0.20 - retryPenalty*0.05
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	band := confidenceBand(score)
	if band == "high" {
		reasons = append(reasons, "confidence is high because plan, validation, tests, and review signals are aligned")
	}
	if band == "low" || band == "very_low" {
		warnings = append(warnings, "confidence is below the preferred threshold for hands-off trust")
	}

	return &models.ConfidenceReport{
		Score:    round2(score),
		Band:     band,
		Reasons:  uniqueNonEmpty(reasons),
		Warnings: uniqueNonEmpty(warnings),
	}
}

func (s *Scorer) planCompleteness(state *models.RunState, reasons, warnings *[]string) float64 {
	if state.Plan == nil {
		*warnings = append(*warnings, "structured plan is missing")
		return 0.2
	}
	score := 0.3
	if strings.TrimSpace(state.Plan.Summary) != "" {
		score += 0.2
	}
	if len(state.Plan.FilesToInspect) > 0 {
		score += 0.15
	}
	if len(state.Plan.FilesToModify) > 0 {
		score += 0.15
	}
	if len(state.Plan.AcceptanceCriteria) > 0 {
		score += 0.1
		*reasons = append(*reasons, "structured plan includes explicit acceptance criteria")
	} else {
		*warnings = append(*warnings, "structured plan acceptance criteria are missing")
	}
	if len(state.Plan.TestRequirements) > 0 || strings.TrimSpace(state.Plan.TestStrategy) != "" {
		score += 0.1
	}
	return clamp01(score)
}

func (s *Scorer) scopeCompliance(state *models.RunState, reasons, warnings *[]string) float64 {
	for _, result := range state.ValidationResults {
		if result.Name != "scope_compliance" {
			continue
		}
		switch result.Status {
		case models.ValidationPass:
			*reasons = append(*reasons, "scope compliance gate passed")
			return 1.0
		case models.ValidationWarn:
			*warnings = append(*warnings, result.Summary)
			return 0.6
		default:
			*warnings = append(*warnings, result.Summary)
			return 0.1
		}
	}
	*warnings = append(*warnings, "scope compliance gate result is missing")
	return 0.4
}

func (s *Scorer) validationQuality(state *models.RunState, reasons, warnings *[]string) float64 {
	if len(state.ValidationResults) == 0 {
		*warnings = append(*warnings, "validation results are missing")
		return 0.3
	}
	passWeight := 0.0
	totalWeight := 0.0
	for _, result := range state.ValidationResults {
		weight := validationWeight(result.Severity)
		totalWeight += weight
		switch result.Status {
		case models.ValidationPass:
			passWeight += weight
		case models.ValidationWarn:
			passWeight += weight * 0.5
			*warnings = append(*warnings, result.Summary)
		default:
			*warnings = append(*warnings, result.Summary)
		}
	}
	if totalWeight == 0 {
		return 0.3
	}
	ratio := passWeight / totalWeight
	if ratio >= 0.9 {
		*reasons = append(*reasons, "validation gates passed with high coverage")
	}
	return clamp01(ratio)
}

func (s *Scorer) testQuality(state *models.RunState, reasons, warnings *[]string) float64 {
	if strings.TrimSpace(state.TestResults) == "" {
		*warnings = append(*warnings, "test output is missing")
		return 0.25
	}
	score := 0.8
	lower := strings.ToLower(state.TestResults)
	if len(state.TestFailures) > 0 {
		score = 0.2
		for _, failure := range state.TestFailures {
			*warnings = append(*warnings, failure.Code+": "+failure.Summary)
			if failure.Flaky {
				score = 0.35
			}
		}
	} else if strings.Contains(lower, "fail") || strings.Contains(lower, "panic") {
		*warnings = append(*warnings, "test output indicates instability or failure history")
		score = 0.2
	} else {
		*reasons = append(*reasons, "test output was recorded for the run")
	}
	if state.Retries.Testing > 0 {
		score -= 0.2
		*warnings = append(*warnings, fmt.Sprintf("tests required %d retry attempt(s)", state.Retries.Testing))
	}
	return clamp01(score)
}

func (s *Scorer) reviewQuality(state *models.RunState, reasons, warnings *[]string) float64 {
	if state.ReviewScorecard == nil {
		*warnings = append(*warnings, "review scorecard is missing")
		return 0.3
	}
	avg := float64(
		state.ReviewScorecard.RequirementCoverage+
			state.ReviewScorecard.ScopeControl+
			state.ReviewScorecard.RegressionRisk+
			state.ReviewScorecard.Readability+
			state.ReviewScorecard.Maintainability+
			state.ReviewScorecard.TestAdequacy,
	) / 60.0
	if state.ReviewScorecard.Decision == models.ReviewAccept {
		*reasons = append(*reasons, "review rubric accepted the patch")
	} else {
		*warnings = append(*warnings, "review rubric did not fully accept the patch")
		avg *= 0.7
	}
	return clamp01(avg)
}

func (s *Scorer) retryPenalty(state *models.RunState, warnings *[]string) float64 {
	total := state.Retries.Validation + state.Retries.Testing + state.Retries.Review
	if total == 0 {
		return 0
	}
	*warnings = append(*warnings, fmt.Sprintf("run used %d total retry attempt(s)", total))
	if total >= 3 {
		return 1
	}
	if total == 2 {
		return 0.6
	}
	return 0.3
}

func validationWeight(severity models.ValidationSeverity) float64 {
	switch severity {
	case models.SeverityCritical:
		return 1.5
	case models.SeverityHigh:
		return 1.2
	case models.SeverityMedium:
		return 1.0
	default:
		return 0.8
	}
}

func confidenceBand(score float64) string {
	switch {
	case score >= 0.85:
		return "high"
	case score >= 0.70:
		return "medium"
	case score >= 0.50:
		return "low"
	default:
		return "very_low"
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func uniqueNonEmpty(values []string) []string {
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
