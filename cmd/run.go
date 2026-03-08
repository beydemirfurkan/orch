// Package cmd implements the run command.
//
// Pipeline:
//
//	Task → Repo Analysis → Context Selection → Planner → Coder
//	→ Patch Validation → Test Runner → Reviewer → Result
package cmd

import (
	"fmt"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/spf13/cobra"
)

// runCmd represents the `orch run` command.
var runCmd = &cobra.Command{
	Use:   "run [task]",
	Short: "Runs full pipeline",
	Long: `Runs the full pipeline for the given task:
analyze -> plan -> code -> validate -> test -> review.`,
	Args: cobra.ExactArgs(1),
	RunE: runRun,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
	result, err := executeRunTask(args[0])
	if err != nil {
		return err
	}

	for _, warning := range result.Warnings {
		fmt.Printf("\n⚠ %s\n", warning)
	}

	fmt.Printf("🚀 Starting pipeline: %s\n\n", result.Task.Description)
	printRunContextSummary(result.ProjectID, result.SessionName, result.Worktree, result.CWD, result.ExecRoot)

	state := result.State
	err = result.Err

	if err != nil {
		fmt.Printf("\n❌ Pipeline failed: %s\n", err.Error())
		if state != nil {
			printRunResultSummary(state)
			if len(state.UnresolvedFailures) > 0 {
				fmt.Println("\n⚠ Unresolved Failures:")
				for _, failure := range state.UnresolvedFailures {
					fmt.Printf("- %s\n", failure)
				}
			}
			if strings.TrimSpace(state.BestPatchSummary) != "" {
				fmt.Printf("\n🩹 Best Patch Summary: %s\n", state.BestPatchSummary)
			}
			if strings.TrimSpace(state.TestResults) != "" {
				fmt.Println("\n🧪 Test Summary:")
				fmt.Println(state.TestResults)
			}
		}
		return err
	}

	fmt.Println("\n═══════════════════════════════════════")
	fmt.Printf("✅ Pipeline completed: %s\n", state.Status)
	fmt.Println("═══════════════════════════════════════")

	if state.Review != nil {
		fmt.Printf("\n📋 Review Decision: %s\n", state.Review.Decision)
		for _, comment := range state.Review.Comments {
			fmt.Printf("   💬 %s\n", comment)
		}
	}

	if state.Patch != nil && state.Patch.RawDiff != "" {
		fmt.Println("\n💡 To view the patch: orch diff")
		fmt.Println("💡 To apply the patch: orch apply")
	}

	printRunResultSummary(state)

	return nil
}

func printRunContextSummary(projectID, sessionName, worktree, cwd, execRoot string) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Run Context")
	fmt.Printf("Project: %s\n", projectID)
	fmt.Printf("Session: %s\n", sessionName)
	if worktree != "" {
		fmt.Printf("Worktree: %s\n", worktree)
	}
	if execRoot != cwd {
		fmt.Printf("Execution root: %s\n", execRoot)
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

func printRunResultSummary(state *models.RunState) {
	if state == nil {
		return
	}
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Run Summary")
	fmt.Printf("Run ID: %s\n", state.ID)
	fmt.Printf("Status: %s\n", state.Status)
	fmt.Printf("Retries: validation=%d test=%d review=%d\n", state.Retries.Validation, state.Retries.Testing, state.Retries.Review)
	fmt.Printf("Log file: .orch/runs/%s.json\n", state.ID)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}
