package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/furkanbeydemir/orch/internal/config"
)

const lockFileName = "lock"

type LockManager struct {
	repoRoot   string
	lockPath   string
	staleAfter time.Duration
}

type LockFile struct {
	PID       int       `json:"pid"`
	RunID     string    `json:"run_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func NewLockManager(repoRoot string, staleAfter time.Duration) *LockManager {
	if staleAfter <= 0 {
		staleAfter = time.Hour
	}
	return &LockManager{
		repoRoot:   repoRoot,
		lockPath:   filepath.Join(repoRoot, config.OrchDir, lockFileName),
		staleAfter: staleAfter,
	}
}

func (m *LockManager) Acquire(runID string) (func() error, error) {
	if err := config.EnsureOrchDir(m.repoRoot); err != nil {
		return nil, err
	}

	for attempt := 0; attempt < 2; attempt++ {
		owner := LockFile{
			PID:       os.Getpid(),
			RunID:     runID,
			CreatedAt: time.Now().UTC(),
		}
		if err := m.create(owner); err == nil {
			return func() error {
				return m.Release(owner)
			}, nil
		} else if !os.IsExist(err) {
			return nil, err
		}

		existing, readErr := m.readLock()
		if readErr != nil {
			if removeErr := os.Remove(m.lockPath); removeErr != nil && !os.IsNotExist(removeErr) {
				return nil, fmt.Errorf("lock exists and is unreadable: %w", readErr)
			}
			continue
		}

		if !m.isStale(existing) {
			return nil, fmt.Errorf("repository locked by pid=%d run=%s", existing.PID, existing.RunID)
		}

		if err := os.Remove(m.lockPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to remove stale lock: %w", err)
		}
	}

	return nil, fmt.Errorf("failed to acquire repository lock")
}

func (m *LockManager) Release(owner LockFile) error {
	existing, err := m.readLock()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if existing.PID != owner.PID || !existing.CreatedAt.Equal(owner.CreatedAt) {
		return nil
	}

	if err := os.Remove(m.lockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	return nil
}

func (m *LockManager) create(lock LockFile) error {
	f, err := os.OpenFile(m.lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	if err := enc.Encode(lock); err != nil {
		return fmt.Errorf("failed to encode lock file: %w", err)
	}

	return nil
}

func (m *LockManager) readLock() (LockFile, error) {
	var lock LockFile
	data, err := os.ReadFile(m.lockPath)
	if err != nil {
		return lock, err
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return lock, err
	}
	return lock, nil
}

func (m *LockManager) isStale(lock LockFile) bool {
	if lock.PID <= 0 {
		return true
	}

	age := time.Since(lock.CreatedAt)
	if age > m.staleAfter {
		return true
	}

	return !processAlive(lock.PID)
}

func processAlive(pid int) bool {
	if pid == os.Getpid() {
		return true
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}
