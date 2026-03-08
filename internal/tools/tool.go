// - ReadFile: file reading
// - WriteFile: file writing
// - ListFiles: file listing
// - SearchCode: kod arama
// - ApplyPatch: patch uygulama
package tools

import (
	"fmt"

	"github.com/furkanbeydemir/orch/internal/models"
)

type Tool interface {
	Name() string

	Description() string

	Execute(params map[string]string) (*models.ToolResult, error)
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

func (r *Registry) Get(name string) (Tool, error) {
	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return tool, nil
}

func (r *Registry) Execute(name string, params map[string]string) (*models.ToolResult, error) {
	tool, ok := r.tools[name]
	if !ok {
		return Failure(name, ErrCodeToolNotFound, fmt.Sprintf("tool not found: %s", name), ""), nil
	}

	result, err := tool.Execute(params)
	if err != nil {
		return Failure(name, ErrCodeExecution, err.Error(), ""), nil
	}

	if result == nil {
		return Failure(name, ErrCodeExecution, "tool returned nil result", ""), nil
	}

	return result, nil
}

func (r *Registry) List() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

func DefaultRegistry(repoRoot string) *Registry {
	return DefaultRegistryWithPolicy(repoRoot, Policy{Mode: ModeRun}, nil)
}

func DefaultRegistryWithPolicy(repoRoot string, policy Policy, logf func(string)) *Registry {
	reg := NewRegistry()

	reg.Register(wrapWithPolicy(NewReadFileTool(repoRoot), policy, logf))
	reg.Register(wrapWithPolicy(NewWriteFileTool(repoRoot), policy, logf))
	reg.Register(wrapWithPolicy(NewListFilesTool(repoRoot), policy, logf))

	reg.Register(wrapWithPolicy(NewSearchCodeTool(repoRoot), policy, logf))

	reg.Register(wrapWithPolicy(NewRunCommandTool(repoRoot), policy, logf))
	reg.Register(wrapWithPolicy(NewRunTestsTool(repoRoot), policy, logf))

	reg.Register(wrapWithPolicy(NewGitDiffTool(repoRoot), policy, logf))
	reg.Register(wrapWithPolicy(NewApplyPatchTool(repoRoot), policy, logf))

	return reg
}

func wrapWithPolicy(tool Tool, policy Policy, logf func(string)) Tool {
	return &guardedTool{inner: tool, policy: policy, logf: logf}
}
