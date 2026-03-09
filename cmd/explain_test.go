package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/furkanbeydemir/orch/internal/models"
)

func TestPrintExplainIncludesConfidenceAndReview(t *testing.T) {
	state := &models.RunState{
		ID:        "run-explain-1",
		Task:      models.Task{ID: "task-1", Description: "demo", CreatedAt: time.Now()},
		Status:    models.StatusCompleted,
		StartedAt: time.Now(),
		Review:    &models.ReviewResult{Decision: models.ReviewAccept, Comments: []string{"looks good"}},
		Confidence: &models.ConfidenceReport{
			Score:   0.91,
			Band:    "high",
			Reasons: []string{"tests and review aligned"},
		},
	}

	output := captureStdout(t, func() {
		printExplain(state)
	})

	if !strings.Contains(output, "RUN EXPLANATION") {
		t.Fatalf("expected explanation header, got: %s", output)
	}
	if !strings.Contains(output, "Score: 0.91") {
		t.Fatalf("expected confidence score in output, got: %s", output)
	}
	if !strings.Contains(output, "Decision: accept") {
		t.Fatalf("expected review decision in output, got: %s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = original }()

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	_ = r.Close()
	return buf.String()
}
