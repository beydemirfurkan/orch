package cmd

import (
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/models"
)

func TestBuildStagePartsFromRunState(t *testing.T) {
	state := &models.RunState{
		Logs: []models.LogEntry{
			{Timestamp: time.Now().UTC(), Actor: "planner", Step: "plan", Message: "Generating plan..."},
			{Timestamp: time.Now().UTC(), Actor: "coder", Step: "code", Message: "Generating patch..."},
		},
	}

	parts := buildStagePartsFromRunState(state)
	if len(parts) != 2 {
		t.Fatalf("expected 2 stage parts, got %d", len(parts))
	}
	for _, part := range parts {
		if part.Type != "stage" {
			t.Fatalf("expected stage part type, got %s", part.Type)
		}
		if part.Payload == "" {
			t.Fatalf("expected non-empty payload")
		}
	}
}
