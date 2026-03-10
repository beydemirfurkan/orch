package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/spf13/cobra"
)

var explainCmd = &cobra.Command{
	Use:   "explain [run-id]",
	Short: "Explain why a run passed, failed, or was downgraded",
	Long:  `Shows the structured reasoning artifacts for a run, including plan, validation, test, review, and confidence signals.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runExplain,
}

func init() {
	rootCmd.AddCommand(explainCmd)
}

func runExplain(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	ctx, err := loadSessionContext(cwd)
	if err != nil {
		return err
	}
	defer ctx.Store.Close()

	var state *models.RunState
	if len(args) == 0 {
		state, err = ctx.Store.GetLatestRunStateBySession(ctx.Session.ID)
		if err != nil {
			return fmt.Errorf("failed to load latest run state: %w", err)
		}
	} else {
		state, err = ctx.Store.GetRunState(ctx.ProjectID, args[0])
		if err != nil {
			return fmt.Errorf("failed to load run state %s: %w", args[0], err)
		}
	}

	printExplain(state)
	return nil
}

func printExplain(state *models.RunState) {
	if state == nil {
		fmt.Println("No run state available.")
		return
	}

	fmt.Println("═══════════════════════════════════════")
	fmt.Println("🔎 RUN EXPLANATION")
	fmt.Println("═══════════════════════════════════════")
	fmt.Printf("Run ID: %s\n", state.ID)
	fmt.Printf("Status: %s\n", state.Status)
	fmt.Printf("Task: %s\n", state.Task.Description)
	if state.Error != "" {
		fmt.Printf("Error: %s\n", state.Error)
	}

	if state.TaskBrief != nil {
		fmt.Println("\n🧭 Task Brief")
		fmt.Printf("  Type: %s\n", state.TaskBrief.TaskType)
		fmt.Printf("  Risk: %s\n", state.TaskBrief.RiskLevel)
		fmt.Printf("  Goal: %s\n", state.TaskBrief.NormalizedGoal)
	}

	if state.Plan != nil {
		fmt.Println("\n📋 Plan")
		if state.Plan.Summary != "" {
			fmt.Printf("  Summary: %s\n", state.Plan.Summary)
		}
		if len(state.Plan.FilesToModify) > 0 {
			fmt.Printf("  Files To Modify: %s\n", strings.Join(state.Plan.FilesToModify, ", "))
		}
		if len(state.Plan.AcceptanceCriteria) > 0 {
			fmt.Println("  Acceptance Criteria:")
			for _, criterion := range state.Plan.AcceptanceCriteria {
				fmt.Printf("    - %s\n", criterion.Description)
			}
		}
	}

	if state.ExecutionContract != nil {
		fmt.Println("\n🧩 Execution Contract")
		if len(state.ExecutionContract.AllowedFiles) > 0 {
			fmt.Printf("  Allowed Files: %s\n", strings.Join(state.ExecutionContract.AllowedFiles, ", "))
		}
		if len(state.ExecutionContract.RequiredEdits) > 0 {
			fmt.Printf("  Required Edits: %s\n", strings.Join(state.ExecutionContract.RequiredEdits, " | "))
		}
		fmt.Printf("  Patch Budget: max_files=%d max_changed_lines=%d\n", state.ExecutionContract.PatchBudget.MaxFiles, state.ExecutionContract.PatchBudget.MaxChangedLines)
	}

	if len(state.ValidationResults) > 0 {
		fmt.Println("\n🛡️ Validation + Review/Test Gates")
		results := append([]models.ValidationResult(nil), state.ValidationResults...)
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].Stage == results[j].Stage {
				return results[i].Name < results[j].Name
			}
			return results[i].Stage < results[j].Stage
		})
		for _, result := range results {
			fmt.Printf("  - [%s] %s = %s (%s)\n", result.Stage, result.Name, result.Status, result.Summary)
		}
	}

	if len(state.TestFailures) > 0 {
		fmt.Println("\n🧪 Test Failure Classification")
		for _, failure := range state.TestFailures {
			line := fmt.Sprintf("  - %s: %s", failure.Code, failure.Summary)
			if failure.Flaky {
				line += " [flaky-suspected]"
			}
			fmt.Println(line)
		}
	} else if strings.TrimSpace(state.TestResults) != "" {
		fmt.Println("\n🧪 Test Output")
		fmt.Printf("  %s\n", strings.ReplaceAll(strings.TrimSpace(state.TestResults), "\n", "\n  "))
	}

	if state.ReviewScorecard != nil {
		fmt.Println("\n🧠 Review Scorecard")
		fmt.Printf("  Requirement Coverage: %d\n", state.ReviewScorecard.RequirementCoverage)
		fmt.Printf("  Scope Control: %d\n", state.ReviewScorecard.ScopeControl)
		fmt.Printf("  Regression Risk: %d\n", state.ReviewScorecard.RegressionRisk)
		fmt.Printf("  Readability: %d\n", state.ReviewScorecard.Readability)
		fmt.Printf("  Maintainability: %d\n", state.ReviewScorecard.Maintainability)
		fmt.Printf("  Test Adequacy: %d\n", state.ReviewScorecard.TestAdequacy)
		fmt.Printf("  Decision: %s\n", state.ReviewScorecard.Decision)
		if len(state.ReviewScorecard.Findings) > 0 {
			fmt.Println("  Findings:")
			for _, finding := range state.ReviewScorecard.Findings {
				fmt.Printf("    - %s\n", finding)
			}
		}
	}

	if state.Confidence != nil {
		fmt.Println("\n🎯 Confidence")
		fmt.Printf("  Score: %.2f\n", state.Confidence.Score)
		fmt.Printf("  Band: %s\n", state.Confidence.Band)
		if len(state.Confidence.Reasons) > 0 {
			fmt.Println("  Reasons:")
			for _, reason := range state.Confidence.Reasons {
				fmt.Printf("    - %s\n", reason)
			}
		}
		if len(state.Confidence.Warnings) > 0 {
			fmt.Println("  Warnings:")
			for _, warning := range state.Confidence.Warnings {
				fmt.Printf("    - %s\n", warning)
			}
		}
	}

	if state.Review != nil {
		fmt.Println("\n💬 Final Review")
		fmt.Printf("  Decision: %s\n", state.Review.Decision)
		for _, comment := range state.Review.Comments {
			fmt.Printf("  - %s\n", comment)
		}
		if len(state.Review.Suggestions) > 0 {
			fmt.Println("  Suggestions:")
			for _, suggestion := range state.Review.Suggestions {
				fmt.Printf("    - %s\n", suggestion)
			}
		}
	}

	if state.RetryDirective != nil {
		fmt.Println("\n🔁 Last Retry Directive")
		fmt.Printf("  Stage: %s\n", state.RetryDirective.Stage)
		fmt.Printf("  Attempt: %d\n", state.RetryDirective.Attempt)
		if len(state.RetryDirective.Reasons) > 0 {
			fmt.Printf("  Reasons: %s\n", strings.Join(state.RetryDirective.Reasons, " | "))
		}
		if len(state.RetryDirective.Instructions) > 0 {
			fmt.Printf("  Instructions: %s\n", strings.Join(state.RetryDirective.Instructions, " | "))
		}
	}
}
