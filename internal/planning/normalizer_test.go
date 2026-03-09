package planning

import (
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/models"
)

func TestNormalizeTaskBugfixClassification(t *testing.T) {
	task := &models.Task{
		ID:          "task-1",
		Description: "fix race condition in auth service",
		CreatedAt:   time.Now(),
	}

	brief := NormalizeTask(task)
	if brief == nil {
		t.Fatalf("expected task brief")
	}
	if brief.TaskType != models.TaskTypeBugfix {
		t.Fatalf("unexpected task type: %s", brief.TaskType)
	}
	if brief.RiskLevel != models.RiskHigh {
		t.Fatalf("unexpected risk level: %s", brief.RiskLevel)
	}
	if brief.NormalizedGoal == "" {
		t.Fatalf("expected normalized goal")
	}
}

func TestCompilePlanIncludesAcceptanceCriteriaAndTests(t *testing.T) {
	task := &models.Task{
		ID:          "task-2",
		Description: "add redis caching to user service",
		CreatedAt:   time.Now(),
	}
	repoMap := &models.RepoMap{
		TestFramework: "go test",
		Files: []models.FileInfo{
			{Path: "internal/user/service.go", Language: "go"},
			{Path: "internal/user/cache.go", Language: "go"},
			{Path: "internal/user/service_test.go", Language: "go"},
		},
	}

	brief := NormalizeTask(task)
	plan := CompilePlan(task, brief, repoMap)
	if plan == nil {
		t.Fatalf("expected plan")
	}
	if len(plan.AcceptanceCriteria) == 0 {
		t.Fatalf("expected acceptance criteria")
	}
	if len(plan.TestRequirements) == 0 {
		t.Fatalf("expected test requirements")
	}
	if len(plan.FilesToInspect) == 0 {
		t.Fatalf("expected files to inspect")
	}
	if len(plan.Steps) == 0 {
		t.Fatalf("expected plan steps")
	}
}
