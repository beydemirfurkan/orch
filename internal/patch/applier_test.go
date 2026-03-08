package patch

import (
	"reflect"
	"testing"
)

func TestParseConflictFiles(t *testing.T) {
	output := "error: patch failed: internal/orchestrator/orchestrator.go:10\nerror: internal/patch/patch.go: patch does not apply\n"

	files := parseConflictFiles(output)
	expected := []string{"internal/orchestrator/orchestrator.go", "internal/patch/patch.go"}
	if !reflect.DeepEqual(files, expected) {
		t.Fatalf("unexpected files. got=%v want=%v", files, expected)
	}
}

func TestParserRejectsInvalidUnifiedDiff(t *testing.T) {
	parser := NewParser()

	_, err := parser.Parse("this is not a diff")
	if err == nil {
		t.Fatalf("expected parser to reject invalid unified diff")
	}
}
