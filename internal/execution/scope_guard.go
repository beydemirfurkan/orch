package execution

import (
	"fmt"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
)

type ScopeGuard struct{}

func NewScopeGuard() *ScopeGuard {
	return &ScopeGuard{}
}

func (g *ScopeGuard) Validate(contract *models.ExecutionContract, patch *models.Patch) models.ValidationResult {
	result := models.ValidationResult{
		Name:     "scope_compliance",
		Stage:    "validation",
		Status:   models.ValidationPass,
		Severity: models.SeverityLow,
		Summary:  "patch stayed inside allowed scope",
		Details:  []string{},
	}

	if contract == nil {
		result.Status = models.ValidationWarn
		result.Severity = models.SeverityMedium
		result.Summary = "execution contract missing; scope compliance could not be fully validated"
		return result
	}
	if patch == nil {
		result.Status = models.ValidationFail
		result.Severity = models.SeverityHigh
		result.Summary = "patch missing for scope validation"
		return result
	}
	if len(contract.AllowedFiles) == 0 || len(patch.Files) == 0 {
		return result
	}

	allowed := map[string]struct{}{}
	for _, file := range contract.AllowedFiles {
		allowed[strings.TrimSpace(file)] = struct{}{}
	}

	violations := make([]string, 0)
	for _, file := range patch.Files {
		path := strings.TrimSpace(file.Path)
		if path == "" {
			continue
		}
		if _, ok := allowed[path]; ok {
			continue
		}
		violations = append(violations, path)
	}

	if len(violations) == 0 {
		return result
	}

	result.Status = models.ValidationFail
	result.Severity = models.SeverityHigh
	result.Summary = fmt.Sprintf("patch changed files outside allowed scope: %s", strings.Join(violations, ", "))
	result.Details = violations
	result.ActionableItems = []string{"Regenerate the patch using only allowed files or expand scope explicitly with justification."}
	return result
}
