// - Coder: plan + relevant files -> unified diff patch
package agents

import (
	"github.com/furkanbeydemir/orch/internal/models"
)

type Agent interface {
	Name() string

	Execute(input *Input) (*Output, error)
}

type Input struct {
	Task        *models.Task
	RepoMap     *models.RepoMap
	Plan        *models.Plan
	Patch       *models.Patch
	Context     *models.ContextResult
	TestResults string
}

type Output struct {
	Plan   *models.Plan
	Patch  *models.Patch
	Review *models.ReviewResult
}
