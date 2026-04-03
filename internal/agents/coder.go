// Package agents contains the Coder agent implementation.
package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/providers"
)

type Coder struct {
	modelID string
	runtime *LLMRuntime
}

func NewCoder(modelID string) *Coder {
	return &Coder{
		modelID: modelID,
	}
}

func (c *Coder) Name() string {
	return "coder"
}

func (c *Coder) SetRuntime(runtime *LLMRuntime) {
	c.runtime = runtime
}

func (c *Coder) Execute(input *Input) (*Output, error) {
	if input.Task == nil {
		return nil, fmt.Errorf("coder: task description is required")
	}

	if input.Plan == nil {
		return nil, fmt.Errorf("coder: plan is required")
	}

	if c.runtime != nil {
		systemPrompt := "You are a constrained coding agent. Return a unified diff patch only, keep scope minimal, and obey the execution contract."
		if input.SkillHints != "" {
			systemPrompt += "\n\n" + input.SkillHints
		}
		response, err := c.runtime.Chat(context.Background(), providers.ChatRequest{
			Role:         providers.RoleCoder,
			SystemPrompt: systemPrompt,
			UserPrompt:   buildCoderPrompt(input),
			MaxTokens:    input.MaxTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("coder provider call failed: %w", err)
		}

		raw := extractUnifiedDiff(response.Text)
		patch := &models.Patch{TaskID: input.Task.ID, Files: []models.PatchFile{}, RawDiff: raw}
		return &Output{Patch: patch, Usage: response.Usage}, nil
	}

	patch := &models.Patch{
		TaskID:  input.Task.ID,
		Files:   []models.PatchFile{},
		RawDiff: "",
	}

	return &Output{
		Patch: patch,
	}, nil
}

func buildCoderPrompt(input *Input) string {
	if input == nil || input.Task == nil || input.Plan == nil {
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
			b.WriteString("\nPlan Summary: ")
			b.WriteString(input.Plan.Summary)
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
	if input.ExecutionContract != nil {
		if len(input.ExecutionContract.AllowedFiles) > 0 {
			b.WriteString("\nAllowed Files: ")
			b.WriteString(strings.Join(input.ExecutionContract.AllowedFiles, ", "))
		}
		if len(input.ExecutionContract.InspectFiles) > 0 {
			b.WriteString("\nInspect Files: ")
			b.WriteString(strings.Join(input.ExecutionContract.InspectFiles, ", "))
		}
		if len(input.ExecutionContract.RequiredEdits) > 0 {
			b.WriteString("\nRequired Edits: ")
			b.WriteString(strings.Join(input.ExecutionContract.RequiredEdits, " | "))
		}
		if len(input.ExecutionContract.ProhibitedActions) > 0 {
			b.WriteString("\nProhibited Actions: ")
			b.WriteString(strings.Join(input.ExecutionContract.ProhibitedActions, " | "))
		}
		if input.ExecutionContract.PatchBudget.MaxFiles > 0 || input.ExecutionContract.PatchBudget.MaxChangedLines > 0 {
			b.WriteString(fmt.Sprintf("\nPatch Budget: max_files=%d max_changed_lines=%d", input.ExecutionContract.PatchBudget.MaxFiles, input.ExecutionContract.PatchBudget.MaxChangedLines))
		}
	}
	if input.Context != nil && len(input.Context.RelatedTests) > 0 {
		// Build set of base names from FilesToModify for filtering
		allowedBases := make(map[string]struct{})
		if input.Plan != nil {
			for _, f := range input.Plan.FilesToModify {
				allowedBases[baseNameNoExt(f)] = struct{}{}
			}
		}
		filtered := filterByBasename(input.Context.RelatedTests, allowedBases)
		if len(filtered) > 0 {
			b.WriteString("\nRelated Tests: ")
			b.WriteString(strings.Join(filtered, ", "))
		}
	}
	if input.RetryDirective != nil {
		b.WriteString("\nRetry Stage: ")
		b.WriteString(input.RetryDirective.Stage)
		b.WriteString(fmt.Sprintf("\nRetry Attempt: %d", input.RetryDirective.Attempt))
		if len(input.RetryDirective.Reasons) > 0 {
			b.WriteString("\nRetry Reasons: ")
			b.WriteString(strings.Join(input.RetryDirective.Reasons, " | "))
		}
		if len(input.RetryDirective.FailedGates) > 0 {
			b.WriteString("\nFailed Gates: ")
			b.WriteString(strings.Join(input.RetryDirective.FailedGates, ", "))
		}
		if len(input.RetryDirective.FailedTests) > 0 {
			b.WriteString("\nFailed Tests: ")
			b.WriteString(strings.Join(input.RetryDirective.FailedTests, " | "))
		}
		if len(input.RetryDirective.Instructions) > 0 {
			b.WriteString("\nRetry Instructions: ")
			b.WriteString(strings.Join(input.RetryDirective.Instructions, " | "))
		}
		if len(input.RetryDirective.Avoid) > 0 {
			b.WriteString("\nAvoid: ")
			b.WriteString(strings.Join(input.RetryDirective.Avoid, " | "))
		}
	}
	b.WriteString("\nReturn unified diff patch only.")
	return b.String()
}

func extractUnifiedDiff(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	idx := strings.Index(trimmed, "diff --git")
	if idx >= 0 {
		return strings.TrimSpace(trimmed[idx:]) + "\n"
	}
	return ""
}
