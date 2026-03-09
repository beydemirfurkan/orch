package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/runstore"
	"github.com/spf13/cobra"
)

var statsLimit int

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show run quality statistics",
	Long:  `Summarizes recent runs using structured Orch artifacts such as status, review, confidence, retries, and classified test failures.`,
	RunE:  runStats,
}

func init() {
	rootCmd.AddCommand(statsCmd)
	statsCmd.Flags().IntVar(&statsLimit, "limit", 50, "Maximum number of recent runs to include")
}

func runStats(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	states, err := runstore.ListRunStates(cwd, statsLimit)
	if err != nil {
		return fmt.Errorf("failed to load run states: %w", err)
	}
	if len(states) == 0 {
		fmt.Println("📊 No runs found yet.")
		fmt.Println("   Run 'orch run <task>' first.")
		return nil
	}

	summary := summarizeRunStats(states)
	printRunStats(summary)
	return nil
}

type runStatsSummary struct {
	TotalRuns             int
	CompletedRuns         int
	FailedRuns            int
	InProgressRuns        int
	AcceptedReviews       int
	RevisedReviews        int
	AverageConfidence     float64
	ConfidenceRunCount    int
	AverageRetries        float64
	TotalRetryCount       int
	StatusCounts          map[string]int
	ConfidenceBandCounts  map[string]int
	TestFailureCodeCounts map[string]int
	LatestRunID           string
	LatestRunStatus       string
}

func summarizeRunStats(states []*models.RunState) runStatsSummary {
	summary := runStatsSummary{
		TotalRuns:             len(states),
		StatusCounts:          map[string]int{},
		ConfidenceBandCounts:  map[string]int{},
		TestFailureCodeCounts: map[string]int{},
	}

	var confidenceTotal float64
	for i, state := range states {
		if state == nil {
			continue
		}
		if i == 0 {
			summary.LatestRunID = state.ID
			summary.LatestRunStatus = string(state.Status)
		}

		summary.StatusCounts[string(state.Status)]++
		switch state.Status {
		case models.StatusCompleted:
			summary.CompletedRuns++
		case models.StatusFailed:
			summary.FailedRuns++
		default:
			summary.InProgressRuns++
		}

		if state.Review != nil {
			switch state.Review.Decision {
			case models.ReviewAccept:
				summary.AcceptedReviews++
			case models.ReviewRevise:
				summary.RevisedReviews++
			}
		}

		if state.Confidence != nil {
			confidenceTotal += state.Confidence.Score
			summary.ConfidenceRunCount++
			if strings.TrimSpace(state.Confidence.Band) != "" {
				summary.ConfidenceBandCounts[state.Confidence.Band]++
			}
		}

		retries := state.Retries.Validation + state.Retries.Testing + state.Retries.Review
		summary.TotalRetryCount += retries

		for _, failure := range state.TestFailures {
			code := strings.TrimSpace(failure.Code)
			if code == "" {
				code = "unknown"
			}
			summary.TestFailureCodeCounts[code]++
		}
	}

	if summary.ConfidenceRunCount > 0 {
		summary.AverageConfidence = confidenceTotal / float64(summary.ConfidenceRunCount)
	}
	if summary.TotalRuns > 0 {
		summary.AverageRetries = float64(summary.TotalRetryCount) / float64(summary.TotalRuns)
	}

	return summary
}

func printRunStats(summary runStatsSummary) {
	fmt.Println("═══════════════════════════════════════")
	fmt.Println("📊 ORCH RUN STATS")
	fmt.Println("═══════════════════════════════════════")
	fmt.Printf("Runs Analyzed: %d\n", summary.TotalRuns)
	fmt.Printf("Latest Run: %s (%s)\n", summary.LatestRunID, summary.LatestRunStatus)
	fmt.Printf("Completed: %d\n", summary.CompletedRuns)
	fmt.Printf("Failed: %d\n", summary.FailedRuns)
	fmt.Printf("In Progress/Other: %d\n", summary.InProgressRuns)
	fmt.Printf("Review Accept: %d\n", summary.AcceptedReviews)
	fmt.Printf("Review Revise: %d\n", summary.RevisedReviews)
	fmt.Printf("Average Retries: %.2f\n", summary.AverageRetries)
	if summary.ConfidenceRunCount > 0 {
		fmt.Printf("Average Confidence: %.2f across %d run(s)\n", summary.AverageConfidence, summary.ConfidenceRunCount)
	}

	printCountMap("Status Breakdown", summary.StatusCounts)
	printCountMap("Confidence Bands", summary.ConfidenceBandCounts)
	printCountMap("Test Failure Codes", summary.TestFailureCodeCounts)
}

func printCountMap(title string, counts map[string]int) {
	if len(counts) == 0 {
		return
	}
	fmt.Printf("\n%s\n", title)
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Printf("  - %s: %d\n", key, counts[key])
	}
}
