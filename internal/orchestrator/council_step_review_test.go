package orchestrator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/agents"
	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/providers"
)

func TestStepReviewWithCouncilProducesOutputs(t *testing.T) {
	repoRoot := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Provider.Flags.OpenAIEnabled = false
	cfg.Safety.FeatureFlags.CouncilEnabled = true
	cfg.Council.Enabled = true
	cfg.Council.SynthesisMode = "majority"
	cfg.Council.Members = []config.CouncilMemberConfig{
		{Model: "stub:model-a", Weight: 1},
		{Model: "stub:model-b", Weight: 1},
	}

	orch := New(cfg, repoRoot, false)
	orch.reviewer = agentStub{name: "reviewer", execute: func(input *agents.Input) (*agents.Output, error) {
		t.Fatalf("single reviewer fallback should not execute. prompt=%s", input.SkillHints)
		return nil, nil
	}}
	orch.providerRegistry = providers.NewRegistry()
	orch.providerRegistry.Register(councilProviderStub{
		name: "stub",
		responses: map[string]providers.ChatResponse{
			"model-a": {
				Text:  "ACCEPT: patch looks good and addresses all requirements",
				Usage: providers.Usage{InputTokens: 90, OutputTokens: 10, TotalTokens: 100},
			},
			"model-b": {
				Text:  "ACCEPT\nTests cover the change. Continue.",
				Usage: providers.Usage{InputTokens: 80, OutputTokens: 8, TotalTokens: 88},
			},
		},
	})
	orch.providerReady = true

	state := &models.RunState{
		Status: models.StatusTesting,
		Task: models.Task{
			ID:          "task-council",
			Description: "add feature",
			CreatedAt:   time.Now(),
		},
		TaskBrief: &models.TaskBrief{
			TaskID:    "task-council",
			TaskType:  models.TaskTypeFeature,
			RiskLevel: models.RiskHigh,
		},
		Plan: &models.Plan{
			TaskID:             "task-council",
			Summary:            "implement feature",
			TaskType:           models.TaskTypeFeature,
			FilesToModify:      []string{"service.go"},
			FilesToInspect:     []string{"service.go"},
			AcceptanceCriteria: []models.AcceptanceCriterion{{ID: "ac-1", Description: "feature works"}},
			TestRequirements:   []string{"go test ./..."},
			Steps:              []models.PlanStep{{Order: 1, Description: "update service"}},
		},
		ExecutionContract: &models.ExecutionContract{
			TaskID:        "task-council",
			AllowedFiles:  []string{"service.go"},
			RequiredEdits: []string{"add feature"},
		},
		Patch: &models.Patch{
			TaskID:  "task-council",
			Files:   []models.PatchFile{{Path: "service.go", Status: "modified", Diff: "-old\n+new"}},
			RawDiff: "diff --git a/service.go b/service.go\n--- a/service.go\n+++ b/service.go\n@@\n-old\n+new\n",
		},
		ValidationResults: []models.ValidationResult{
			{Name: "plan_compliance", Stage: "validation", Status: models.ValidationPass, Severity: models.SeverityLow, Summary: "plan respected"},
			{Name: "scope_compliance", Stage: "validation", Status: models.ValidationPass, Severity: models.SeverityLow, Summary: "scope ok"},
			{Name: "patch_hygiene", Stage: "validation", Status: models.ValidationPass, Severity: models.SeverityLow, Summary: "clean"},
			{Name: "required_tests_passed", Stage: "test", Status: models.ValidationPass, Severity: models.SeverityLow, Summary: "tests passed"},
		},
		TestResults: "ok",
		StartedAt:   time.Now(),
		Logs:        []models.LogEntry{},
	}

	if err := orch.stepReview(state); err != nil {
		t.Fatalf("stepReview error: %v", err)
	}

	if state.Status != models.StatusReviewing {
		t.Fatalf("status=%s want=%s", state.Status, models.StatusReviewing)
	}
	if state.ReviewScorecard == nil {
		t.Fatalf("expected review scorecard")
	}
	if state.Review == nil {
		t.Fatalf("expected final review result")
	}
	if state.Confidence == nil {
		t.Fatalf("expected confidence report")
	}
	if !hasReviewGate(state.ValidationResults, "review_scorecard_valid") {
		t.Fatalf("missing review_scorecard_valid gate")
	}
	if !hasReviewGate(state.ValidationResults, "review_decision_threshold_met") {
		t.Fatalf("missing review_decision_threshold_met gate")
	}
	if len(state.TokenUsages) != len(cfg.Council.Members) {
		t.Fatalf("token usages not recorded for council votes: got=%d want=%d", len(state.TokenUsages), len(cfg.Council.Members))
	}
	for _, usage := range state.TokenUsages {
		if usage.Stage != "reviewing" {
			t.Fatalf("unexpected token usage stage: %s", usage.Stage)
		}
		if usage.Role != string(providers.RoleReviewer) {
			t.Fatalf("unexpected token usage role: %s", usage.Role)
		}
	}
	comments := strings.Join(state.Review.Comments, " ")
	if !strings.Contains(comments, "Council verdict") {
		t.Fatalf("council review comments missing council verdict marker: %v", state.Review.Comments)
	}
}

func TestStepReviewWithSplitCouncilDowngradesToRevise(t *testing.T) {
	repoRoot := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Provider.Flags.OpenAIEnabled = false
	cfg.Safety.FeatureFlags.CouncilEnabled = true
	cfg.Council.Enabled = true
	cfg.Council.SynthesisMode = "majority"
	cfg.Council.Members = []config.CouncilMemberConfig{
		{Model: "stub:model-a", Weight: 1},
		{Model: "stub:model-b", Weight: 1},
		{Model: "stub:model-c", Weight: 1},
	}

	orch := New(cfg, repoRoot, false)
	orch.providerRegistry = providers.NewRegistry()
	orch.providerRegistry.Register(councilProviderStub{
		name: "stub",
		responses: map[string]providers.ChatResponse{
			"model-a": {Text: "ACCEPT\nLooks fine", Usage: providers.Usage{InputTokens: 50, OutputTokens: 10, TotalTokens: 60}},
			"model-b": {Text: "ACCEPT\nOkay", Usage: providers.Usage{InputTokens: 50, OutputTokens: 10, TotalTokens: 60}},
			"model-c": {Text: "REVISE\nNeed more caution", Usage: providers.Usage{InputTokens: 50, OutputTokens: 10, TotalTokens: 60}},
		},
	})
	orch.providerReady = true

	state := councilReviewState(time.Now())
	if err := orch.stepReview(state); err != nil {
		t.Fatalf("stepReview error: %v", err)
	}
	if state.Review == nil {
		t.Fatalf("expected final review result")
	}
	if state.Review.Decision != models.ReviewRevise {
		t.Fatalf("decision=%s want=%s", state.Review.Decision, models.ReviewRevise)
	}
	if !strings.Contains(strings.Join(state.Review.Comments, " "), "did not reach consensus") {
		t.Fatalf("expected non-consensus downgrade comment, got=%v", state.Review.Comments)
	}
}

func councilReviewState(now time.Time) *models.RunState {
	return &models.RunState{
		Status: models.StatusTesting,
		Task: models.Task{
			ID:          "task-council",
			Description: "add feature",
			CreatedAt:   now,
		},
		TaskBrief: &models.TaskBrief{
			TaskID:    "task-council",
			TaskType:  models.TaskTypeFeature,
			RiskLevel: models.RiskHigh,
		},
		Plan: &models.Plan{
			TaskID:             "task-council",
			Summary:            "implement feature",
			TaskType:           models.TaskTypeFeature,
			FilesToModify:      []string{"service.go"},
			FilesToInspect:     []string{"service.go"},
			AcceptanceCriteria: []models.AcceptanceCriterion{{ID: "ac-1", Description: "feature works"}},
			TestRequirements:   []string{"go test ./..."},
			Steps:              []models.PlanStep{{Order: 1, Description: "update service"}},
		},
		ExecutionContract: &models.ExecutionContract{
			TaskID:        "task-council",
			AllowedFiles:  []string{"service.go"},
			RequiredEdits: []string{"add feature"},
		},
		Patch: &models.Patch{
			TaskID:  "task-council",
			Files:   []models.PatchFile{{Path: "service.go", Status: "modified", Diff: "-old\n+new"}},
			RawDiff: "diff --git a/service.go b/service.go\n--- a/service.go\n+++ b/service.go\n@@\n-old\n+new\n",
		},
		ValidationResults: []models.ValidationResult{
			{Name: "plan_compliance", Stage: "validation", Status: models.ValidationPass, Severity: models.SeverityLow, Summary: "plan respected"},
			{Name: "scope_compliance", Stage: "validation", Status: models.ValidationPass, Severity: models.SeverityLow, Summary: "scope ok"},
			{Name: "patch_hygiene", Stage: "validation", Status: models.ValidationPass, Severity: models.SeverityLow, Summary: "clean"},
			{Name: "required_tests_passed", Stage: "test", Status: models.ValidationPass, Severity: models.SeverityLow, Summary: "tests passed"},
		},
		TestResults: "ok",
		StartedAt:   now,
		Logs:        []models.LogEntry{},
	}
}

func hasReviewGate(results []models.ValidationResult, name string) bool {
	for _, result := range results {
		if result.Name == name && strings.EqualFold(result.Stage, "review") {
			return true
		}
	}
	return false
}

type councilProviderStub struct {
	name      string
	responses map[string]providers.ChatResponse
}

func (p councilProviderStub) Name() string { return p.name }

func (p councilProviderStub) Validate(ctx context.Context) error { return nil }

func (p councilProviderStub) Chat(ctx context.Context, req providers.ChatRequest) (providers.ChatResponse, error) {
	if resp, ok := p.responses[req.Model]; ok {
		return resp, nil
	}
	return providers.ChatResponse{Text: "ACCEPT", Usage: providers.Usage{TotalTokens: 1}}, nil
}

func (p councilProviderStub) Stream(ctx context.Context, req providers.ChatRequest) (<-chan providers.StreamEvent, <-chan error) {
	events := make(chan providers.StreamEvent)
	errs := make(chan error)
	close(events)
	close(errs)
	return events, errs
}
