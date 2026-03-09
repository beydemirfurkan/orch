package runstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
)

const (
	latestRunFile = "latest-run-id"
	latestPatch   = "latest.patch"
)

func SaveRunState(repoRoot string, state *models.RunState) error {
	if state == nil {
		return fmt.Errorf("run state cannot be nil")
	}

	if err := config.EnsureOrchDir(repoRoot); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run state: %w", err)
	}

	runsDir := filepath.Join(repoRoot, config.OrchDir, config.RunsDir)
	statePath := filepath.Join(runsDir, state.ID+".state")
	if err := os.WriteFile(statePath, data, 0o644); err != nil {
		return fmt.Errorf("write run state: %w", err)
	}

	latestRunPath := filepath.Join(repoRoot, config.OrchDir, latestRunFile)
	if err := os.WriteFile(latestRunPath, []byte(state.ID), 0o644); err != nil {
		return fmt.Errorf("write latest run id: %w", err)
	}

	patchPath := filepath.Join(repoRoot, config.OrchDir, latestPatch)
	if state.Patch != nil && strings.TrimSpace(state.Patch.RawDiff) != "" {
		if err := os.WriteFile(patchPath, []byte(state.Patch.RawDiff), 0o644); err != nil {
			return fmt.Errorf("write latest patch: %w", err)
		}
	} else {
		if err := os.Remove(patchPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale latest patch: %w", err)
		}
	}

	return nil
}

func LoadLatestRunState(repoRoot string) (*models.RunState, error) {
	latestRunPath := filepath.Join(repoRoot, config.OrchDir, latestRunFile)
	runIDBytes, err := os.ReadFile(latestRunPath)
	if err != nil {
		return nil, fmt.Errorf("read latest run id: %w", err)
	}

	runID := strings.TrimSpace(string(runIDBytes))
	if runID == "" {
		return nil, fmt.Errorf("latest run id is empty")
	}

	return LoadRunState(repoRoot, runID)
}

func LoadRunState(repoRoot, runID string) (*models.RunState, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("run id is required")
	}

	statePath := filepath.Join(repoRoot, config.OrchDir, config.RunsDir, runID+".state")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("read run state: %w", err)
	}

	var state models.RunState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal run state: %w", err)
	}

	return &state, nil
}

func ListRunStates(repoRoot string, limit int) ([]*models.RunState, error) {
	if err := config.EnsureOrchDir(repoRoot); err != nil {
		return nil, err
	}

	runsDir := filepath.Join(repoRoot, config.OrchDir, config.RunsDir)
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil, fmt.Errorf("read runs dir: %w", err)
	}

	states := make([]*models.RunState, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".state") {
			continue
		}
		runID := strings.TrimSuffix(entry.Name(), ".state")
		state, err := LoadRunState(repoRoot, runID)
		if err != nil {
			return nil, fmt.Errorf("load run %s: %w", runID, err)
		}
		states = append(states, state)
	}

	sort.SliceStable(states, func(i, j int) bool {
		return states[i].StartedAt.After(states[j].StartedAt)
	})

	if limit > 0 && len(states) > limit {
		states = states[:limit]
	}

	return states, nil
}

func LoadLatestPatch(repoRoot string) (string, error) {
	patchPath := filepath.Join(repoRoot, config.OrchDir, latestPatch)
	data, err := os.ReadFile(patchPath)
	if err != nil {
		return "", fmt.Errorf("read latest patch: %w", err)
	}

	return string(data), nil
}
