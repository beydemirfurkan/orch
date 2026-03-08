// Package agents contains the Reviewer agent implementation.
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
		return nil, fmt.Errorf("reviewer: patch is required")
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
