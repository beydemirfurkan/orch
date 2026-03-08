// Package patch contains patch application implementation.
package patch

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
)

type Applier struct{}

type ConflictError struct {
	Files            []string
	Reason           string
	BestPatchSummary string
}

func (e *ConflictError) Error() string {
	if len(e.Files) == 0 {
		return fmt.Sprintf("patch conflict: %s", e.Reason)
	}
	return fmt.Sprintf("patch conflict in %s: %s", strings.Join(e.Files, ", "), e.Reason)
}

type InvalidPatchError struct {
	Reason           string
	BestPatchSummary string
}

func (e *InvalidPatchError) Error() string {
	return fmt.Sprintf("invalid patch: %s", e.Reason)
}

func NewApplier() *Applier {
	return &Applier{}
}

// Apply applies patch content to the target repository.
func (a *Applier) Apply(p *models.Patch, repoRoot string, dryRun bool) error {
	if p == nil || strings.TrimSpace(p.RawDiff) == "" {
		return fmt.Errorf("no patch to apply")
	}

	args := []string{"apply"}

	if dryRun {
		args = append(args, "--check")
	}

	// Stdin'den patch oku
	args = append(args, "-")

	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(p.RawDiff)

	output, err := cmd.CombinedOutput()
	if err != nil {
		summary := Summarize(p)
		files := parseConflictFiles(string(output))
		if len(files) > 0 {
			return &ConflictError{
				Files:            files,
				Reason:           strings.TrimSpace(string(output)),
				BestPatchSummary: summary,
			}
		}

		if isInvalidPatchOutput(string(output)) {
			return &InvalidPatchError{
				Reason:           strings.TrimSpace(string(output)),
				BestPatchSummary: summary,
			}
		}

		mode := "apply"
		if dryRun {
			mode = "dry-run check"
		}
		return fmt.Errorf("patch %s failed: %s\n%s", mode, err.Error(), string(output))
	}

	return nil
}

func (a *Applier) DryRun(p *models.Patch, repoRoot string) error {
	return a.Apply(p, repoRoot, true)
}

var (
	patchFailedPattern    = regexp.MustCompile(`patch failed:\s+([^:]+):`)                 //nolint:gochecknoglobals
	patchDoesNotApplyExpr = regexp.MustCompile(`error:\s+([^:]+):\s+patch does not apply`) //nolint:gochecknoglobals
)

func parseConflictFiles(output string) []string {
	unique := map[string]struct{}{}

	for _, match := range patchFailedPattern.FindAllStringSubmatch(output, -1) {
		if len(match) > 1 {
			unique[strings.TrimSpace(match[1])] = struct{}{}
		}
	}

	for _, match := range patchDoesNotApplyExpr.FindAllStringSubmatch(output, -1) {
		if len(match) > 1 {
			unique[strings.TrimSpace(match[1])] = struct{}{}
		}
	}

	if len(unique) == 0 {
		return nil
	}

	files := make([]string, 0, len(unique))
	for file := range unique {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

func isInvalidPatchOutput(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "no valid patches in input") ||
		strings.Contains(lower, "corrupt patch") ||
		strings.Contains(lower, "malformed patch")
}
