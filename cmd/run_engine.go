package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/orchestrator"
	"github.com/furkanbeydemir/orch/internal/runstore"
	runlock "github.com/furkanbeydemir/orch/internal/runtime"
)

type runExecutionResult struct {
	Task        *models.Task
	State       *models.RunState
	Err         error
	ProjectID   string
	SessionName string
	Worktree    string
	CWD         string
	ExecRoot    string
	Warnings    []string
}

func executeRunTask(taskDescription string) (*runExecutionResult, error) {
	cwd, err := getWorkingDirectory()
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	sessionCtx, err := loadSessionContext(cwd)
	if err != nil {
		return nil, err
	}
	defer sessionCtx.Store.Close()

	execRoot := sessionCtx.ExecutionRoot(cwd)

	task := &models.Task{
		ID:          fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Description: taskDescription,
		CreatedAt:   time.Now(),
	}

	result := &runExecutionResult{
		Task:        task,
		ProjectID:   sessionCtx.ProjectID,
		SessionName: sessionCtx.Session.Name,
		Worktree:    sessionCtx.Session.Worktree,
		CWD:         cwd,
		ExecRoot:    execRoot,
		Warnings:    make([]string, 0),
	}

	var unlock func() error
	if cfg.Safety.FeatureFlags.RepoLock {
		lockManager := runlock.NewLockManager(execRoot, time.Duration(cfg.Safety.LockStaleAfterSeconds)*time.Second)
		unlock, err = lockManager.Acquire(task.ID)
		if err != nil {
			return nil, fmt.Errorf("run blocked by repository lock: %w", err)
		}
		defer func() {
			if unlockErr := unlock(); unlockErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("failed to release lock: %s", unlockErr))
			}
		}()
	}

	orch := orchestrator.New(cfg, execRoot, verbose)
	state, runErr := orch.Run(task)
	result.State = state
	result.Err = runErr

	if state != nil {
		state.ProjectID = sessionCtx.ProjectID
		state.SessionID = sessionCtx.Session.ID
		if saveErr := runstore.SaveRunState(cwd, state); saveErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to save run state: %s", saveErr))
		}
		if saveErr := sessionCtx.Store.SaveRunState(state); saveErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to save SQLite run state: %s", saveErr))
		}
	}

	return result, nil
}

func getWorkingDirectory() (string, error) {
	return osGetwd()
}

var osGetwd = func() (string, error) {
	return os.Getwd()
}
