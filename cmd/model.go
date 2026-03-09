package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/spf13/cobra"
)

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Show or manage role model mapping",
	RunE:  runModelShow,
}

var modelSetCmd = &cobra.Command{
	Use:   "set [role] [model]",
	Short: "Set model for role (planner|coder|reviewer)",
	Args:  cobra.ExactArgs(2),
	RunE:  runModelSet,
}

func init() {
	modelCmd.AddCommand(modelSetCmd)
	rootCmd.AddCommand(modelCmd)
}

func runModelShow(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Println("Model Mapping")
	fmt.Println("-------------")
	fmt.Printf("planner:  %s\n", cfg.Provider.OpenAI.Models.Planner)
	fmt.Printf("coder:    %s\n", cfg.Provider.OpenAI.Models.Coder)
	fmt.Printf("reviewer: %s\n", cfg.Provider.OpenAI.Models.Reviewer)

	return nil
}

func runModelSet(cmd *cobra.Command, args []string) error {
	role := strings.ToLower(strings.TrimSpace(args[0]))
	model := strings.TrimSpace(args[1])
	if model == "" {
		return fmt.Errorf("model cannot be empty")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	switch role {
	case "planner":
		cfg.Provider.OpenAI.Models.Planner = model
	case "coder":
		cfg.Provider.OpenAI.Models.Coder = model
	case "reviewer":
		cfg.Provider.OpenAI.Models.Reviewer = model
	default:
		return fmt.Errorf("invalid role: %s (expected planner|coder|reviewer)", role)
	}

	if err := config.Save(cwd, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Model for %s set to %s\n", role, model)
	return nil
}
