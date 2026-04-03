// - Coder: plan + relevant files -> unified diff patch
package agents

import (
	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/providers"
)

type Agent interface {
	Name() string

	Execute(input *Input) (*Output, error)
}

type Input struct {
	Task              *models.Task
	TaskBrief         *models.TaskBrief
	RepoMap           *models.RepoMap
	Plan              *models.Plan
	ExecutionContract *models.ExecutionContract
	Patch             *models.Patch
	Context           *models.ContextResult
	ValidationResults []models.ValidationResult
	RetryDirective    *models.RetryDirective
	TestResults       string
	// MaxTokens limits LLM output tokens for this agent call. 0 means provider default.
	MaxTokens    int
	ContextDepth models.ContextDepth
	// Skills lists skill names whose system hints should be injected.
	Skills []string
	// SkillHints is the pre-collected hint string from assigned skills.
	SkillHints string
}

type Output struct {
	Plan   *models.Plan
	Patch  *models.Patch
	Review *models.ReviewResult
	// Usage captures token consumption for the agent call.
	Usage providers.Usage
}
