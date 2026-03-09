package execution

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
)

type PlanComplianceGuard struct{}

func NewPlanComplianceGuard() *PlanComplianceGuard {
	return &PlanComplianceGuard{}
}

func (g *PlanComplianceGuard) Validate(plan *models.Plan, contract *models.ExecutionContract, patch *models.Patch) models.ValidationResult {
	result := models.ValidationResult{
		Name:     "plan_compliance",
		Stage:    "validation",
		Status:   models.ValidationPass,
		Severity: models.SeverityLow,
		Summary:  "patch remains compliant with the structured plan",
		Metadata: map[string]string{},
	}

	if plan == nil {
		result.Status = models.ValidationWarn
		result.Severity = models.SeverityMedium
		result.Summary = "structured plan missing; plan compliance could not be fully validated"
		return result
	}
	if patch == nil {
		result.Status = models.ValidationFail
		result.Severity = models.SeverityHigh
		result.Summary = "patch missing for plan compliance validation"
		return result
	}
	if len(plan.AcceptanceCriteria) == 0 && (contract == nil || len(contract.AcceptanceCriteria) == 0) {
		result.Status = models.ValidationFail
		result.Severity = models.SeverityHigh
		result.Summary = "structured plan is missing acceptance criteria"
		result.ActionableItems = []string{"Regenerate the plan with explicit acceptance criteria before coding."}
		return result
	}
	if len(patch.Files) == 0 {
		result.Status = models.ValidationFail
		result.Severity = models.SeverityHigh
		result.Summary = "patch contains no changed files despite a code task plan"
		result.ActionableItems = []string{"Generate a non-empty patch that satisfies the required edits and acceptance criteria."}
		return result
	}

	changedFiles := changedFileSet(patch)
	requiredFiles := requiredModifyFiles(plan, contract)
	missingFiles := make([]string, 0)
	for _, file := range requiredFiles {
		if _, ok := changedFiles[file]; ok {
			continue
		}
		missingFiles = append(missingFiles, file)
	}
	if len(missingFiles) > 0 {
		result.Status = models.ValidationFail
		result.Severity = models.SeverityHigh
		result.Summary = fmt.Sprintf("patch did not modify required planned files: %s", strings.Join(missingFiles, ", "))
		result.Details = missingFiles
		result.ActionableItems = []string{"Ensure all required planned files are updated or explicitly reduce scope before retrying."}
		result.Metadata["required_files"] = strings.Join(requiredFiles, ",")
		return result
	}

	forbiddenViolations := detectForbiddenViolations(plan, patch)
	if len(forbiddenViolations) > 0 {
		result.Status = models.ValidationFail
		result.Severity = models.SeverityHigh
		result.Summary = fmt.Sprintf("patch appears to violate forbidden change rules: %s", strings.Join(forbiddenViolations, "; "))
		result.Details = forbiddenViolations
		result.ActionableItems = []string{"Remove forbidden changes and keep the patch aligned with the approved plan."}
		return result
	}

	result.Metadata["changed_files"] = strings.Join(sortedKeys(changedFiles), ",")
	result.Metadata["required_files"] = strings.Join(requiredFiles, ",")
	return result
}

func changedFileSet(patch *models.Patch) map[string]struct{} {
	result := map[string]struct{}{}
	if patch == nil {
		return result
	}
	for _, file := range patch.Files {
		path := strings.TrimSpace(file.Path)
		if path == "" {
			continue
		}
		result[path] = struct{}{}
	}
	return result
}

func requiredModifyFiles(plan *models.Plan, contract *models.ExecutionContract) []string {
	files := make([]string, 0)
	if plan != nil {
		files = append(files, plan.FilesToModify...)
	}
	if len(files) == 0 && contract != nil {
		files = append(files, contract.AllowedFiles...)
	}
	return uniqueNonEmpty(files)
}

func detectForbiddenViolations(plan *models.Plan, patch *models.Patch) []string {
	if plan == nil || patch == nil {
		return nil
	}
	violations := make([]string, 0)
	for _, rule := range plan.ForbiddenChanges {
		lowerRule := strings.ToLower(rule)
		for _, file := range patch.Files {
			path := strings.TrimSpace(file.Path)
			if path == "" {
				continue
			}
			if strings.Contains(lowerRule, "config") && isConfigLike(path) {
				violations = append(violations, fmt.Sprintf("forbidden config change touched %s", path))
			}
			if strings.Contains(lowerRule, "test") && isTestLike(path) {
				violations = append(violations, fmt.Sprintf("forbidden test change touched %s", path))
			}
		}
	}
	return uniqueNonEmpty(violations)
}

func isConfigLike(path string) bool {
	lower := strings.ToLower(path)
	base := strings.ToLower(filepath.Base(path))
	return strings.Contains(lower, "/config/") || strings.Contains(lower, "\\config\\") || strings.Contains(base, "config") || strings.Contains(base, ".env") || base == "package.json" || base == "go.mod" || base == "dockerfile" || strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".toml") || strings.HasSuffix(base, ".ini")
}

func isTestLike(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, "_test.") || strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.") || strings.Contains(lower, "/test") || strings.Contains(lower, "\\test")
}

func sortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j] < result[i] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}
