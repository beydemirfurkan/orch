package cmd

import (
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/models"
)

func TestSummarizeRunStats(t *testing.T) {
	now := time.Now()
	states := []*models.RunState{
		{
			ID:           "run-3",
			Task:         models.Task{ID: "task-3", Description: "latest", CreatedAt: now},
			Status:       models.StatusFailed,
			StartedAt:    now,
			Review:       &models.ReviewResult{Decision: models.ReviewRevise},
			Confidence:   &models.ConfidenceReport{Score: 0.40, Band: "very_low"},
			Retries:      models.RetryState{Validation: 1, Testing: 1, Review: 0},
			TestFailures: []models.TestFailure{{Code: "test_assertion_failure", Summary: "boom"}},
		},
		{
			ID:         "run-2",
			Task:       models.Task{ID: "task-2", Description: "middle", CreatedAt: now.Add(-time.Hour)},
			Status:     models.StatusCompleted,
			StartedAt:  now.Add(-time.Hour),
			Review:     &models.ReviewResult{Decision: models.ReviewAccept},
			Confidence: &models.ConfidenceReport{Score: 0.80, Band: "high"},
			Retries:    models.RetryState{Validation: 0, Testing: 1, Review: 0},
		},
		{
			ID:        "run-1",
			Task:      models.Task{ID: "task-1", Description: "old", CreatedAt: now.Add(-2 * time.Hour)},
			Status:    models.StatusReviewing,
			StartedAt: now.Add(-2 * time.Hour),
			Review:    &models.ReviewResult{Decision: models.ReviewRevise},
			Retries:   models.RetryState{Validation: 0, Testing: 0, Review: 1},
		},
	}

	summary := summarizeRunStats(states)

	if summary.TotalRuns != 3 {
		t.Fatalf("expected 3 runs, got %d", summary.TotalRuns)
	}
	if summary.LatestRunID != "run-3" {
		t.Fatalf("expected latest run to be run-3, got %s", summary.LatestRunID)
	}
	if summary.CompletedRuns != 1 || summary.FailedRuns != 1 || summary.InProgressRuns != 1 {
		t.Fatalf("unexpected status counts: %+v", summary)
	}
	if summary.AcceptedReviews != 1 || summary.RevisedReviews != 2 {
		t.Fatalf("unexpected review counts: %+v", summary)
	}
	if summary.ConfidenceRunCount != 2 {
		t.Fatalf("expected 2 confidence-bearing runs, got %d", summary.ConfidenceRunCount)
	}
	if summary.AverageConfidence < 0.599 || summary.AverageConfidence > 0.601 {
		t.Fatalf("expected average confidence about 0.60, got %.4f", summary.AverageConfidence)
	}
	if summary.TotalRetryCount != 4 {
		t.Fatalf("expected total retries 4, got %d", summary.TotalRetryCount)
	}
	if summary.TestFailureCodeCounts["test_assertion_failure"] != 1 {
		t.Fatalf("expected test failure code count to be recorded")
	}
	if summary.ConfidenceBandCounts["high"] != 1 || summary.ConfidenceBandCounts["very_low"] != 1 {
		t.Fatalf("unexpected confidence band counts: %+v", summary.ConfidenceBandCounts)
	}
}
