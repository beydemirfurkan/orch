// Package agents — Explorer agent: read-only codebase reconnaissance.
// The Explorer scans the repo map with an LLM and identifies entry points,
// key symbols, and impact radius before the Planner runs.
package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/providers"
)

// ExplorationResult captures the Explorer's findings.
type ExplorationResult struct {
	EntryPoints  []string `json:"entry_points"`
	KeySymbols   []string `json:"key_symbols"`
	ImpactRadius []string `json:"impact_radius"`
	Summary      string   `json:"summary"`
}

type Explorer struct {
	runtime *LLMRuntime
}

func NewExplorer() *Explorer {
	return &Explorer{}
}

func (e *Explorer) Name() string { return "explorer" }

func (e *Explorer) SetRuntime(runtime *LLMRuntime) {
	e.runtime = runtime
}

func (e *Explorer) Execute(input *Input) (*Output, error) {
	if input.Task == nil {
		return nil, fmt.Errorf("explorer: task is required")
	}

	result := &ExplorationResult{}

	if e.runtime != nil {
		response, err := e.runtime.Chat(context.Background(), providers.ChatRequest{
			Role:         providers.RoleExplorer,
			SystemPrompt: "You are a read-only codebase exploration agent. Identify entry points, key symbols, and files likely impacted by the task. Be concise.",
			UserPrompt:   buildExplorerPrompt(input),
			MaxTokens:    input.MaxTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("explorer provider call failed: %w", err)
		}

		result = parseExplorationResult(response.Text, input)
		return &Output{Usage: response.Usage}, nil
	}

	// Offline fallback: derive a basic result from the repo map.
	result = offlineExploration(input)
	_ = result
	return &Output{}, nil
}

func buildExplorerPrompt(input *Input) string {
	var b strings.Builder
	b.WriteString("Task: ")
	b.WriteString(input.Task.Description)
	if input.RepoMap != nil {
		b.WriteString(fmt.Sprintf("\nLanguage: %s | Files: %d", input.RepoMap.Language, len(input.RepoMap.Files)))
		files := truncateList(filePathsFromRepoMap(input.RepoMap), 40)
		b.WriteString("\nRepository files:\n")
		b.WriteString(strings.Join(files, "\n"))
	}
	b.WriteString("\n\nIdentify:\n1. Entry point files most relevant to this task\n2. Key function/type names to look for\n3. Files likely impacted by changes\nReturn a concise bullet list.")
	return b.String()
}

func parseExplorationResult(text string, input *Input) *ExplorationResult {
	result := &ExplorationResult{Summary: strings.TrimSpace(text)}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimLeft(line, "-•*0123456789. "))
		if line == "" {
			continue
		}
		// Heuristic: lines containing file path characters go to ImpactRadius.
		if strings.Contains(line, "/") || strings.Contains(line, ".go") || strings.Contains(line, ".ts") {
			result.ImpactRadius = append(result.ImpactRadius, line)
		}
	}
	// Seed EntryPoints from plan if available.
	if input.Plan != nil {
		result.EntryPoints = append(result.EntryPoints, input.Plan.FilesToModify...)
	}
	return result
}

func offlineExploration(input *Input) *ExplorationResult {
	result := &ExplorationResult{}
	if input.Plan != nil {
		result.EntryPoints = input.Plan.FilesToModify
		result.ImpactRadius = input.Plan.FilesToInspect
	}
	return result
}

func filePathsFromRepoMap(rm *models.RepoMap) []string {
	if rm == nil {
		return nil
	}
	paths := make([]string, 0, len(rm.Files))
	for _, f := range rm.Files {
		paths = append(paths, f.Path)
	}
	return paths
}
