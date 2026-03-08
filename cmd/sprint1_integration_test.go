package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/config"
)

func TestRunBlockedByRepoLock(t *testing.T) {
	repoRoot := t.TempDir()
	t.Chdir(repoRoot)

	if err := config.EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}

	cfg := config.DefaultConfig()
	if err := config.Save(repoRoot, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	lockContent := fmt.Sprintf(`{"pid":%d,"run_id":"other-run","created_at":"%s"}`,
		os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
	lockPath := filepath.Join(repoRoot, config.OrchDir, "lock")
	if err := os.WriteFile(lockPath, []byte(lockContent), 0o644); err != nil {
		t.Fatalf("write lock file: %v", err)
	}

	err := runRun(nil, []string{"dummy task"})
	if err == nil {
		t.Fatalf("expected run to be blocked by repo lock")
	}
	if !strings.Contains(err.Error(), "run blocked by repository lock") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyRequiresDestructiveApproval(t *testing.T) {
	repoRoot := t.TempDir()
	t.Chdir(repoRoot)

	if err := config.EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Safety.DryRun = true
	cfg.Safety.RequireDestructiveApproval = true
	if err := config.Save(repoRoot, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	patchContent := strings.Join([]string{
		"diff --git a/demo.txt b/demo.txt",
		"--- a/demo.txt",
		"+++ b/demo.txt",
		"@@ -1 +1 @@",
		"-hello",
		"+world",
		"",
	}, "\n")
	patchPath := filepath.Join(repoRoot, config.OrchDir, "latest.patch")
	if err := os.WriteFile(patchPath, []byte(patchContent), 0o644); err != nil {
		t.Fatalf("write latest patch: %v", err)
	}

	forceApply = true
	approveDestructive = false
	t.Cleanup(func() {
		forceApply = false
		approveDestructive = false
	})

	err := runApply(nil, nil)
	if err == nil {
		t.Fatalf("expected destructive apply to be blocked")
	}
	if !strings.Contains(err.Error(), "destructive apply blocked") {
		t.Fatalf("unexpected error: %v", err)
	}
}
