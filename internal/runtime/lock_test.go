package runtime

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/config"
)

func TestLockAcquireAndRelease(t *testing.T) {
	repoRoot := t.TempDir()
	if err := config.EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}

	m := NewLockManager(repoRoot, time.Minute)
	release, err := m.Acquire("run-1")
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}

	if _, err := m.Acquire("run-2"); err == nil {
		t.Fatalf("expected second acquire to fail while lock is held")
	}

	if err := release(); err != nil {
		t.Fatalf("release lock: %v", err)
	}

	if _, err := m.Acquire("run-3"); err != nil {
		t.Fatalf("expected acquire to succeed after release: %v", err)
	}
}

func TestLockRecoversStaleLock(t *testing.T) {
	repoRoot := t.TempDir()
	if err := config.EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}

	lockPath := filepath.Join(repoRoot, config.OrchDir, lockFileName)
	stale := `{"pid":999999,"run_id":"old","created_at":"2000-01-01T00:00:00Z"}`
	if err := os.WriteFile(lockPath, []byte(stale), 0o644); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}

	m := NewLockManager(repoRoot, time.Second)
	release, err := m.Acquire("run-fresh")
	if err != nil {
		t.Fatalf("acquire should recover stale lock: %v", err)
	}
	if err := release(); err != nil {
		t.Fatalf("release lock: %v", err)
	}
}
