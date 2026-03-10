package session

import (
	"testing"

	"github.com/furkanbeydemir/orch/internal/storage"
)

func TestEstimateTokensModelAware(t *testing.T) {
	messages := []MessageWithParts{{
		Message: storage.SessionMessage{Role: "assistant"},
		Parts: []storage.SessionPart{
			{Type: "text", Payload: `{"text":"hello world"}`},
			{Type: "stage", Payload: `{"actor":"planner","step":"plan","message":"a"}`},
		},
	}}

	gpt5 := EstimateTokens(messages, "gpt-5.3-codex")
	defaultModel := EstimateTokens(messages, "unknown-model")
	if gpt5 <= 0 || defaultModel <= 0 {
		t.Fatalf("expected positive token estimates, got gpt5=%d default=%d", gpt5, defaultModel)
	}
	if gpt5 == defaultModel {
		t.Fatalf("expected model-aware token difference, both=%d", gpt5)
	}
}

func TestResolveBudgetSafetyMargin(t *testing.T) {
	b := ResolveBudget("gpt-5.3-codex")
	if b.UsableInput() <= 0 {
		t.Fatalf("expected usable input to be positive")
	}
	if b.SafetyMargin <= 0 || b.SafetyMargin >= 1 {
		t.Fatalf("expected safety margin to be between 0 and 1, got %f", b.SafetyMargin)
	}
}
