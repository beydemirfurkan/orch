package tools

import (
	"fmt"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
)

const (
	ModeRun  = "run"
	ModePlan = "plan"
)

type Policy struct {
	Mode                       string
	RequireDestructiveApproval bool
}

type policyDecision struct {
	allowed bool
	reason  string
}

func (p Policy) Decide(toolName string, params map[string]string) policyDecision {
	if p.Mode == "" {
		p.Mode = ModeRun
	}

	if p.Mode == ModePlan {
		switch toolName {
		case "write_file", "apply_patch", "run_command":
			return policyDecision{
				allowed: false,
				reason:  fmt.Sprintf("%s blocked in plan mode (read-only)", toolName),
			}
		}
	}

	if p.RequireDestructiveApproval && isDestructiveTool(toolName) {
		if strings.TrimSpace(params["approved"]) != "true" {
			return policyDecision{
				allowed: false,
				reason:  fmt.Sprintf("%s requires explicit approval (set approved=true)", toolName),
			}
		}
	}

	return policyDecision{allowed: true}
}

func isDestructiveTool(toolName string) bool {
	switch toolName {
	case "write_file", "apply_patch", "run_command":
		return true
	default:
		return false
	}
}

type guardedTool struct {
	inner  Tool
	policy Policy
	logf   func(string)
}

func (t *guardedTool) Name() string {
	return t.inner.Name()
}

func (t *guardedTool) Description() string {
	return t.inner.Description()
}

func (t *guardedTool) Execute(params map[string]string) (*models.ToolResult, error) {
	decision := t.policy.Decide(t.inner.Name(), params)
	t.logDecision(t.inner.Name(), decision)
	if !decision.allowed {
		return Failure(t.inner.Name(), ErrCodePolicyBlocked, decision.reason, ""), nil
	}

	return t.inner.Execute(params)
}

func (t *guardedTool) logDecision(toolName string, decision policyDecision) {
	if t.logf == nil {
		return
	}
	result := "allow"
	if !decision.allowed {
		result = "deny"
	}
	message := fmt.Sprintf("policy decision tool=%s mode=%s result=%s", toolName, t.policy.Mode, result)
	if decision.reason != "" {
		message = fmt.Sprintf("%s reason=%s", message, decision.reason)
	}
	t.logf(message)
}
