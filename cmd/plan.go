// Package cmd - plan komutu.
//
//   - Riskler
//   - Test stratejisi
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/orchestrator"
	"github.com/spf13/cobra"
)

// planCmd, orch plan komutunu temsil eder.
var planCmd = &cobra.Command{
	Use:   "plan [task]",
	Short: "Generates an implementation plan for a task",
	Long:  `Generates an AI implementation plan for the given task. Does not change code.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runPlan,
}

func init() {
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

	fmt.Printf("📝 Generating plan: %s\n\n", task.Description)

	orch := orchestrator.New(cfg, cwd, verbose)
	plan, err := orch.Plan(task)
	if err != nil {
		return fmt.Errorf("plan generation failed: %w", err)
	}

	fmt.Println("═══════════════════════════════════════")
	fmt.Println("📋 IMPLEMENTATION PLAN")
	fmt.Println("═══════════════════════════════════════")

	if len(plan.Steps) > 0 {
		fmt.Println("\n📌 Steps:")
		for _, step := range plan.Steps {
			fmt.Printf("  %d. %s\n", step.Order, step.Description)
		}
	}

	if len(plan.FilesToModify) > 0 {
		fmt.Println("\n📝 Files To Modify:")
		for _, f := range plan.FilesToModify {
			fmt.Printf("  - %s\n", f)
		}
	}

	if len(plan.FilesToInspect) > 0 {
		fmt.Println("\n🔍 Files To Inspect:")
		for _, f := range plan.FilesToInspect {
			fmt.Printf("  - %s\n", f)
		}
	}

	if len(plan.Risks) > 0 {
		fmt.Println("\n⚠️  Riskler:")
		for _, r := range plan.Risks {
			fmt.Printf("  - %s\n", r)
		}
	}

	if plan.TestStrategy != "" {
		fmt.Printf("\n🧪 Test Stratejisi: %s\n", plan.TestStrategy)
	}

	return nil
}
