package tools

import (
	"errors"
	"testing"

	"github.com/furkanbeydemir/orch/internal/models"
)

func TestRegistryExecuteReturnsStructuredNotFound(t *testing.T) {
	reg := NewRegistry()

	result, err := reg.Execute("missing_tool", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected result")
	}
	if result.Success {
		t.Fatalf("expected failure result")
	}
	if result.ErrorCode != ErrCodeToolNotFound {
		t.Fatalf("unexpected error code: %s", result.ErrorCode)
	}
}

func TestRegistryExecuteWrapsToolErrors(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&failingTool{})

	result, err := reg.Execute("failing_tool", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatalf("expected failure result")
	}
	if result.ErrorCode != ErrCodeExecution {
		t.Fatalf("unexpected error code: %s", result.ErrorCode)
	}
}

type failingTool struct{}

func (f *failingTool) Name() string { return "failing_tool" }

func (f *failingTool) Description() string { return "fails intentionally" }

func (f *failingTool) Execute(params map[string]string) (*models.ToolResult, error) {
	return nil, errors.New("boom")
}
