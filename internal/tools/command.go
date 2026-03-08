package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/furkanbeydemir/orch/internal/models"
)

const (
	defaultCommandTimeout = 60 * time.Second
	defaultTestTimeout    = 120 * time.Second
	maxOutputBytes        = 50 * 1024
)

type RunCommandTool struct {
	repoRoot string
}

func NewRunCommandTool(repoRoot string) *RunCommandTool {
	return &RunCommandTool{repoRoot: repoRoot}
}

func (t *RunCommandTool) Name() string { return "run_command" }

func (t *RunCommandTool) Description() string { return "Runs a system command" }

func (t *RunCommandTool) Execute(params map[string]string) (*models.ToolResult, error) {
	command, ok := params["command"]
	if !ok {
		return Failure("run_command", ErrCodeInvalidParams, "run_command: 'command' parameter is required", ""), nil
	}

	if risky, reason := classifyCommandRisk(command); risky && strings.TrimSpace(params["approved"]) != "true" {
		return Failure("run_command", ErrCodePolicyBlocked, fmt.Sprintf("command blocked by safety policy: %s", reason), ""), nil
	}

	timeout := parseTimeout(params, defaultCommandTimeout)
	return runCommand("run_command", t.repoRoot, command, timeout)
}

type RunTestsTool struct {
	repoRoot string
}

func NewRunTestsTool(repoRoot string) *RunTestsTool {
	return &RunTestsTool{repoRoot: repoRoot}
}

func (t *RunTestsTool) Name() string { return "run_tests" }

func (t *RunTestsTool) Description() string { return "Runs project tests" }

// Params: "command" optional test command (default: "go test ./...").
func (t *RunTestsTool) Execute(params map[string]string) (*models.ToolResult, error) {
	command := params["command"]
	if command == "" {
		command = "go test ./..."
	}

	timeout := parseTimeout(params, defaultTestTimeout)
	return runCommand("run_tests", t.repoRoot, command, timeout)
}

func runCommand(toolName, repoRoot, command string, timeout time.Duration) (*models.ToolResult, error) {

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return Failure(toolName, ErrCodeInvalidParams, "empty command", ""), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = repoRoot

	output, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return Failure(toolName, ErrCodeTimeout, fmt.Sprintf("command timed out after %s", timeout), truncateOutput(string(output))), nil
	}

	normalizedOutput, truncatedPath, truncated := normalizeOutput(repoRoot, toolName, output)
	if err != nil {
		result := Failure(toolName, ErrCodeExecution, err.Error(), normalizedOutput)
		if truncated {
			result.ErrorCode = ErrCodeOutputTrunc
			result.Metadata = map[string]string{"output_file": truncatedPath}
		}
		return result, nil
	}

	result := Success(toolName, normalizedOutput)
	if truncated {
		result.Metadata = map[string]string{"output_file": truncatedPath}
	}
	return result, nil
}

func parseTimeout(params map[string]string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(params["timeout_seconds"])
	if raw == "" {
		return fallback
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func classifyCommandRisk(command string) (bool, string) {
	lower := strings.ToLower(strings.TrimSpace(command))
	riskyPatterns := []string{
		"rm -rf",
		"mkfs",
		"dd if=",
		"shutdown",
		"reboot",
		":(){",
	}
	for _, pattern := range riskyPatterns {
		if strings.Contains(lower, pattern) {
			return true, pattern
		}
	}
	return false, ""
}

func normalizeOutput(repoRoot, toolName string, output []byte) (string, string, bool) {
	text := string(output)
	if len(output) <= maxOutputBytes {
		return text, "", false
	}

	if err := os.MkdirAll(filepath.Join(repoRoot, ".orch", "runs"), 0o755); err != nil {
		return truncateOutput(text), "", true
	}

	path := filepath.Join(repoRoot, ".orch", "runs", fmt.Sprintf("%s-output-%d.log", toolName, time.Now().UnixNano()))
	if err := os.WriteFile(path, output, 0o644); err != nil {
		return truncateOutput(text), "", true
	}

	return fmt.Sprintf("output truncated; full output saved to %s\n%s", path, truncateOutput(text)), path, true
}

func truncateOutput(text string) string {
	if len(text) <= maxOutputBytes {
		return text
	}
	return text[:maxOutputBytes]
}
