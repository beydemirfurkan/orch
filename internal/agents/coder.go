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
		response, err := c.runtime.Chat(context.Background(), providers.ChatRequest{
			Role:         providers.RoleCoder,
			SystemPrompt: "You are a coding agent. Return a unified diff patch when possible.",
			UserPrompt:   buildCoderPrompt(input),
		})
		if err != nil {
			return nil, fmt.Errorf("coder provider call failed: %w", err)
		}

		raw := extractUnifiedDiff(response.Text)
		patch := &models.Patch{TaskID: input.Task.ID, Files: []models.PatchFile{}, RawDiff: raw}
		return &Output{Patch: patch}, nil
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
	return fmt.Sprintf("Task: %s\nPlan steps: %d\nReturn unified diff patch.", input.Task.Description, len(input.Plan.Steps))
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
