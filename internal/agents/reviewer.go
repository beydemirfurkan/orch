// Package agents contains the Reviewer agent implementation.
package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/providers"
)

type Reviewer struct {
	modelID string
	runtime *LLMRuntime
}

func NewReviewer(modelID string) *Reviewer {
	return &Reviewer{
		modelID: modelID,
	}
}

func (r *Reviewer) Name() string {
	return "reviewer"
}

func (r *Reviewer) SetRuntime(runtime *LLMRuntime) {
	r.runtime = runtime
}

func (r *Reviewer) Execute(input *Input) (*Output, error) {
	if input.Task == nil {
		return nil, fmt.Errorf("reviewer: task description is required")
	}

	if input.Patch == nil {
		return nil, fmt.Errorf("reviewer: patch is required")
	}

	if r.runtime != nil {
		response, err := r.runtime.Chat(context.Background(), providers.ChatRequest{
			Role:         providers.RoleReviewer,
			SystemPrompt: "You are a reviewer. Decide accept or revise and give concise feedback.",
			UserPrompt:   buildReviewerPrompt(input),
		})
		if err != nil {
			return nil, fmt.Errorf("reviewer provider call failed: %w", err)
		}

		review := parseReviewResponse(response.Text)
		return &Output{Review: review}, nil
	}

	review := &models.ReviewResult{
		Decision:    models.ReviewAccept,
		Comments:    []string{"Patch reviewed"},
		Suggestions: []string{},
	}

	return &Output{
		Review: review,
	}, nil
}

func buildReviewerPrompt(input *Input) string {
	if input == nil || input.Task == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("Task: ")
	b.WriteString(input.Task.Description)
	b.WriteString(fmt.Sprintf("\nPatch length: %d chars", len(input.Patch.RawDiff)))
	if input.TaskBrief != nil {
		b.WriteString("\nTask Type: ")
		b.WriteString(string(input.TaskBrief.TaskType))
		b.WriteString("\nRisk Level: ")
		b.WriteString(string(input.TaskBrief.RiskLevel))
	}
	if input.Plan != nil && len(input.Plan.AcceptanceCriteria) > 0 {
		criteria := make([]string, 0, len(input.Plan.AcceptanceCriteria))
		for _, criterion := range input.Plan.AcceptanceCriteria {
			criteria = append(criteria, criterion.Description)
		}
		b.WriteString("\nAcceptance Criteria: ")
		b.WriteString(strings.Join(criteria, " | "))
	}
	if len(input.ValidationResults) > 0 {
		parts := make([]string, 0, len(input.ValidationResults))
		for _, result := range input.ValidationResults {
			parts = append(parts, fmt.Sprintf("%s=%s", result.Name, result.Status))
		}
		b.WriteString("\nValidation Gates: ")
		b.WriteString(strings.Join(parts, ", "))
	}
	b.WriteString("\nTest results: ")
	b.WriteString(input.TestResults)
	b.WriteString("\nDecide accept or revise and give concise findings.")
	return b.String()
}

func parseReviewResponse(text string) *models.ReviewResult {
	trimmed := strings.TrimSpace(text)
	decision := models.ReviewAccept
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "revise") {
		decision = models.ReviewRevise
	}
	if trimmed == "" {
		trimmed = "Patch reviewed"
	}
	return &models.ReviewResult{
		Decision:    decision,
		Comments:    []string{trimmed},
		Suggestions: []string{},
	}
}
