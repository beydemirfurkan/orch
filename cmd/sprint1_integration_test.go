package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/storage"
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
	store, err := storage.Open(repoRoot)
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()

	projectID, err := store.GetOrCreateProject()
	if err != nil {
		t.Fatalf("get project id: %v", err)
	}
	sess, err := store.EnsureDefaultSession(projectID)
	if err != nil {
		t.Fatalf("ensure default session: %v", err)
	}

	err = store.SaveRunState(&models.RunState{
		ID:        "run-test-apply",
		ProjectID: projectID,
		SessionID: sess.ID,
		Task: models.Task{
			ID:          "task-apply",
			Description: "apply test task",
			CreatedAt:   time.Now(),
		},
		Status: models.StatusCompleted,
		Patch: &models.Patch{
			TaskID:  "task-apply",
			RawDiff: patchContent,
		},
		Retries:   models.RetryState{},
		StartedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("save run state: %v", err)
	}

	forceApply = true
	approveDestructive = false
	t.Cleanup(func() {
		forceApply = false
		approveDestructive = false
	})

	err = runApply(nil, nil)
	if err == nil {
		t.Fatalf("expected destructive apply to be blocked")
	}
	if !strings.Contains(err.Error(), "destructive apply blocked") {
		t.Fatalf("unexpected error: %v", err)
	}
}
