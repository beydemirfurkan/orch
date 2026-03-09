package execution

import (
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
)

type RetryDirectiveBuilder struct{}

func NewRetryDirectiveBuilder() *RetryDirectiveBuilder {
	return &RetryDirectiveBuilder{}
}

func (b *RetryDirectiveBuilder) FromValidation(state *models.RunState, attempt int) *models.RetryDirective {
	if state == nil {
		return nil
	}
	directive := &models.RetryDirective{
		Stage:        "validation",
		Attempt:      attempt,
		Reasons:      []string{},
		FailedGates:  []string{},
		Instructions: []string{},
		Avoid: []string{
			"Do not retry with the same scope violation or forbidden change.",
			"Do not expand scope unless the change is explicitly justified.",
		},
	}
	for _, result := range state.ValidationResults {
		if result.Status != models.ValidationFail {
			continue
		}
		directive.FailedGates = append(directive.FailedGates, result.Name)
		directive.Reasons = append(directive.Reasons, result.Summary)
		directive.Instructions = append(directive.Instructions, result.ActionableItems...)
	}
	if len(directive.Instructions) == 0 {
		directive.Instructions = append(directive.Instructions, "Regenerate the patch to satisfy all failed validation gates.")
	}
	return normalizeDirective(directive)
}

func (b *RetryDirectiveBuilder) FromTest(state *models.RunState, attempt int) *models.RetryDirective {
	if state == nil {
		return nil
	}
	reasons := []string{"The previous patch failed test execution."}
	failedTests := []string{}
	instructions := []string{"Fix the failing test behavior while keeping the patch inside the approved scope."}
	for _, failure := range state.TestFailures {
		failedTests = append(failedTests, failure.Code)
		reasons = append(reasons, failure.Summary)
		switch failure.Code {
		case "test_timeout":
			instructions = append(instructions, "Reduce the cause of timeout or make the code path deterministic enough to complete within the test budget.")
		case "missing_required_tests":
			instructions = append(instructions, "Add or restore the required tests instead of bypassing verification.")
		case "test_assertion_failure":
			instructions = append(instructions, "Fix the functional behavior that caused the assertion failure.")
		case "flaky_test_suspected":
			instructions = append(instructions, "Stabilize the code path instead of weakening the test.")
		default:
			instructions = append(instructions, "Resolve the setup or runtime issue causing the test command failure.")
		}
	}
	if len(failedTests) == 0 {
		failedTests = splitNonEmptyLines(state.TestResults)
	}
	directive := &models.RetryDirective{
		Stage:        "test",
		Attempt:      attempt,
		Reasons:      reasons,
		FailedTests:  failedTests,
		Instructions: instructions,
		Avoid: []string{
			"Do not remove or weaken tests to make them pass.",
			"Do not introduce unrelated changes while fixing test failures.",
		},
	}
	for _, result := range state.ValidationResults {
		if result.Status == models.ValidationFail {
			directive.FailedGates = append(directive.FailedGates, result.Name)
		}
	}
	return normalizeDirective(directive)
}

func (b *RetryDirectiveBuilder) FromReview(state *models.RunState, attempt int) *models.RetryDirective {
	if state == nil || state.Review == nil {
		return nil
	}
	directive := &models.RetryDirective{
		Stage:        "review",
		Attempt:      attempt,
		Reasons:      append([]string{}, state.Review.Comments...),
		Instructions: append([]string{}, state.Review.Suggestions...),
		Avoid: []string{
			"Do not ignore reviewer findings.",
			"Do not broaden the patch beyond what is needed to resolve review findings.",
		},
	}
	if len(directive.Instructions) == 0 {
		directive.Instructions = append(directive.Instructions, "Address the review findings and preserve all prior passing validation and test conditions.")
	}
	return normalizeDirective(directive)
}

func normalizeDirective(d *models.RetryDirective) *models.RetryDirective {
	if d == nil {
		return nil
	}
	d.Reasons = uniqueNonEmpty(d.Reasons)
	d.FailedGates = uniqueNonEmpty(d.FailedGates)
	d.FailedTests = uniqueNonEmpty(d.FailedTests)
	d.Instructions = uniqueNonEmpty(d.Instructions)
	d.Avoid = uniqueNonEmpty(d.Avoid)
	return d
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
