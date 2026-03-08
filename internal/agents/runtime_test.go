package agents

import (
	"context"
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/providers"
)

type providerStub struct {
	name string
	text string
}

func (p providerStub) Name() string { return p.name }

func (p providerStub) Validate(ctx context.Context) error { return nil }

func (p providerStub) Chat(ctx context.Context, req providers.ChatRequest) (providers.ChatResponse, error) {
	return providers.ChatResponse{Text: p.text, FinishReason: "completed"}, nil
}

func (p providerStub) Stream(ctx context.Context, req providers.ChatRequest) (<-chan providers.StreamEvent, <-chan error) {
	ev := make(chan providers.StreamEvent)
	err := make(chan error)
	close(ev)
	close(err)
	return ev, err
}

func TestPlannerUsesProviderRuntime(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := providers.NewRegistry()
	reg.Register(providerStub{name: "openai", text: "Plan from provider"})
	router := providers.NewRouter(cfg, reg)

	planner := NewPlanner("gpt-5.3-codex")
	planner.SetRuntime(&LLMRuntime{Router: router})

	output, err := planner.Execute(&Input{Task: &models.Task{ID: "t1", Description: "demo", CreatedAt: time.Now()}})
	if err != nil {
		t.Fatalf("planner execute: %v", err)
	}
	if output == nil || output.Plan == nil || len(output.Plan.Steps) == 0 {
		t.Fatalf("expected plan output")
	}
	if output.Plan.Steps[0].Description != "Plan from provider" {
		t.Fatalf("unexpected planner description: %q", output.Plan.Steps[0].Description)
	}
}

func TestReviewerParsesReviseDecision(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := providers.NewRegistry()
	reg.Register(providerStub{name: "openai", text: "revise: missing tests"})
	router := providers.NewRouter(cfg, reg)

	reviewer := NewReviewer("gpt-5.3-codex")
	reviewer.SetRuntime(&LLMRuntime{Router: router})

	output, err := reviewer.Execute(&Input{
		Task:  &models.Task{ID: "t1", Description: "demo", CreatedAt: time.Now()},
		Patch: &models.Patch{TaskID: "t1", RawDiff: ""},
	})
	if err != nil {
		t.Fatalf("reviewer execute: %v", err)
	}
	if output == nil || output.Review == nil {
		t.Fatalf("expected review output")
	}
	if output.Review.Decision != models.ReviewRevise {
		t.Fatalf("expected revise decision")
	}
}
