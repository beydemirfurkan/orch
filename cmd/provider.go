package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/spf13/cobra"
)

var providerCmd = &cobra.Command{
	Use:   "provider",
	Short: "Show or manage provider settings",
	RunE:  runProviderShow,
}

var providerSetCmd = &cobra.Command{
	Use:   "set [provider]",
	Short: "Set default provider",
	Args:  cobra.ExactArgs(1),
	RunE:  runProviderSet,
}

func init() {
	providerCmd.AddCommand(providerSetCmd)
	rootCmd.AddCommand(providerCmd)
}

func runProviderShow(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Println("Provider Configuration")
	fmt.Println("----------------------")
	fmt.Printf("default: %s\n", cfg.Provider.Default)
	fmt.Printf("openai.enabled: %t\n", cfg.Provider.Flags.OpenAIEnabled)
	fmt.Printf("openai.authMode: %s\n", cfg.Provider.OpenAI.AuthMode)
	fmt.Printf("openai.baseURL: %s\n", cfg.Provider.OpenAI.BaseURL)
	fmt.Printf("openai.apiKeyEnv: %s\n", cfg.Provider.OpenAI.APIKeyEnv)
	fmt.Printf("openai.accountTokenEnv: %s\n", cfg.Provider.OpenAI.AccountTokenEnv)
	fmt.Printf("openai.models: planner=%s coder=%s reviewer=%s\n",
		cfg.Provider.OpenAI.Models.Planner,
		cfg.Provider.OpenAI.Models.Coder,
		cfg.Provider.OpenAI.Models.Reviewer,
	)

	return nil
}

func runProviderSet(cmd *cobra.Command, args []string) error {
	provider := strings.ToLower(strings.TrimSpace(args[0]))
	if provider != "openai" {
		return fmt.Errorf("unsupported provider: %s (supported: openai)", provider)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	cfg.Provider.Default = provider
	cfg.Provider.Flags.OpenAIEnabled = true

	if err := config.Save(cwd, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Default provider set to %s\n", provider)
	return nil
}
