package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/runstore"
	"github.com/spf13/cobra"
)

var (
	historyLimit      int
	historyStatusFilter string
	historyJSON       bool
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "List recent runs in a table",
	Long:  `Shows recent runs with their task, status, confidence score, retry count, and start time. Use --status to filter by outcome.`,
	RunE:  runHistory,
}

func init() {
	rootCmd.AddCommand(historyCmd)
	historyCmd.Flags().IntVar(&historyLimit, "limit", 20, "Maximum number of runs to show")
	historyCmd.Flags().StringVar(&historyStatusFilter, "status", "", "Filter by status (completed, failed, ...)")
	historyCmd.Flags().BoolVar(&historyJSON, "json", false, "Output as JSON")
}

func runHistory(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	states, err := runstore.ListRunStates(cwd, 0)
	if err != nil {
		return fmt.Errorf("failed to load run states: %w", err)
	}

	if len(states) == 0 {
		fmt.Println("No runs found yet. Run 'orch run <task>' first.")
		return nil
	}

	filter := strings.ToLower(strings.TrimSpace(historyStatusFilter))
	if filter != "" {
		filtered := states[:0]
		for _, s := range states {
			if strings.ToLower(string(s.Status)) == filter {
				filtered = append(filtered, s)
			}
		}
		states = filtered
	}

	if historyLimit > 0 && len(states) > historyLimit {
		states = states[:historyLimit]
	}

	if historyJSON {
		return printHistoryJSON(states)
	}

	printHistoryTable(states)
	return nil
}

type historyRow struct {
	RunID   string `json:"run_id"`
	Task    string `json:"task"`
	Status  string `json:"status"`
	Conf    string `json:"confidence"`
	Retries int    `json:"retries"`
	Started string `json:"started"`
}

func printHistoryTable(states []*models.RunState) {
	fmt.Printf("%-18s  %-42s  %-11s  %-6s  %-7s  %s\n",
		"RUN ID", "TASK", "STATUS", "CONF", "RETRIES", "STARTED")
	fmt.Println(strings.Repeat("─", 100))

	for _, s := range states {
		if s == nil {
			continue
		}

		runID := s.ID
		if len(runID) > 16 {
			runID = runID[:16]
		}

		task := s.Task.Description
		if len(task) > 42 {
			task = task[:39] + "..."
		}

		status := string(s.Status)

		conf := "—"
		if s.Confidence != nil {
			conf = fmt.Sprintf("%.2f", s.Confidence.Score)
		}

		retries := s.Retries.Validation + s.Retries.Testing + s.Retries.Review

		started := humanDuration(time.Since(s.StartedAt))

		fmt.Printf("%-18s  %-42s  %-11s  %-6s  %-7d  %s\n",
			runID, task, status, conf, retries, started)
	}

	fmt.Printf("\n%d run(s) shown\n", len(states))
}

func printHistoryJSON(states []*models.RunState) error {
	rows := make([]historyRow, 0, len(states))
	for _, s := range states {
		if s == nil {
			continue
		}
		conf := "—"
		if s.Confidence != nil {
			conf = fmt.Sprintf("%.2f", s.Confidence.Score)
		}
		rows = append(rows, historyRow{
			RunID:   s.ID,
			Task:    s.Task.Description,
			Status:  string(s.Status),
			Conf:    conf,
			Retries: s.Retries.Validation + s.Retries.Testing + s.Retries.Review,
			Started: s.StartedAt.Format(time.RFC3339),
		})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

func humanDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1d ago"
	}
	return fmt.Sprintf("%dd ago", days)
}
