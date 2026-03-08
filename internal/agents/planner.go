// Package agents contains the Planner agent implementation.
package agents

import (
	"fmt"

	"github.com/furkanbeydemir/orch/internal/models"
)

type Planner struct {
	modelID string
}

func NewPlanner(modelID string) *Planner {
	return &Planner{
		modelID: modelID,
	}
}

func (p *Planner) Name() string {
	return "planner"
}

func (p *Planner) Execute(input *Input) (*Output, error) {
	if input.Task == nil {
		return nil, fmt.Errorf("planner: task description is required")
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
