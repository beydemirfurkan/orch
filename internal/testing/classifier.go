package testingx

import (
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
)

type Classifier struct{}

func NewClassifier() *Classifier {
	return &Classifier{}
}

func (c *Classifier) Classify(output, errText string) []models.TestFailure {
	combined := strings.TrimSpace(strings.Join([]string{strings.TrimSpace(output), strings.TrimSpace(errText)}, "\n"))
	if combined == "" {
		return []models.TestFailure{{
			Code:    "test_setup_failure",
			Summary: "test command failed without output",
			Details: []string{"No test output was captured from the failed command."},
		}}
	}

	lines := splitNonEmptyLines(combined)
	lower := strings.ToLower(combined)

	switch {
	case strings.Contains(lower, "timed out") || strings.Contains(lower, "timeout"):
		return []models.TestFailure{{
			Code:    "test_timeout",
			Summary: "test command timed out",
			Details: lines,
		}}
	case strings.Contains(lower, "panic:") || strings.Contains(lower, "segmentation fault"):
		return []models.TestFailure{{
			Code:    "test_setup_failure",
			Summary: "test runtime crashed or panicked",
			Details: lines,
		}}
	case strings.Contains(lower, "no test files"):
		return []models.TestFailure{{
			Code:    "missing_required_tests",
			Summary: "required tests appear to be missing",
			Details: lines,
		}}
	case strings.Contains(lower, "assert") || strings.Contains(lower, "expected") || strings.Contains(lower, "--- fail") || strings.Contains(lower, "not equal"):
		return []models.TestFailure{{
			Code:    "test_assertion_failure",
			Summary: "test assertions failed",
			Details: lines,
		}}
	case strings.Contains(lower, "flake") || strings.Contains(lower, "flaky"):
		return []models.TestFailure{{
			Code:    "flaky_test_suspected",
			Summary: "test output suggests flaky behavior",
			Details: lines,
			Flaky:   true,
		}}
	default:
		return []models.TestFailure{{
			Code:    "test_setup_failure",
			Summary: "test command failed",
			Details: lines,
		}}
	}
}

func splitNonEmptyLines(text string) []string {
	parts := strings.Split(strings.TrimSpace(text), "\n")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}
