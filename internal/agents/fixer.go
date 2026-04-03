// Package agents — Fixer agent: fast targeted repair for retry paths.
// The Fixer receives a failing validation result or test failure plus the
// original patch and produces a minimal surgical correction (1–3 file scope).
package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/providers"
)

type Fixer struct {
	runtime *LLMRuntime
}

func NewFixer() *Fixer {
	return &Fixer{}
}

func (f *Fixer) Name() string { return "fixer" }

func (f *Fixer) SetRuntime(runtime *LLMRuntime) {
	f.runtime = runtime
}

func (f *Fixer) Execute(input *Input) (*Output, error) {
	if input.Task == nil {
		return nil, fmt.Errorf("fixer: task is required")
	}

	if f.runtime != nil {
		response, err := f.runtime.Chat(context.Background(), providers.ChatRequest{
			Role:         providers.RoleFixer,
			SystemPrompt: "You are a surgical code fixer. Given a failing patch and the specific error, produce the minimal unified diff correction. Touch at most 3 files. Return only the diff.",
			UserPrompt:   buildFixerPrompt(input),
			MaxTokens:    input.MaxTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("fixer provider call failed: %w", err)
		}

		raw := extractUnifiedDiff(response.Text)
		patch := &models.Patch{TaskID: input.Task.ID, Files: []models.PatchFile{}, RawDiff: raw}
		return &Output{Patch: patch, Usage: response.Usage}, nil
	}

	// No runtime: pass the existing patch through unchanged (let validation retry handle it).
	return &Output{Patch: input.Patch}, nil
}

func buildFixerPrompt(input *Input) string {
	var b strings.Builder
	b.WriteString("Task: ")
	b.WriteString(input.Task.Description)

	if input.RetryDirective != nil {
		b.WriteString(fmt.Sprintf("\nRetry Attempt: %d", input.RetryDirective.Attempt))
		if len(input.RetryDirective.Reasons) > 0 {
			b.WriteString("\nFailure Reasons: ")
			b.WriteString(strings.Join(input.RetryDirective.Reasons, " | "))
		}
		if len(input.RetryDirective.FailedGates) > 0 {
			b.WriteString("\nFailed Gates: ")
			b.WriteString(strings.Join(input.RetryDirective.FailedGates, ", "))
		}
		if len(input.RetryDirective.FailedTests) > 0 {
			b.WriteString("\nFailed Tests: ")
			b.WriteString(strings.Join(input.RetryDirective.FailedTests, " | "))
		}
		if len(input.RetryDirective.Instructions) > 0 {
			b.WriteString("\nFix Instructions: ")
			b.WriteString(strings.Join(input.RetryDirective.Instructions, " | "))
		}
	}

	if input.Patch != nil && strings.TrimSpace(input.Patch.RawDiff) != "" {
		b.WriteString("\nOriginal Patch (abbreviated):\n")
		diff := input.Patch.RawDiff
		if len(diff) > 2000 {
			diff = diff[:2000] + "\n... [truncated]"
		}
		b.WriteString(diff)
	}

	if len(input.ValidationResults) > 0 {
		var failed []string
		for _, vr := range input.ValidationResults {
			if vr.Status == models.ValidationFail {
				failed = append(failed, fmt.Sprintf("%s: %s", vr.Name, vr.Summary))
			}
		}
		if len(failed) > 0 {
			b.WriteString("\nFailed Validations: ")
			b.WriteString(strings.Join(failed, " | "))
		}
	}

	b.WriteString("\nReturn a minimal unified diff patch correcting only the failure. Max 3 files.")
	return b.String()
}
