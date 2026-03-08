package tools

import (
	"strings"
	"testing"
)

func TestRunCommandBlocksRiskyCommandWithoutApproval(t *testing.T) {
	tool := NewRunCommandTool(t.TempDir())

	result, err := tool.Execute(map[string]string{"command": "rm -rf /tmp/orch-test"})
	if err != nil {
		t.Fatalf("unexpected execute error: %v", err)
	}
	if result.Success {
		t.Fatalf("expected risky command to be blocked")
	}
	if result.ErrorCode != ErrCodePolicyBlocked {
		t.Fatalf("unexpected error code: %s", result.ErrorCode)
	}
}

func TestRunCommandTimesOut(t *testing.T) {
	tool := NewRunCommandTool(t.TempDir())

	result, err := tool.Execute(map[string]string{
		"command":         "sleep 2",
		"timeout_seconds": "1",
	})
	if err != nil {
		t.Fatalf("unexpected execute error: %v", err)
	}
	if result.Success {
		t.Fatalf("expected timeout failure")
	}
	if result.ErrorCode != ErrCodeTimeout {
		t.Fatalf("unexpected timeout code: %s", result.ErrorCode)
	}
}

func TestRunCommandTruncatesLargeOutputToFile(t *testing.T) {
	tool := NewRunCommandTool(t.TempDir())

	result, err := tool.Execute(map[string]string{
		"command": "seq 1 25000",
	})
	if err != nil {
		t.Fatalf("unexpected execute error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected command success, got error: %s", result.Error)
	}
	if result.Metadata == nil || strings.TrimSpace(result.Metadata["output_file"]) == "" {
		t.Fatalf("expected truncated output metadata with output_file")
	}
}
