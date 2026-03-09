package review

import (
	"fmt"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
)

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Evaluate(state *models.RunState, providerReview *models.ReviewResult) (*models.ReviewScorecard, *models.ReviewResult) {
	if state == nil {
		return nil, nil
	}

	requirementCoverage, requirementFindings := scoreRequirementCoverage(state)
	scopeControl, scopeFindings := scoreScopeControl(state)
	regressionRisk, regressionFindings := scoreRegressionRisk(state)
	readability, readabilityFindings := scoreReadability(state)
	maintainability, maintainabilityFindings := scoreMaintainability(state)
	testAdequacy, testFindings := scoreTestAdequacy(state)

	findings := make([]string, 0)
	findings = append(findings, requirementFindings...)
	findings = append(findings, scopeFindings...)
	findings = append(findings, regressionFindings...)
	findings = append(findings, readabilityFindings...)
	findings = append(findings, maintainabilityFindings...)
	findings = append(findings, testFindings...)

	decision := models.ReviewAccept
	average := float64(requirementCoverage+scopeControl+regressionRisk+readability+maintainability+testAdequacy) / 6.0
	if requirementCoverage < 7 || scopeControl < 7 || testAdequacy < 7 || average < 7.5 {
		decision = models.ReviewRevise
	}
	if hasFailedValidation(state.ValidationResults) {
		decision = models.ReviewRevise
	}
	if providerReview != nil && providerReview.Decision == models.ReviewRevise {
		decision = models.ReviewRevise
		findings = append(findings, providerReview.Comments...)
	}

	scorecard := &models.ReviewScorecard{
		RequirementCoverage: requirementCoverage,
		ScopeControl:        scopeControl,
		RegressionRisk:      regressionRisk,
		Readability:         readability,
		Maintainability:     maintainability,
		TestAdequacy:        testAdequacy,
		Decision:            decision,
		Findings:            uniqueNonEmpty(findings),
	}

	finalReview := &models.ReviewResult{
		Decision:    decision,
		Comments:    buildReviewComments(scorecard, providerReview, average),
		Suggestions: buildReviewSuggestions(scorecard),
	}
	return scorecard, finalReview
}

func scoreRequirementCoverage(state *models.RunState) (int, []string) {
	score := 5
	findings := []string{}
	if state.Plan == nil || len(state.Plan.AcceptanceCriteria) == 0 {
		return 2, []string{"Structured plan acceptance criteria are missing or incomplete."}
	}
	score += 2
	if state.Patch != nil && len(state.Patch.Files) > 0 {
		score += 1
	} else {
		findings = append(findings, "Patch does not contain concrete file changes for the planned task.")
	}
	if validationPassed(state.ValidationResults, "plan_compliance") {
		score += 2
	} else {
		findings = append(findings, "Patch did not clearly satisfy plan compliance expectations.")
	}
	return clampScore(score), findings
}

func scoreScopeControl(state *models.RunState) (int, []string) {
	score := 5
	findings := []string{}
	if validationPassed(state.ValidationResults, "scope_compliance") {
		score += 3
	} else {
		score = 2
		findings = append(findings, "Scope compliance gate did not pass cleanly.")
	}
	if validationPassed(state.ValidationResults, "patch_hygiene") {
		score += 2
	} else {
		findings = append(findings, "Patch hygiene gate indicates the diff may be too risky or malformed.")
	}
	return clampScore(score), findings
}

func scoreRegressionRisk(state *models.RunState) (int, []string) {
	score := 7
	findings := []string{}
	if state.TaskBrief != nil && state.TaskBrief.RiskLevel == models.RiskHigh {
		score--
		findings = append(findings, "Task is classified as high-risk and needs extra caution.")
	}
	if strings.TrimSpace(state.TestResults) == "" {
		score -= 2
		findings = append(findings, "Test output is empty, which weakens regression confidence.")
	}
	if hasFailedValidation(state.ValidationResults) {
		score -= 3
		findings = append(findings, "One or more validation gates failed earlier in the pipeline.")
	}
	if state.Retries.Testing > 0 || state.Retries.Validation > 0 {
		score--
		findings = append(findings, "Retry activity indicates prior instability before review.")
	}
	return clampScore(score), findings
}

func scoreReadability(state *models.RunState) (int, []string) {
	score := 8
	findings := []string{}
	if state.Patch == nil {
		return 3, []string{"No patch is available to assess readability."}
	}
	if len(state.Patch.Files) > 4 {
		score -= 2
		findings = append(findings, "Patch touches many files, making review and readability harder.")
	}
	lineCount := diffLineCount(state.Patch)
	if lineCount > 300 {
		score -= 3
		findings = append(findings, "Patch is large enough to reduce readability confidence.")
	} else if lineCount > 120 {
		score -= 1
	}
	return clampScore(score), findings
}

func scoreMaintainability(state *models.RunState) (int, []string) {
	score := 8
	findings := []string{}
	if state.Plan == nil {
		score = 4
		findings = append(findings, "Structured plan is missing, so maintainability alignment is unclear.")
	}
	if state.ExecutionContract == nil {
		score -= 2
		findings = append(findings, "Execution contract is missing, reducing maintainability guarantees.")
	}
	if len(state.UnresolvedFailures) > 0 {
		score -= 2
		findings = append(findings, "There are unresolved failures recorded in the run state.")
	}
	return clampScore(score), findings
}

func scoreTestAdequacy(state *models.RunState) (int, []string) {
	score := 4
	findings := []string{}
	if state.Plan == nil || len(state.Plan.TestRequirements) == 0 {
		findings = append(findings, "Plan does not define explicit test requirements.")
	}
	if strings.TrimSpace(state.TestResults) != "" {
		score = 8
	} else {
		findings = append(findings, "No concrete test output was recorded for the review step.")
	}
	if state.Retries.Testing > 0 {
		score--
		findings = append(findings, "Tests required retries before review acceptance could be considered.")
	}
	return clampScore(score), findings
}

func buildReviewComments(scorecard *models.ReviewScorecard, providerReview *models.ReviewResult, average float64) []string {
	comments := []string{
		fmt.Sprintf("Review scorecard: requirement=%d scope=%d regression=%d readability=%d maintainability=%d test=%d avg=%.1f", scorecard.RequirementCoverage, scorecard.ScopeControl, scorecard.RegressionRisk, scorecard.Readability, scorecard.Maintainability, scorecard.TestAdequacy, average),
	}
	if providerReview != nil {
		comments = append(comments, providerReview.Comments...)
	}
	comments = append(comments, scorecard.Findings...)
	return uniqueNonEmpty(comments)
}

func buildReviewSuggestions(scorecard *models.ReviewScorecard) []string {
	if scorecard == nil || scorecard.Decision != models.ReviewRevise {
		return []string{}
	}
	suggestions := []string{}
	for _, finding := range scorecard.Findings {
		suggestions = append(suggestions, "Address review finding: "+finding)
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "Improve the patch so that all review rubric categories meet the acceptance threshold.")
	}
	return uniqueNonEmpty(suggestions)
}

func validationPassed(results []models.ValidationResult, name string) bool {
	for _, result := range results {
		if result.Name == name {
			return result.Status == models.ValidationPass
		}
	}
	return false
}

func hasFailedValidation(results []models.ValidationResult) bool {
	for _, result := range results {
		if result.Status == models.ValidationFail {
			return true
		}
	}
	return false
}

func diffLineCount(patch *models.Patch) int {
	if patch == nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(patch.RawDiff, "\n") {
		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
			if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
				continue
			}
			count++
		}
	}
	return count
}

func clampScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 10 {
		return 10
	}
	return score
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
