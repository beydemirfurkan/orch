// Package agents — Oracle agent: read-only senior advisor.
// The Oracle reviews the Planner's output for architectural concerns,
// scope creep, and invariant violations before coding begins.
package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/furkanbeydemir/orch/internal/providers"
)

// OracleAdvisory contains the Oracle's findings.
type OracleAdvisory struct {
	Concerns       []string `json:"concerns"`
	ApprovedSteps  []int    `json:"approved_steps"`
	FlaggedSteps   []int    `json:"flagged_steps"`
	Recommendation string   `json:"recommendation"`
	// NeedsReplan is true when the Oracle found issues serious enough
	// to warrant a Planner re-run before coding.
	NeedsReplan bool `json:"needs_replan"`
}

type Oracle struct {
	runtime *LLMRuntime
}

func NewOracle() *Oracle {
	return &Oracle{}
}

func (o *Oracle) Name() string { return "oracle" }

func (o *Oracle) SetRuntime(runtime *LLMRuntime) {
	o.runtime = runtime
}

func (o *Oracle) Execute(input *Input) (*Output, error) {
	if input.Task == nil {
		return nil, fmt.Errorf("oracle: task is required")
	}

	if o.runtime != nil {
		response, err := o.runtime.Chat(context.Background(), providers.ChatRequest{
			Role:         providers.RoleOracle,
			SystemPrompt: "You are a senior software architect. Review the plan for architectural concerns, scope creep, and invariant violations. Be concise and decisive.",
			UserPrompt:   buildOraclePrompt(input),
			MaxTokens:    input.MaxTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("oracle provider call failed: %w", err)
		}

		advisory := parseOracleAdvisory(response.Text)
		_ = advisory
		return &Output{Usage: response.Usage}, nil
	}

	return &Output{}, nil
}

func buildOraclePrompt(input *Input) string {
	var b strings.Builder
	b.WriteString("Task: ")
	b.WriteString(input.Task.Description)
	if input.TaskBrief != nil {
		b.WriteString("\nRisk Level: ")
		b.WriteString(string(input.TaskBrief.RiskLevel))
	}
	if input.Plan != nil {
		b.WriteString("\nPlan Summary: ")
		b.WriteString(input.Plan.Summary)
		if len(input.Plan.Steps) > 0 {
			b.WriteString("\nPlanned Steps:")
			for _, step := range input.Plan.Steps {
				b.WriteString(fmt.Sprintf("\n  %d. %s", step.Order, step.Description))
			}
		}
		if len(input.Plan.FilesToModify) > 0 {
			b.WriteString("\nFiles To Modify: ")
			b.WriteString(strings.Join(input.Plan.FilesToModify, ", "))
		}
		if len(input.Plan.Risks) > 0 {
			b.WriteString("\nKnown Risks: ")
			b.WriteString(strings.Join(input.Plan.Risks, " | "))
		}
		if len(input.Plan.Invariants) > 0 {
			b.WriteString("\nInvariants: ")
			b.WriteString(strings.Join(input.Plan.Invariants, " | "))
		}
	}
	b.WriteString("\n\nReview for: architectural concerns, scope creep, invariant violations.")
	b.WriteString("\nRespond with: APPROVED or NEEDS_REPLAN, followed by bullet-point concerns if any.")
	return b.String()
}

func parseOracleAdvisory(text string) *OracleAdvisory {
	advisory := &OracleAdvisory{}
	lower := strings.ToLower(strings.TrimSpace(text))

	advisory.NeedsReplan = strings.Contains(lower, "needs_replan") || strings.Contains(lower, "needs replan")
	if !advisory.NeedsReplan {
		advisory.Recommendation = "approved"
	} else {
		advisory.Recommendation = "replan"
	}

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "-•* "))
		if line == "" || strings.HasPrefix(strings.ToUpper(line), "APPROVED") || strings.HasPrefix(strings.ToUpper(line), "NEEDS") {
			continue
		}
		advisory.Concerns = append(advisory.Concerns, line)
	}
	return advisory
}
