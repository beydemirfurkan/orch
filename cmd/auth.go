package cmd

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/furkanbeydemir/orch/internal/auth"
	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/spf13/cobra"
)

var (
	authModeFlag     string
	authMethodFlag   string
	authProviderFlag string
	authTokenFlag    string
	authEmailFlag    string
	authAPIKeyFlag   string
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage provider credentials",
}

var authLoginCmd = &cobra.Command{
	Use:   "login [provider]",
	Short: "Log in to a provider",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAuthLogin,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	RunE:  runAuthStatus,
}

var authListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List stored credentials",
	RunE:    runAuthList,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout [provider]",
	Short: "Remove stored credentials",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAuthLogout,
}

var authOpenAICmd = &cobra.Command{
	Use:    "openai",
	Hidden: true,
	Short:  "Compatibility shim for old auth syntax",
}

func init() {
	authLoginCmd.Flags().StringVarP(&authProviderFlag, "provider", "p", "openai", "Provider id")
	authLoginCmd.Flags().StringVarP(&authMethodFlag, "method", "m", "", "Auth method: api or account")
	authLoginCmd.Flags().StringVar(&authModeFlag, "mode", "", "Deprecated: account or api_key")
	authLoginCmd.Flags().StringVar(&authTokenFlag, "token", "", "Account token")
	authLoginCmd.Flags().StringVar(&authEmailFlag, "email", "", "Account email for status display")
	authLoginCmd.Flags().StringVar(&authAPIKeyFlag, "api-key", "", "API key")

	authLogoutCmd.Flags().StringVarP(&authProviderFlag, "provider", "p", "openai", "Provider id")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authListCmd)
	authCmd.AddCommand(authLogoutCmd)
	authOpenAICmd.AddCommand(newAuthCompatLoginCmd())
	authOpenAICmd.AddCommand(newAuthCompatLogoutCmd())
	authOpenAICmd.AddCommand(newAuthCompatStatusCmd())
	authCmd.AddCommand(authOpenAICmd)
	rootCmd.AddCommand(authCmd)
}

func newAuthCompatLoginCmd() *cobra.Command {
	compat := &cobra.Command{
		Use:   "login",
		Short: "Compatibility login command",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthLogin(cmd, []string{"openai"})
		},
	}
	compat.Flags().StringVarP(&authProviderFlag, "provider", "p", "openai", "Provider id")
	compat.Flags().StringVarP(&authMethodFlag, "method", "m", "", "Auth method: api or account")
	compat.Flags().StringVar(&authModeFlag, "mode", "", "Deprecated: account or api_key")
	compat.Flags().StringVar(&authTokenFlag, "token", "", "Account token")
	compat.Flags().StringVar(&authEmailFlag, "email", "", "Account email for status display")
	compat.Flags().StringVar(&authAPIKeyFlag, "api-key", "", "API key")
	return compat
}

func newAuthCompatLogoutCmd() *cobra.Command {
	compat := &cobra.Command{
		Use:   "logout",
		Short: "Compatibility logout command",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthLogout(cmd, []string{"openai"})
		},
	}
	compat.Flags().StringVarP(&authProviderFlag, "provider", "p", "openai", "Provider id")
	return compat
}

func newAuthCompatStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Compatibility status command",
		RunE:  runAuthStatus,
	}
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

	provider := resolveProviderArg(args)
	if provider != "openai" {
		return fmt.Errorf("unsupported provider: %s (supported: openai)", provider)
	}

	method, err := resolveAuthMethod()
	if err != nil {
		return err
	}

	if method == "api" {
		key := strings.TrimSpace(authAPIKeyFlag)
		if key == "" {
			fmt.Print("Paste OpenAI API key: ")
			line, readErr := bufio.NewReader(os.Stdin).ReadString('\n')
			if readErr != nil {
				return fmt.Errorf("failed to read API key: %w", readErr)
			}
			key = strings.TrimSpace(line)
		}
		if key == "" {
			return fmt.Errorf("api key cannot be empty")
		}

		if err := auth.Set(cwd, provider, auth.Credential{Type: "api", Key: key}); err != nil {
			return err
		}

		cfg.Provider.OpenAI.AuthMode = "api_key"
		if err := config.Save(cwd, cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println("Credential saved to .orch/auth.json (0600).")
		fmt.Printf("Auth mode set to api_key. Env %s is still supported with higher priority.\n", cfg.Provider.OpenAI.APIKeyEnv)
		return nil
	}

	token := strings.TrimSpace(authTokenFlag)
	if token == "" {
		fmt.Println("No token provided. Starting automated login...")
		t, flowErr := auth.RunOAuthFlow()
		if flowErr != nil {
			fmt.Printf("\nAutomated login failed: %v\n", flowErr)
			fmt.Print("Paste OpenAI account token manually: ")
			line, readErr := bufio.NewReader(os.Stdin).ReadString('\n')
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

	if err := auth.Set(cwd, provider, auth.Credential{
		Type:        "oauth",
		AccessToken: token,
		Email:       strings.TrimSpace(authEmailFlag),
	}); err != nil {
		return err
	}

	cfg.Provider.OpenAI.AuthMode = "account"
	if err := config.Save(cwd, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("Credential saved to .orch/auth.json (0600).")
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

	cred, err := auth.Get(cwd, "openai")
	if err != nil {
		return err
	}

	fmt.Println("Auth Status")
	fmt.Println("-----------")
	fmt.Printf("provider: openai\n")
	fmt.Printf("mode: %s\n", cfg.Provider.OpenAI.AuthMode)

	envAPIKey := strings.TrimSpace(os.Getenv(cfg.Provider.OpenAI.APIKeyEnv)) != ""
	envAccount := strings.TrimSpace(os.Getenv(cfg.Provider.OpenAI.AccountTokenEnv)) != ""
	storedAPIKey := cred != nil && cred.Type == "api" && strings.TrimSpace(cred.Key) != ""
	storedAccount := cred != nil && cred.Type == "oauth" && strings.TrimSpace(cred.AccessToken) != ""

	fmt.Printf("api_key_env: %s (present=%t)\n", cfg.Provider.OpenAI.APIKeyEnv, envAPIKey)
	fmt.Printf("account_token_env: %s (present=%t)\n", cfg.Provider.OpenAI.AccountTokenEnv, envAccount)
	fmt.Printf("stored_api_key: %t\n", storedAPIKey)
	fmt.Printf("stored_account_token: %t\n", storedAccount)
	if cred != nil && strings.TrimSpace(cred.Email) != "" {
		fmt.Printf("email: %s\n", cred.Email)
	}

	return nil
}

func runAuthList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	all, err := auth.LoadAll(cwd)
	if err != nil {
		return err
	}

	fmt.Println("Stored Credentials")
	fmt.Println("------------------")
	if len(all) == 0 {
		fmt.Println("No stored credentials found.")
		return nil
	}

	providers := make([]string, 0, len(all))
	for provider := range all {
		providers = append(providers, provider)
	}
	sort.Strings(providers)

	for _, provider := range providers {
		cred := all[provider]
		fmt.Printf("%s (%s)\n", provider, cred.Type)
	}

	return nil
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	provider := resolveProviderArg(args)
	if provider != "openai" {
		return fmt.Errorf("unsupported provider: %s (supported: openai)", provider)
	}

	if err := auth.Remove(cwd, provider); err != nil {
		return err
	}

	fmt.Printf("Stored credential removed for %s.\n", provider)
	return nil
}

func resolveProviderArg(args []string) string {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		return strings.ToLower(strings.TrimSpace(args[0]))
	}
	if strings.TrimSpace(authProviderFlag) != "" {
		return strings.ToLower(strings.TrimSpace(authProviderFlag))
	}
	return "openai"
}

func resolveAuthMethod() (string, error) {
	mode := strings.ToLower(strings.TrimSpace(authModeFlag))
	if mode != "" {
		if mode == "api_key" {
			return "api", nil
		}
		if mode == "account" {
			return "account", nil
		}
		return "", fmt.Errorf("invalid auth mode: %s (expected account or api_key)", mode)
	}

	method := strings.ToLower(strings.TrimSpace(authMethodFlag))
	if method == "api_key" || method == "key" {
		method = "api"
	}
	if method == "oauth" || method == "browser" {
		method = "account"
	}
	if method == "api" || method == "account" {
		return method, nil
	}

	if strings.TrimSpace(authAPIKeyFlag) != "" {
		return "api", nil
	}
	if strings.TrimSpace(authTokenFlag) != "" {
		return "account", nil
	}

	fmt.Println("Select auth method:")
	fmt.Println("  1) OpenAI account (browser)")
	fmt.Println("  2) API key")
	fmt.Print("Choice [1-2]: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read choice: %w", err)
	}
	choice := strings.TrimSpace(line)
	if choice == "" || choice == "1" {
		return "account", nil
	}
	if choice == "2" {
		return "api", nil
	}
	return "", fmt.Errorf("invalid auth choice: %s", choice)
}
