package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/furkanbeydemir/orch/internal/models"
)

type Logger struct {
	runID    string
	repoRoot string
	entries  []models.LogEntry
	mu       sync.Mutex
	verbose  bool
}

func New(runID, repoRoot string, verbose bool) *Logger {
	return &Logger{
		runID:    runID,
		repoRoot: repoRoot,
		entries:  make([]models.LogEntry, 0),
		verbose:  verbose,
	}
}

func (l *Logger) Log(actor, step, message string) {
	entry := models.LogEntry{
		Timestamp: time.Now(),
		Actor:     actor,
		Step:      step,
		Message:   message,
	}

	l.mu.Lock()
	l.entries = append(l.entries, entry)
	l.mu.Unlock()

	if l.verbose {
		fmt.Printf("[%s] %s\n", actor, message)
	}
}

func (l *Logger) Verbose(actor, step, message string) {
	if l.verbose {
		l.Log(actor, step, message)
	}
}

func (l *Logger) Entries() []models.LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	result := make([]models.LogEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

func (l *Logger) Save() error {
	l.mu.Lock()
	entries := make([]models.LogEntry, len(l.entries))
	copy(entries, l.entries)
	l.mu.Unlock()

	runsDir := filepath.Join(l.repoRoot, ".orch", "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create runs directory: %w", err)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize logs: %w", err)
	}

	logPath := filepath.Join(runsDir, l.runID+".json")
	if err := os.WriteFile(logPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write log file: %w", err)
	}

	return nil
}

func LoadRunLog(repoRoot, runID string) ([]models.LogEntry, error) {
	logPath := filepath.Join(repoRoot, ".orch", "runs", runID+".json")

	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	var entries []models.LogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse log file: %w", err)
	}

	return entries, nil
}

func ListRuns(repoRoot string) ([]string, error) {
	runsDir := filepath.Join(repoRoot, ".orch", "runs")

	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read runs directory: %w", err)
	}

	var runs []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			name := entry.Name()
			runs = append(runs, name[:len(name)-len(".json")])
		}
	}

	return runs, nil
}
