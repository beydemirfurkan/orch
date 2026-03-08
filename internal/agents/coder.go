// Package agents contains the Coder agent implementation.
package agents

import (
	"fmt"

	"github.com/furkanbeydemir/orch/internal/models"
)

type Coder struct {
	modelID string
}

func NewCoder(modelID string) *Coder {
	return &Coder{
		modelID: modelID,
	}
}

func (c *Coder) Name() string {
	return "coder"
}

func (c *Coder) Execute(input *Input) (*Output, error) {
	if input.Task == nil {
		return nil, fmt.Errorf("coder: task description is required")
	}

	if input.Plan == nil {
		return nil, fmt.Errorf("coder: plan is required")
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
