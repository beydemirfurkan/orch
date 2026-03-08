package cmd

import (
	"testing"
)

func TestSessionCommandsLifecycle(t *testing.T) {
	repoRoot := t.TempDir()
	t.Chdir(repoRoot)

	if err := runSessionCreate(nil, []string{"feature-a"}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := runSessionCurrent(nil, nil); err != nil {
		t.Fatalf("current session: %v", err)
	}

	if err := runSessionSelect(nil, []string{"default"}); err != nil {
		t.Fatalf("select default session: %v", err)
	}

	if err := runSessionClose(nil, []string{"feature-a"}); err != nil {
		t.Fatalf("close session: %v", err)
	}

	if err := runSessionRuns(nil, []string{"default"}); err != nil {
		t.Fatalf("session runs command: %v", err)
	}
}
