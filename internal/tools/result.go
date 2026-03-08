package tools

import "github.com/furkanbeydemir/orch/internal/models"

const (
	ErrCodeInvalidParams = "invalid_params"
	ErrCodeExecution     = "execution_error"
	ErrCodePolicyBlocked = "policy_blocked"
	ErrCodeTimeout       = "timeout"
	ErrCodeOutputTrunc   = "output_truncated"
	ErrCodeToolNotFound  = "tool_not_found"
)

func Success(toolName, output string) *models.ToolResult {
	return &models.ToolResult{
		ToolName: toolName,
		Success:  true,
		Output:   output,
	}
}

func Failure(toolName, code, message, output string) *models.ToolResult {
	return &models.ToolResult{
		ToolName:  toolName,
		Success:   false,
		Output:    output,
		Error:     message,
		ErrorCode: code,
	}
}
