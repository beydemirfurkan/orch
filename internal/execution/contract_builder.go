package execution

import (
	"strings"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
)

type ContractBuilder struct {
	cfg *config.Config
}

func NewContractBuilder(cfg *config.Config) *ContractBuilder {
	return &ContractBuilder{cfg: cfg}
}

func (b *ContractBuilder) Build(task *models.Task, brief *models.TaskBrief, plan *models.Plan, ctx *models.ContextResult) *models.ExecutionContract {
	if task == nil {
		return nil
	}

	allowedFiles := uniqueNonEmpty(planFilesToModify(plan, ctx))
	inspectFiles := uniqueNonEmpty(planFilesToInspect(plan, ctx))
	requiredEdits := buildRequiredEdits(plan)
	prohibitedActions := buildProhibitedActions(plan)
	acceptanceCriteria := buildAcceptanceCriteria(plan)
	invariants := buildInvariants(plan, brief)

	return &models.ExecutionContract{
		TaskID:             task.ID,
		AllowedFiles:       allowedFiles,
		InspectFiles:       inspectFiles,
		RequiredEdits:      requiredEdits,
		ProhibitedActions:  prohibitedActions,
		AcceptanceCriteria: acceptanceCriteria,
		Invariants:         invariants,
		PatchBudget: models.PatchBudget{
			MaxFiles:        patchMaxFiles(b.cfg),
			MaxChangedLines: patchMaxLines(b.cfg),
		},
		ScopeExpansionPolicy: models.ScopeExpansionPolicy{
			Allowed:        true,
			RequiresReason: true,
			MaxExtraFiles:  1,
		},
	}
}

func planFilesToModify(plan *models.Plan, ctx *models.ContextResult) []string {
	files := make([]string, 0)
	if plan != nil {
		files = append(files, plan.FilesToModify...)
	}
	if len(files) == 0 && ctx != nil {
		files = append(files, ctx.SelectedFiles...)
	}
	return files
}

func planFilesToInspect(plan *models.Plan, ctx *models.ContextResult) []string {
	files := make([]string, 0)
	if plan != nil {
		files = append(files, plan.FilesToInspect...)
		files = append(files, plan.FilesToModify...)
	}
	if ctx != nil {
		files = append(files, ctx.SelectedFiles...)
		files = append(files, ctx.RelatedTests...)
		files = append(files, ctx.RelevantConfigs...)
	}
	return files
}

func buildRequiredEdits(plan *models.Plan) []string {
	if plan == nil {
		return []string{}
	}
	items := make([]string, 0, len(plan.AcceptanceCriteria)+len(plan.Steps))
	for _, criterion := range plan.AcceptanceCriteria {
		if strings.TrimSpace(criterion.Description) == "" {
			continue
		}
		items = append(items, criterion.Description)
	}
	if len(items) == 0 {
		for _, step := range plan.Steps {
			if strings.TrimSpace(step.Description) == "" {
				continue
			}
			items = append(items, step.Description)
		}
	}
	return uniqueNonEmpty(items)
}

func buildProhibitedActions(plan *models.Plan) []string {
	items := []string{
		"Do not modify files outside the allowed file set unless scope expansion is explicitly justified.",
		"Do not introduce unrelated refactors or formatting-only churn.",
		"Do not change sensitive files, secrets, or unrelated configuration.",
	}
	if plan != nil {
		items = append(items, plan.ForbiddenChanges...)
	}
	return uniqueNonEmpty(items)
}

func buildAcceptanceCriteria(plan *models.Plan) []string {
	if plan == nil {
		return []string{}
	}
	items := make([]string, 0, len(plan.AcceptanceCriteria))
	for _, criterion := range plan.AcceptanceCriteria {
		items = append(items, criterion.Description)
	}
	return uniqueNonEmpty(items)
}

func buildInvariants(plan *models.Plan, brief *models.TaskBrief) []string {
	items := make([]string, 0)
	if plan != nil {
		items = append(items, plan.Invariants...)
	}
	if brief != nil && brief.RiskLevel == models.RiskHigh {
		items = append(items, "Preserve existing behavior and public interfaces unless the task explicitly requires a contract change.")
	}
	return uniqueNonEmpty(items)
}

func patchMaxFiles(cfg *config.Config) int {
	if cfg == nil || cfg.Patch.MaxFiles <= 0 {
		return 10
	}
	return cfg.Patch.MaxFiles
}

func patchMaxLines(cfg *config.Config) int {
	if cfg == nil || cfg.Patch.MaxLines <= 0 {
		return 800
	}
	return cfg.Patch.MaxLines
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
