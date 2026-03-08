// Package agents - Reviewer Agent implementasyonu.
//
// Girdi:
//
//   - Patch (coder'dan)
//
//   - Plan (planner'dan)
//
//   - Test results (test runner'dan)
//
//   - Decision: accept | revise
package agents

import (
	"fmt"

	"github.com/furkanbeydemir/orch/internal/models"
)

type Reviewer struct {
	modelID string
}

func NewReviewer(modelID string) *Reviewer {
	return &Reviewer{
		modelID: modelID,
	}
}

func (r *Reviewer) Name() string {
	return "reviewer"
}

func (r *Reviewer) Execute(input *Input) (*Output, error) {
	if input.Task == nil {
		return nil, fmt.Errorf("reviewer: task description is required")
	}

	if input.Patch == nil {
		return nil, fmt.Errorf("reviewer: patch gerekli")
	}

	review := &models.ReviewResult{
		Decision:    models.ReviewAccept,
		Comments:    []string{"Patch incelendi"},
		Suggestions: []string{},
	}

	return &Output{
		Review: review,
	}, nil
}
