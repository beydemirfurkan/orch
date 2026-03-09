package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/furkanbeydemir/orch/internal/auth"
	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/spf13/cobra"
)

var (
	authModeFlag  string
	authTokenFlag string
	authEmailFlag string
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage OpenAI authentication",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login with account token or configure api_key mode",
	RunE:  runAuthLogin,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	RunE:  runAuthStatus,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored account auth token",
	RunE:  runAuthLogout,
}

func init() {
	authLoginCmd.Flags().StringVar(&authModeFlag, "mode", "account", "Auth mode: account or api_key")
	authLoginCmd.Flags().StringVar(&authTokenFlag, "token", "", "Account token (optional, prompt if not set)")
	authLoginCmd.Flags().StringVar(&authEmailFlag, "email", "", "Account email for status display")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLogoutCmd)
	rootCmd.AddCommand(authCmd)
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	mode := strings.ToLower(strings.TrimSpace(authModeFlag))
	switch mode {
	case "api_key":
		cfg.Provider.OpenAI.AuthMode = "api_key"
		if err := config.Save(cwd, cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		fmt.Printf("Auth mode set to api_key. Export %s and run `orch doctor`.\n", cfg.Provider.OpenAI.APIKeyEnv)
		return nil
	case "account":
		// Continue below
	default:
		return fmt.Errorf("invalid auth mode: %s (expected account or api_key)", mode)
	}

	token := strings.TrimSpace(authTokenFlag)
	if token == "" {
		fmt.Println("No token provided. Starting automated login...")
		t, err := auth.RunOAuthFlow()
		if err != nil {
			fmt.Printf("\nAutomated login failed: %v\n", err)
			fmt.Print("Paste OpenAI account token manually: ")
			reader := bufio.NewReader(os.Stdin)
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				return fmt.Errorf("failed to read token: %w", readErr)
			}
			token = strings.TrimSpace(line)
		} else {
			token = t
		}
	}
	if token == "" {
		return fmt.Errorf("account token cannot be empty")
	}

	state := &auth.State{
		Provider:    "openai",
		Mode:        "account",
		AccessToken: token,
		Email:       strings.TrimSpace(authEmailFlag),
	}
	if err := auth.Save(cwd, state); err != nil {
		return err
	}

	cfg.Provider.OpenAI.AuthMode = "account"
	if err := config.Save(cwd, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("Account login saved to .orch/auth.json (0600).")
	fmt.Printf("Auth mode set to account. You can also use %s.\n", cfg.Provider.OpenAI.AccountTokenEnv)
	return nil
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	state, err := auth.Load(cwd)
	if err != nil {
		return err
	}

	fmt.Println("Auth Status")
	fmt.Println("-----------")
	fmt.Printf("provider: openai\n")
	fmt.Printf("mode: %s\n", cfg.Provider.OpenAI.AuthMode)

	if cfg.Provider.OpenAI.AuthMode == "api_key" {
		present := strings.TrimSpace(os.Getenv(cfg.Provider.OpenAI.APIKeyEnv)) != ""
		fmt.Printf("api_key_env: %s (present=%t)\n", cfg.Provider.OpenAI.APIKeyEnv, present)
		return nil
	}

	envTokenPresent := strings.TrimSpace(os.Getenv(cfg.Provider.OpenAI.AccountTokenEnv)) != ""
	stored := state != nil && strings.TrimSpace(state.AccessToken) != ""
	fmt.Printf("account_token_env: %s (present=%t)\n", cfg.Provider.OpenAI.AccountTokenEnv, envTokenPresent)
	fmt.Printf("stored_account_token: %t\n", stored)
	if state != nil && strings.TrimSpace(state.Email) != "" {
		fmt.Printf("email: %s\n", state.Email)
	}

	return nil
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	if err := auth.Clear(cwd); err != nil {
		return err
	}
	fmt.Println("Stored account auth removed.")
	return nil
}
