package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/providers"
	"github.com/spf13/cobra"
)

var providerJSONFlag bool

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

var providerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all/default/connected providers",
	RunE:  runProviderList,
}

func init() {
	providerCmd.Flags().BoolVar(&providerJSONFlag, "json", false, "Output as JSON")
	providerListCmd.Flags().BoolVar(&providerJSONFlag, "json", false, "Output as JSON")
	providerCmd.AddCommand(providerSetCmd)
	providerCmd.AddCommand(providerListCmd)
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

	state, err := providers.ReadState(cwd)
	if err != nil {
		return err
	}

	if providerJSONFlag {
		payload := map[string]any{
			"all":       state.All,
			"default":   state.Default,
			"connected": state.Connected,
			"openai": map[string]any{
				"enabled":   state.OpenAI.Enabled,
				"connected": state.OpenAI.Connected,
				"mode":      state.OpenAI.Mode,
				"source":    state.OpenAI.Source,
				"reason":    state.OpenAI.Reason,
			},
		}
		encoded, marshalErr := json.MarshalIndent(payload, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("failed to serialize provider output: %w", marshalErr)
		}
		fmt.Println(string(encoded))
		return nil
	}

	fmt.Println("Provider Configuration")
	fmt.Println("----------------------")
	fmt.Printf("default: %s\n", cfg.Provider.Default)
	fmt.Printf("all: %s\n", strings.Join(state.All, ", "))
	if len(state.Connected) == 0 {
		fmt.Println("connected: (none)")
	} else {
		fmt.Printf("connected: %s\n", strings.Join(state.Connected, ", "))
	}
	fmt.Printf("openai.enabled: %t\n", cfg.Provider.Flags.OpenAIEnabled)
	fmt.Printf("openai.authMode: %s\n", cfg.Provider.OpenAI.AuthMode)
	if state.OpenAI.Connected {
		fmt.Printf("openai.connection: connected (%s)\n", state.OpenAI.Source)
	} else {
		fmt.Printf("openai.connection: disconnected (%s)\n", state.OpenAI.Reason)
	}
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

func runProviderList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	state, err := providers.ReadState(cwd)
	if err != nil {
		return err
	}

	if providerJSONFlag {
		payload := map[string]any{
			"all":       state.All,
			"default":   state.Default,
			"connected": state.Connected,
		}
		encoded, marshalErr := json.MarshalIndent(payload, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("failed to serialize provider output: %w", marshalErr)
		}
		fmt.Println(string(encoded))
		return nil
	}

	fmt.Println("Providers")
	fmt.Println("---------")
	if len(state.All) == 0 {
		fmt.Println("all: (none)")
	} else {
		fmt.Printf("all: %s\n", strings.Join(state.All, ", "))
	}

	if len(state.Default) == 0 {
		fmt.Println("default: (none)")
	} else {
		keys := make([]string, 0, len(state.Default))
		for provider := range state.Default {
			keys = append(keys, provider)
		}
		sort.Strings(keys)
		for _, provider := range keys {
			fmt.Printf("default.%s: %s\n", provider, state.Default[provider])
		}
	}

	if len(state.Connected) == 0 {
		fmt.Println("connected: (none)")
	} else {
		fmt.Printf("connected: %s\n", strings.Join(state.Connected, ", "))
	}

	return nil
}
