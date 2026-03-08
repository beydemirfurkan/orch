package tools

import "testing"

func TestToolsReturnInvalidParamsContract(t *testing.T) {
	repoRoot := t.TempDir()

	tests := []struct {
		name string
		tool Tool
	}{
		{name: "read_file", tool: NewReadFileTool(repoRoot)},
		{name: "write_file", tool: NewWriteFileTool(repoRoot)},
		{name: "search_code", tool: NewSearchCodeTool(repoRoot)},
		{name: "run_command", tool: NewRunCommandTool(repoRoot)},
		{name: "apply_patch", tool: NewApplyPatchTool(repoRoot)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.tool.Execute(map[string]string{})
			if err != nil {
				t.Fatalf("unexpected execute error: %v", err)
			}
			if result == nil {
				t.Fatalf("expected tool result")
			}
			if result.Success {
				t.Fatalf("expected failure result for invalid params")
			}
			if result.ErrorCode != ErrCodeInvalidParams {
				t.Fatalf("unexpected error code: %s", result.ErrorCode)
			}
		})
	}
}
