// Package cmd implements the plan command.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/orchestrator"
	"github.com/spf13/cobra"
)

var planJSON bool

// planCmd represents the `orch plan` command.
var planCmd = &cobra.Command{
	Use:   "plan [task]",
	Short: "Generates an implementation plan for a task",
	Long:  `Generates an AI implementation plan for the given task. Does not change code.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runPlan,
}

func init() {
	planCmd.Flags().BoolVar(&planJSON, "json", false, "Output the structured plan as JSON")
	rootCmd.AddCommand(planCmd)
}

func runPlan(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("failed to load configuration (run 'orch init' first): %w", err)
	}

	task := &models.Task{
		ID:          fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Description: args[0],
		CreatedAt:   time.Now(),
	}

	orch := orchestrator.New(cfg, cwd, verbose)
	taskBrief, plan, err := orch.PlanDetailed(task)
	if err != nil {
		return fmt.Errorf("plan generation failed: %w", err)
	}

	if planJSON {
		payload := struct {
			Task      *models.Task      `json:"task"`
			TaskBrief *models.TaskBrief `json:"task_brief,omitempty"`
			Plan      *models.Plan      `json:"plan"`
		}{
			Task:      task,
			TaskBrief: taskBrief,
			Plan:      plan,
		}
		encoded, marshalErr := json.MarshalIndent(payload, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("failed to encode plan output: %w", marshalErr)
		}
		fmt.Println(string(encoded))
		return nil
	}

	fmt.Printf("📝 Generating plan: %s\n\n", task.Description)
	fmt.Println("═══════════════════════════════════════")
	fmt.Println("📋 IMPLEMENTATION PLAN")
	fmt.Println("═══════════════════════════════════════")

	if taskBrief != nil {
		fmt.Printf("\n🎯 Task Type: %s\n", taskBrief.TaskType)
		fmt.Printf("⚠️  Risk Level: %s\n", taskBrief.RiskLevel)
		if taskBrief.NormalizedGoal != "" {
			fmt.Printf("🧭 Goal: %s\n", taskBrief.NormalizedGoal)
		}
	}

	if plan != nil && plan.Summary != "" {
		fmt.Printf("\n🗺️  Summary: %s\n", plan.Summary)
	}

	if plan != nil && len(plan.Steps) > 0 {
		fmt.Println("\n📌 Steps:")
		for _, step := range plan.Steps {
			fmt.Printf("  %d. %s\n", step.Order, step.Description)
		}
	}

	if plan != nil && len(plan.FilesToModify) > 0 {
		fmt.Println("\n📝 Files To Modify:")
		for _, f := range plan.FilesToModify {
			fmt.Printf("  - %s\n", f)
		}
	}

	if plan != nil && len(plan.FilesToInspect) > 0 {
		fmt.Println("\n🔍 Files To Inspect:")
		for _, f := range plan.FilesToInspect {
			fmt.Printf("  - %s\n", f)
		}
	}

	if plan != nil && len(plan.AcceptanceCriteria) > 0 {
		fmt.Println("\n✅ Acceptance Criteria:")
		for _, criterion := range plan.AcceptanceCriteria {
			fmt.Printf("  - %s\n", criterion.Description)
		}
	}

	if plan != nil && len(plan.Invariants) > 0 {
		fmt.Println("\n🧱 Invariants:")
		for _, invariant := range plan.Invariants {
			fmt.Printf("  - %s\n", invariant)
		}
	}

	if plan != nil && len(plan.ForbiddenChanges) > 0 {
		fmt.Println("\n⛔ Forbidden Changes:")
		for _, forbidden := range plan.ForbiddenChanges {
			fmt.Printf("  - %s\n", forbidden)
		}
	}

	if plan != nil && len(plan.Risks) > 0 {
		fmt.Println("\n⚠️  Risks:")
		for _, r := range plan.Risks {
			fmt.Printf("  - %s\n", r)
		}
	}

	if plan != nil && len(plan.TestRequirements) > 0 {
		fmt.Println("\n🧪 Test Requirements:")
		for _, req := range plan.TestRequirements {
			fmt.Printf("  - %s\n", req)
		}
	} else if plan != nil && plan.TestStrategy != "" {
		fmt.Printf("\n🧪 Test Strategy: %s\n", plan.TestStrategy)
	}

	return nil
}
