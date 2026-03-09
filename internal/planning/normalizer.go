package planning

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
)

var stopWords = map[string]struct{}{
	"add": {}, "the": {}, "and": {}, "for": {}, "with": {}, "into": {}, "from": {},
	"this": {}, "that": {}, "fix": {}, "make": {}, "code": {}, "write": {},
	"update": {}, "service": {}, "feature": {}, "bug": {}, "test": {}, "tests": {},
}

type candidate struct {
	path  string
	score int
}

func NormalizeTask(task *models.Task) *models.TaskBrief {
	if task == nil {
		return nil
	}

	desc := strings.TrimSpace(task.Description)
	lower := strings.ToLower(desc)
	taskType := classifyTaskType(lower)
	risk := classifyRisk(lower, taskType)

	brief := &models.TaskBrief{
		TaskID:            task.ID,
		UserRequest:       desc,
		NormalizedGoal:    normalizeGoal(desc, taskType),
		TaskType:          taskType,
		RiskLevel:         risk,
		Constraints:       []string{},
		Assumptions:       deriveAssumptions(taskType, risk),
		SuccessDefinition: successDefinitionFor(taskType),
	}

	return brief
}

func CompilePlan(task *models.Task, brief *models.TaskBrief, repoMap *models.RepoMap) *models.Plan {
	if task == nil {
		return nil
	}
	if brief == nil {
		brief = NormalizeTask(task)
	}

	inspect, modify := rankFiles(task.Description, brief.TaskType, repoMap)
	criteria := acceptanceCriteriaFor(brief)
	testRequirements := testRequirementsFor(brief, repoMap)
	plan := &models.Plan{
		TaskID:             task.ID,
		Summary:            brief.NormalizedGoal,
		TaskType:           brief.TaskType,
		RiskLevel:          brief.RiskLevel,
		Steps:              buildSteps(brief, inspect, modify),
		FilesToModify:      modify,
		FilesToInspect:     inspect,
		Risks:              risksFor(brief),
		TestStrategy:       strings.Join(testRequirements, "; "),
		TestRequirements:   testRequirements,
		AcceptanceCriteria: criteria,
		Invariants:         invariantsFor(brief),
		ForbiddenChanges:   forbiddenChangesFor(brief),
	}
	return plan
}

func classifyTaskType(lower string) models.TaskType {
	switch {
	case containsAny(lower, "race", "bug", "fix", "issue", "error", "regression"):
		return models.TaskTypeBugfix
	case containsAny(lower, "unit test", "integration test", "test ", "tests ", "coverage"):
		return models.TaskTypeTest
	case containsAny(lower, "refactor", "cleanup", "readability", "simplify"):
		return models.TaskTypeRefactor
	case containsAny(lower, "docs", "readme", "documentation"):
		return models.TaskTypeDocs
	case containsAny(lower, "add", "implement", "create", "support", "enable"):
		return models.TaskTypeFeature
	case containsAny(lower, "bump", "upgrade", "rename", "move", "remove", "chore"):
		return models.TaskTypeChore
	default:
		return models.TaskTypeUnknown
	}
}

func classifyRisk(lower string, taskType models.TaskType) models.RiskLevel {
	if containsAny(lower, "race", "concurrency", "auth", "security", "payment", "migration", "schema", "database") {
		return models.RiskHigh
	}
	switch taskType {
	case models.TaskTypeDocs, models.TaskTypeTest:
		return models.RiskLow
	case models.TaskTypeRefactor, models.TaskTypeFeature, models.TaskTypeBugfix:
		return models.RiskMedium
	default:
		return models.RiskMedium
	}
}

func normalizeGoal(desc string, taskType models.TaskType) string {
	trimmed := strings.TrimSpace(desc)
	switch taskType {
	case models.TaskTypeBugfix:
		return fmt.Sprintf("Fix %s while preserving existing behavior.", trimmed)
	case models.TaskTypeTest:
		return fmt.Sprintf("Add or update tests for %s.", trimmed)
	case models.TaskTypeRefactor:
		return fmt.Sprintf("Refactor %s with minimal behavior change.", trimmed)
	case models.TaskTypeDocs:
		return fmt.Sprintf("Update documentation for %s.", trimmed)
	case models.TaskTypeFeature:
		return fmt.Sprintf("Implement %s following existing repository patterns.", trimmed)
	default:
		return fmt.Sprintf("Address task: %s", trimmed)
	}
}

func deriveAssumptions(taskType models.TaskType, risk models.RiskLevel) []string {
	assumptions := []string{"Follow existing repository conventions and keep the diff minimal."}
	if risk == models.RiskHigh {
		assumptions = append(assumptions, "Prefer behavior-preserving changes and protect public interfaces.")
	}
	if taskType == models.TaskTypeTest {
		assumptions = append(assumptions, "Prefer colocated or adjacent test files when possible.")
	}
	return assumptions
}

func successDefinitionFor(taskType models.TaskType) []string {
	switch taskType {
	case models.TaskTypeBugfix:
		return []string{"The reported failure path is addressed.", "Existing behavior outside the bug scope remains unchanged."}
	case models.TaskTypeFeature:
		return []string{"The requested behavior is implemented.", "Relevant tests or verification steps exist."}
	case models.TaskTypeTest:
		return []string{"Relevant tests are added or updated.", "Tests validate the intended behavior."}
	case models.TaskTypeRefactor:
		return []string{"Code structure improves without changing intended behavior.", "Existing tests still pass."}
	case models.TaskTypeDocs:
		return []string{"Documentation reflects the requested behavior accurately."}
	default:
		return []string{"The task is completed with minimal scope change."}
	}
}

func acceptanceCriteriaFor(brief *models.TaskBrief) []models.AcceptanceCriterion {
	if brief == nil {
		return nil
	}

	descriptions := []string{}
	switch brief.TaskType {
	case models.TaskTypeBugfix:
		descriptions = []string{
			"The original failure condition is no longer reproducible in the intended path.",
			"Existing behavior outside the bug scope remains unchanged.",
			"Relevant regression verification is included or documented.",
		}
	case models.TaskTypeFeature:
		descriptions = []string{
			"The requested behavior is implemented and reachable through the intended code path.",
			"Changes follow existing repository patterns and remain scoped to the task.",
			"Relevant verification or tests are included.",
		}
	case models.TaskTypeTest:
		descriptions = []string{
			"Relevant tests are added or updated.",
			"Tests cover the requested behavior or failure mode.",
			"Production code changes remain minimal unless required for testability.",
		}
	case models.TaskTypeRefactor:
		descriptions = []string{
			"Behavior remains unchanged for supported paths.",
			"The resulting code is easier to follow or maintain.",
			"Existing validation and tests still pass.",
		}
	case models.TaskTypeDocs:
		descriptions = []string{
			"Documentation accurately reflects the requested behavior or workflow.",
			"Examples or instructions remain consistent with the repository.",
		}
	default:
		descriptions = []string{
			"The task goal is addressed with a minimal scoped change.",
			"Relevant verification steps are documented or executed.",
		}
	}

	criteria := make([]models.AcceptanceCriterion, 0, len(descriptions))
	for i, description := range descriptions {
		criteria = append(criteria, models.AcceptanceCriterion{
			ID:          fmt.Sprintf("ac-%d", i+1),
			Description: description,
		})
	}
	return criteria
}

func testRequirementsFor(brief *models.TaskBrief, repoMap *models.RepoMap) []string {
	requirements := []string{"Run the configured test command and confirm no new failures."}
	framework := ""
	if repoMap != nil {
		framework = strings.TrimSpace(repoMap.TestFramework)
	}
	if framework != "" && framework != "unknown" {
		requirements = append(requirements, fmt.Sprintf("Use repository test framework: %s.", framework))
	}
	if brief != nil && brief.TaskType == models.TaskTypeBugfix {
		requirements = append(requirements, "Prefer regression coverage for the reported failure path.")
	}
	if brief != nil && brief.TaskType == models.TaskTypeRefactor {
		requirements = append(requirements, "Verify behavior remains unchanged after refactoring.")
	}
	return requirements
}

func risksFor(brief *models.TaskBrief) []string {
	if brief == nil {
		return nil
	}
	risks := []string{"Scope drift or unrelated edits must be avoided."}
	if brief.RiskLevel == models.RiskHigh {
		risks = append(risks, "High-risk paths require especially careful review and regression validation.")
	}
	switch brief.TaskType {
	case models.TaskTypeBugfix:
		risks = append(risks, "Bug fixes can unintentionally change behavior in adjacent code paths.")
	case models.TaskTypeRefactor:
		risks = append(risks, "Refactors can introduce subtle regressions without obvious API changes.")
	case models.TaskTypeFeature:
		risks = append(risks, "Feature work can expand into unrelated modules if scope is not enforced.")
	}
	return risks
}

func invariantsFor(brief *models.TaskBrief) []string {
	if brief == nil {
		return nil
	}
	invariants := []string{"Do not modify sensitive files or unrelated code paths."}
	switch brief.TaskType {
	case models.TaskTypeBugfix, models.TaskTypeRefactor:
		invariants = append(invariants, "Preserve existing public API behavior unless the task explicitly requires a contract change.")
	case models.TaskTypeTest:
		invariants = append(invariants, "Keep production behavior unchanged unless a minimal testability fix is required.")
	}
	return invariants
}

func forbiddenChangesFor(brief *models.TaskBrief) []string {
	if brief == nil {
		return nil
	}
	forbidden := []string{
		"Do not introduce unrelated refactors.",
		"Do not modify sensitive configuration or secret material.",
		"Do not reformat unrelated files.",
	}
	if brief.TaskType != models.TaskTypeChore {
		forbidden = append(forbidden, "Do not upgrade dependencies unless the task explicitly requires it.")
	}
	return forbidden
}

func buildSteps(brief *models.TaskBrief, inspect, modify []string) []models.PlanStep {
	goal := "the requested task"
	if brief != nil && strings.TrimSpace(brief.NormalizedGoal) != "" {
		goal = brief.NormalizedGoal
	}

	steps := []models.PlanStep{{
		Order:       1,
		Description: fmt.Sprintf("Inspect the most relevant files and confirm scope for %s", goal),
	}}
	if len(modify) > 0 {
		steps = append(steps, models.PlanStep{
			Order:       2,
			Description: "Implement the smallest possible code change that satisfies the acceptance criteria.",
			TargetFile:  modify[0],
		})
	}
	steps = append(steps,
		models.PlanStep{Order: 3, Description: "Validate the resulting patch against scope, safety, and repository conventions."},
		models.PlanStep{Order: 4, Description: "Run the required verification or tests and review the outcome before apply."},
	)
	return steps
}

func rankFiles(description string, taskType models.TaskType, repoMap *models.RepoMap) ([]string, []string) {
	if repoMap == nil || len(repoMap.Files) == 0 {
		return []string{}, []string{}
	}

	tokens := tokenize(description)
	candidates := make([]candidate, 0, len(repoMap.Files))
	for _, file := range repoMap.Files {
		path := strings.ToLower(file.Path)
		score := scorePath(path, tokens, taskType)
		if score <= 0 {
			continue
		}
		candidates = append(candidates, candidate{path: file.Path, score: score})
	}

	if len(candidates) == 0 {
		for _, file := range repoMap.Files {
			candidates = append(candidates, candidate{path: file.Path, score: 1})
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].path < candidates[j].path
		}
		return candidates[i].score > candidates[j].score
	})

	inspect := topUnique(candidates, 6, func(path string) bool { return true })
	modifyFilter := func(path string) bool {
		lower := strings.ToLower(path)
		if taskType == models.TaskTypeTest {
			return isTestPath(lower)
		}
		return !isTestPath(lower) && !isConfigPath(lower)
	}
	modify := topUnique(candidates, 4, modifyFilter)
	if len(modify) == 0 && len(inspect) > 0 {
		modify = append(modify, inspect[0])
	}
	return inspect, modify
}

func topUnique(candidates []candidate, max int, include func(path string) bool) []string {
	result := make([]string, 0, max)
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		if !include(candidate.path) {
			continue
		}
		if _, ok := seen[candidate.path]; ok {
			continue
		}
		seen[candidate.path] = struct{}{}
		result = append(result, candidate.path)
		if len(result) >= max {
			break
		}
	}
	return result
}

func scorePath(path string, tokens []string, taskType models.TaskType) int {
	score := 0
	base := strings.ToLower(filepath.Base(path))
	for _, token := range tokens {
		if strings.Contains(path, token) {
			score += 3
		}
		if strings.Contains(base, token) {
			score += 2
		}
	}
	if taskType == models.TaskTypeTest && isTestPath(path) {
		score += 4
	}
	if taskType != models.TaskTypeTest && !isTestPath(path) {
		score += 1
	}
	if isConfigPath(path) {
		score--
	}
	return score
}

func tokenize(description string) []string {
	parts := strings.Fields(strings.ToLower(description))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		cleaned := strings.Trim(part, `"'.,:;()[]{}<>!?`)
		if len(cleaned) < 3 {
			continue
		}
		if _, ok := stopWords[cleaned]; ok {
			continue
		}
		result = append(result, cleaned)
	}
	return result
}

func isTestPath(path string) bool {
	return strings.Contains(path, "_test.") || strings.Contains(path, ".test.") || strings.Contains(path, ".spec.") || strings.Contains(path, "/test") || strings.Contains(path, "\\test")
}

func isConfigPath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return strings.Contains(base, "config") || strings.Contains(base, ".env") || base == "package.json" || base == "go.mod" || base == "dockerfile"
}

func containsAny(s string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(s, value) {
			return true
		}
	}
	return false
}
