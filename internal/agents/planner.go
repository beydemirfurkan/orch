// Package agents contains the Planner agent implementation.
package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/providers"
)

type Planner struct {
	modelID string
	runtime *LLMRuntime
}

func NewPlanner(modelID string) *Planner {
	return &Planner{
		modelID: modelID,
	}
}

func (p *Planner) Name() string {
	return "planner"
}

func (p *Planner) SetRuntime(runtime *LLMRuntime) {
	p.runtime = runtime
}

func (p *Planner) Execute(input *Input) (*Output, error) {
	if input.Task == nil {
		return nil, fmt.Errorf("planner: task description is required")
	}

	basePlan := buildBasePlan(input)
	if p.runtime != nil {
		systemPrompt := "You are a planning refinement agent. Refine the plan concisely and keep scope bounded."
		if input.SkillHints != "" {
			systemPrompt += "\n\n" + input.SkillHints
		}
		response, err := p.runtime.Chat(context.Background(), providers.ChatRequest{
			Role:         providers.RolePlanner,
			SystemPrompt: systemPrompt,
			UserPrompt:   buildPlannerPrompt(input),
			MaxTokens:    input.MaxTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("planner provider call failed: %w", err)
		}

		description := strings.TrimSpace(response.Text)
		if description == "" {
			description = fmt.Sprintf("Analyze task: %s", input.Task.Description)
		}

		plan := clonePlan(basePlan)
		if strings.TrimSpace(plan.Summary) == "" {
			plan.Summary = description
		}
		plan.Steps = prependRefinementStep(plan.Steps, description)
		if strings.TrimSpace(plan.TestStrategy) == "" {
			plan.TestStrategy = "Run the configured test command after code changes"
		}

		return &Output{Plan: plan, Usage: response.Usage}, nil
	}

	return &Output{Plan: clonePlan(basePlan)}, nil
}

func buildBasePlan(input *Input) *models.Plan {
	if input != nil && input.Plan != nil {
		return input.Plan
	}
	if input == nil || input.Task == nil {
		return &models.Plan{}
	}
	return &models.Plan{
		TaskID:  input.Task.ID,
		Summary: fmt.Sprintf("Plan task: %s", input.Task.Description),
		Steps: []models.PlanStep{{
			Order:       1,
			Description: fmt.Sprintf("Analyzing task: %s", input.Task.Description),
		}},
		FilesToModify:  []string{},
		FilesToInspect: []string{},
		Risks:          []string{},
		TestStrategy:   "Unit tests will be run",
	}
}

func buildPlannerPrompt(input *Input) string {
	if input == nil || input.Task == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("Task: ")
	b.WriteString(input.Task.Description)
	if input.TaskBrief != nil {
		b.WriteString("\nNormalized Goal: ")
		b.WriteString(input.TaskBrief.NormalizedGoal)
		if input.TaskBrief.TaskType != "" {
			b.WriteString("\nTask Type: ")
			b.WriteString(string(input.TaskBrief.TaskType))
		}
		if input.TaskBrief.RiskLevel != "" {
			b.WriteString("\nRisk Level: ")
			b.WriteString(string(input.TaskBrief.RiskLevel))
		}
	}
	if input.Plan != nil {
		if input.Plan.Summary != "" {
			b.WriteString("\nDraft Plan Summary: ")
			b.WriteString(input.Plan.Summary)
		}
		if len(input.Plan.FilesToInspect) > 0 {
			inspect := truncateList(input.Plan.FilesToInspect, 20)
			b.WriteString("\nCandidate Files To Inspect: ")
			b.WriteString(strings.Join(inspect, ", "))
		}
		if len(input.Plan.AcceptanceCriteria) > 0 {
			criteria := make([]string, 0, len(input.Plan.AcceptanceCriteria))
			for _, criterion := range input.Plan.AcceptanceCriteria {
				criteria = append(criteria, criterion.Description)
			}
			b.WriteString("\nAcceptance Criteria: ")
			b.WriteString(strings.Join(criteria, " | "))
		}
	}
	b.WriteString("\nReturn concise plan refinement guidance only.")
	return b.String()
}

func clonePlan(plan *models.Plan) *models.Plan {
	if plan == nil {
		return &models.Plan{}
	}
	cloned := *plan
	cloned.Steps = append([]models.PlanStep(nil), plan.Steps...)
	cloned.FilesToModify = append([]string(nil), plan.FilesToModify...)
	cloned.FilesToInspect = append([]string(nil), plan.FilesToInspect...)
	cloned.Risks = append([]string(nil), plan.Risks...)
	cloned.TestRequirements = append([]string(nil), plan.TestRequirements...)
	cloned.AcceptanceCriteria = append([]models.AcceptanceCriterion(nil), plan.AcceptanceCriteria...)
	cloned.Invariants = append([]string(nil), plan.Invariants...)
	cloned.ForbiddenChanges = append([]string(nil), plan.ForbiddenChanges...)
	return &cloned
}

func prependRefinementStep(steps []models.PlanStep, description string) []models.PlanStep {
	updated := make([]models.PlanStep, 0, len(steps)+1)
	updated = append(updated, models.PlanStep{Order: 1, Description: description})
	for i, step := range steps {
		step.Order = i + 2
		updated = append(updated, step)
	}
	return updated
}
