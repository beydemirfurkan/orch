// - ApplyPatch: patch uygulama
package tools

import (
	"os/exec"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
)

type GitDiffTool struct {
	repoRoot string
}

func NewGitDiffTool(repoRoot string) *GitDiffTool {
	return &GitDiffTool{repoRoot: repoRoot}
}

func (t *GitDiffTool) Name() string { return "git_diff" }

func (t *GitDiffTool) Description() string { return "Produces git diff output" }

func (t *GitDiffTool) Execute(params map[string]string) (*models.ToolResult, error) {
	args := []string{"diff"}
	if path, ok := params["path"]; ok && path != "" {
		args = append(args, "--", path)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = t.repoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return Failure("git_diff", ErrCodeExecution, err.Error(), string(output)), nil
	}

	return Success("git_diff", string(output)), nil
}

type ApplyPatchTool struct {
	repoRoot string
}

func NewApplyPatchTool(repoRoot string) *ApplyPatchTool {
	return &ApplyPatchTool{repoRoot: repoRoot}
}

func (t *ApplyPatchTool) Name() string { return "apply_patch" }

func (t *ApplyPatchTool) Description() string { return "Git patch uygular" }

// Execute, patch'i git apply ile uygular.
func (t *ApplyPatchTool) Execute(params map[string]string) (*models.ToolResult, error) {
	patchContent, ok := params["patch"]
	if !ok {
		return Failure("apply_patch", ErrCodeInvalidParams, "apply_patch: 'patch' parameter is required", ""), nil
	}

	args := []string{"apply"}

	if dryRun, exists := params["dry_run"]; exists && dryRun == "true" {
		args = append(args, "--check")
	}

	args = append(args, "-")

	cmd := exec.Command("git", args...)
	cmd.Dir = t.repoRoot
	cmd.Stdin = strings.NewReader(patchContent)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return Failure("apply_patch", ErrCodeExecution, err.Error(), string(output)), nil
	}

	return Success("apply_patch", "Patch applied successfully"), nil
}
