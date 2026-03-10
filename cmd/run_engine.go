package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/orchestrator"
	runlock "github.com/furkanbeydemir/orch/internal/runtime"
	"github.com/furkanbeydemir/orch/internal/session"
	"github.com/furkanbeydemir/orch/internal/storage"
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

	svc := session.NewService(sessionCtx.Store)
	if compacted, note, compactErr := svc.MaybeCompact(sessionCtx.Session.ID, cfg.Provider.OpenAI.Models.Coder); compactErr == nil && compacted {
		result.Warnings = append(result.Warnings, note)
	}
	userMsg, sessionErr := svc.AppendText(session.MessageInput{
		SessionID:  sessionCtx.Session.ID,
		Role:       "user",
		ProviderID: "orch",
		ModelID:    "run-engine",
		Text:       taskDescription,
	})
	if sessionErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to persist run user message: %s", sessionErr))
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
		if saveErr := sessionCtx.Store.SaveRunState(state); saveErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to save SQLite run state: %s", saveErr))
		}
	}

	runSummary := summarizeRunForSession(result)
	stagePayload := map[string]any{
		"run_id":   "",
		"status":   "failed",
		"warnings": result.Warnings,
	}
	finishReason := "error"
	errorText := ""
	if state != nil {
		stagePayload["run_id"] = state.ID
		stagePayload["status"] = string(state.Status)
		if state.Status == models.StatusCompleted && result.Err == nil {
			finishReason = "stop"
		}
	}
	if result.Err != nil {
		errorText = result.Err.Error()
	}
	payloadBytes, _ := json.Marshal(stagePayload)
	parentID := ""
	if userMsg != nil {
		parentID = userMsg.Message.ID
	}
	assistantParts := []storage.SessionPart{{Type: "stage", Payload: string(payloadBytes)}}
	assistantParts = append(assistantParts, buildStagePartsFromRunState(state)...)
	if _, appendErr := svc.AppendMessage(session.MessageInput{
		SessionID:    sessionCtx.Session.ID,
		Role:         "assistant",
		ParentID:     parentID,
		ProviderID:   "orch",
		ModelID:      "run-engine",
		FinishReason: finishReason,
		Error:        errorText,
		Text:         runSummary,
	}, assistantParts); appendErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to persist run assistant message: %s", appendErr))
	}

	if compacted, note, compactErr := svc.MaybeCompact(sessionCtx.Session.ID, cfg.Provider.OpenAI.Models.Coder); compactErr == nil && compacted {
		result.Warnings = append(result.Warnings, note)
	}

	return result, nil
}

func buildStagePartsFromRunState(state *models.RunState) []storage.SessionPart {
	if state == nil || len(state.Logs) == 0 {
		return []storage.SessionPart{}
	}

	parts := make([]storage.SessionPart, 0, len(state.Logs))
	for _, entry := range state.Logs {
		payload := map[string]any{
			"actor":     strings.TrimSpace(entry.Actor),
			"step":      strings.TrimSpace(entry.Step),
			"message":   strings.TrimSpace(entry.Message),
			"timestamp": entry.Timestamp.UTC().Format(time.RFC3339Nano),
		}
		body, err := json.Marshal(payload)
		if err != nil {
			continue
		}
		parts = append(parts, storage.SessionPart{Type: "stage", Payload: string(body)})
	}

	return parts
}

func summarizeRunForSession(result *runExecutionResult) string {
	if result == nil || result.State == nil {
		if result != nil && result.Err != nil {
			return fmt.Sprintf("Run failed: %s", result.Err.Error())
		}
		return "Run finished without state output"
	}

	state := result.State
	if result.Err != nil || state.Status == models.StatusFailed {
		errText := strings.TrimSpace(state.Error)
		if errText == "" && result.Err != nil {
			errText = result.Err.Error()
		}
		if errText == "" {
			errText = "unknown failure"
		}
		return fmt.Sprintf("Run failed at status=%s: %s", state.Status, errText)
	}

	if state.Patch == nil || len(state.Patch.Files) == 0 {
		return fmt.Sprintf("Run completed with status=%s and no patch files.", state.Status)
	}

	return fmt.Sprintf("Run completed with status=%s and %d patch file(s).", state.Status, len(state.Patch.Files))
}

func getWorkingDirectory() (string, error) {
	return osGetwd()
}

var osGetwd = func() (string, error) {
	return os.Getwd()
}
