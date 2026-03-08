// 2. Parse - unified diff format parse
// 5. Apply - apply to working tree
//
// - Patch size limits are enforced
package patch

import (
	"fmt"

	"github.com/furkanbeydemir/orch/internal/models"
)

type Pipeline struct {
	maxFiles  int
	maxLines  int
	validator *Validator
	parser    *Parser
	applier   *Applier
}

func NewPipeline(maxFiles, maxLines int) *Pipeline {
	return &Pipeline{
		maxFiles:  maxFiles,
		maxLines:  maxLines,
		validator: NewValidator(maxFiles, maxLines),
		parser:    NewParser(),
		applier:   NewApplier(),
	}
}

func (p *Pipeline) Process(rawDiff string) (*models.Patch, error) {
	// 1. Parse
	patchResult, err := p.parser.Parse(rawDiff)
	if err != nil {
		return nil, fmt.Errorf("patch parse error: %w", err)
	}

	// 2. Validate
	if err := p.validator.Validate(patchResult); err != nil {
		return nil, fmt.Errorf("patch validation error: %w", err)
	}

	return patchResult, nil
}

func (p *Pipeline) Validate(patch *models.Patch) error {
	return p.validator.Validate(patch)
}

func (p *Pipeline) Preview(patch *models.Patch) string {
	if patch == nil || patch.RawDiff == "" {
		return "No changes to display."
	}
	return patch.RawDiff
}

// Apply applies patch content to the working tree.
func (p *Pipeline) Apply(patch *models.Patch, repoRoot string, dryRun bool) error {
	return p.applier.Apply(patch, repoRoot, dryRun)
}
