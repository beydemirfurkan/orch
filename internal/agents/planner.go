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

	if p.runtime != nil {
		response, err := p.runtime.Chat(context.Background(), providers.ChatRequest{
			Role:         providers.RolePlanner,
			SystemPrompt: "You are a planning agent. Return concise implementation planning guidance.",
			UserPrompt:   buildPlannerPrompt(input),
		})
		if err != nil {
			return nil, fmt.Errorf("planner provider call failed: %w", err)
		}

		description := strings.TrimSpace(response.Text)
		if description == "" {
			description = fmt.Sprintf("Analyze task: %s", input.Task.Description)
		}

		plan := &models.Plan{
			TaskID: input.Task.ID,
			Steps: []models.PlanStep{{
				Order:       1,
				Description: description,
			}},
			FilesToModify:  []string{},
			FilesToInspect: []string{},
			Risks:          []string{},
			TestStrategy:   "Run the configured test command after code changes",
		}

		return &Output{Plan: plan}, nil
	}

	plan := &models.Plan{
		TaskID: input.Task.ID,
		Steps: []models.PlanStep{
			{
				Order:       1,
				Description: fmt.Sprintf("Analyzing task: %s", input.Task.Description),
			},
		},
		FilesToModify:  []string{},
		FilesToInspect: []string{},
		Risks:          []string{},
		TestStrategy:   "Unit tests will be run",
	}

	return &Output{
		Plan: plan,
	}, nil
}

func buildPlannerPrompt(input *Input) string {
	if input == nil || input.Task == nil {
		return ""
	}
	return fmt.Sprintf("Task: %s", input.Task.Description)
}
