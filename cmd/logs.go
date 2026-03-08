// Package cmd implements the logs command.
//
//	[analyzer] scanning repository
//	[planner] generating plan
//	[coder] editing userService.ts
//	[test] running npm test
//	[reviewer] patch approved
package cmd

import (
	"fmt"
	"os"

	"github.com/furkanbeydemir/orch/internal/logger"
	"github.com/spf13/cobra"
)

// logsCmd represents the `orch logs` command.
var logsCmd = &cobra.Command{
	Use:   "logs [run-id]",
	Short: "Shows execution trace",
	Long:  `Lists agent execution steps in chronological order.`,
	RunE:  runLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	if len(args) > 0 {
		entries, err := logger.LoadRunLog(cwd, args[0])
		if err != nil {
			return fmt.Errorf("failed to load logs: %w", err)
		}

		for _, entry := range entries {
			fmt.Printf("[%s] %s | %s\n",
				entry.Actor,
				entry.Timestamp.Format("15:04:05"),
				entry.Message,
			)
		}
		return nil
	}

	runs, err := logger.ListRuns(cwd)
	if err != nil {
		fmt.Println("📋 No run records found yet.")
		fmt.Println("   'orch run <task>' to run a task.")
		return nil
	}

	if len(runs) == 0 {
		fmt.Println("📋 No run records found yet.")
		return nil
	}

	fmt.Println("📋 Run Records:")
	fmt.Println("─────────────────────────────────────")
	for _, run := range runs {
		fmt.Printf("  • %s\n", run)
	}
	fmt.Println("\nFor details: orch logs <run-id>")

	return nil
}
