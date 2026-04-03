package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/furkanbeydemir/orch/internal/auth"
	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/spf13/cobra"
)

var (
	authModeFlag     string
	authMethodFlag   string
	authFlowFlag     string
	authProviderFlag string
	authEmailFlag    string
	authAPIKeyFlag   string
)

var runOAuthLoginFlow = auth.RunOAuthFlow

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

var authUseCmd = &cobra.Command{
	Use:   "use <credential-id>",
	Short: "Set the active stored credential",
	Args:  cobra.ExactArgs(1),
	RunE:  runAuthUse,
}

var authRemoveCmd = &cobra.Command{
	Use:     "remove <credential-id>",
	Aliases: []string{"rm"},
	Short:   "Remove one stored credential",
	Args:    cobra.ExactArgs(1),
	RunE:    runAuthRemove,
}

var authOpenAICmd = &cobra.Command{
	Use:    "openai",
	Hidden: true,
	Short:  "Compatibility shim for old auth syntax",
}

func init() {
	authLoginCmd.Flags().StringVarP(&authProviderFlag, "provider", "p", "openai", "Provider id")
	authLoginCmd.Flags().StringVarP(&authMethodFlag, "method", "m", "", "Auth method: api or account")
	authLoginCmd.Flags().StringVar(&authFlowFlag, "flow", "auto", "Account auth flow: auto, browser, or headless")
	authLoginCmd.Flags().StringVar(&authModeFlag, "mode", "", "Deprecated: account or api_key")
	authLoginCmd.Flags().StringVar(&authEmailFlag, "email", "", "Account email for status display")
	authLoginCmd.Flags().StringVar(&authAPIKeyFlag, "api-key", "", "API key")

	authLogoutCmd.Flags().StringVarP(&authProviderFlag, "provider", "p", "openai", "Provider id")
	authUseCmd.Flags().StringVarP(&authProviderFlag, "provider", "p", "openai", "Provider id")
	authRemoveCmd.Flags().StringVarP(&authProviderFlag, "provider", "p", "openai", "Provider id")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authListCmd)
	authCmd.AddCommand(authUseCmd)
	authCmd.AddCommand(authRemoveCmd)
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
	compat.Flags().StringVar(&authFlowFlag, "flow", "auto", "Account auth flow: auto, browser, or headless")
	compat.Flags().StringVar(&authModeFlag, "mode", "", "Deprecated: account or api_key")
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
		if strings.TrimSpace(authFlowFlag) != "" {
			flow := strings.ToLower(strings.TrimSpace(authFlowFlag))
			if flow != "" && flow != "auto" {
				return fmt.Errorf("--flow is only supported with --method account")
			}
		}

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
		if active, activeErr := auth.Get(cwd, provider); activeErr == nil && active != nil {
			fmt.Printf("Active credential id: %s\n", active.ID)
		}
		return nil
	}

	flow, err := resolveAccountFlow()
	if err != nil {
		return err
	}

	fmt.Printf("Starting OpenAI account login (%s)...\n", flow)
	result, err := runOAuthLoginFlow(flow)
	if err != nil {
		return fmt.Errorf("account login failed: %w", err)
	}
	if strings.TrimSpace(result.AccessToken) == "" {
		return fmt.Errorf("oauth flow returned an empty access token")
	}

	email := strings.TrimSpace(result.Email)
	if explicit := strings.TrimSpace(authEmailFlag); explicit != "" {
		email = explicit
	}

	if err := auth.Set(cwd, provider, auth.Credential{
		Type:         "oauth",
		AccessToken:  strings.TrimSpace(result.AccessToken),
		RefreshToken: strings.TrimSpace(result.RefreshToken),
		ExpiresAt:    result.ExpiresAt,
		AccountID:    strings.TrimSpace(result.AccountID),
		Email:        email,
	}); err != nil {
		return err
	}

	cfg.Provider.OpenAI.AuthMode = "account"
	if err := config.Save(cwd, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("Credential saved to .orch/auth.json (0600).")
	fmt.Printf("Auth mode set to account. You can also use %s.\n", cfg.Provider.OpenAI.AccountTokenEnv)
	if active, activeErr := auth.Get(cwd, provider); activeErr == nil && active != nil {
		fmt.Printf("Active credential id: %s\n", active.ID)
	}
	if !result.ExpiresAt.IsZero() {
		fmt.Printf("Token expires at: %s\n", result.ExpiresAt.UTC().Format(time.RFC3339))
	}
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
	credentials, activeID, err := auth.List(cwd, "openai")
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
	storedRefresh := cred != nil && cred.Type == "oauth" && strings.TrimSpace(cred.RefreshToken) != ""

	fmt.Printf("api_key_env: %s (present=%t)\n", cfg.Provider.OpenAI.APIKeyEnv, envAPIKey)
	fmt.Printf("account_token_env: %s (present=%t)\n", cfg.Provider.OpenAI.AccountTokenEnv, envAccount)
	fmt.Printf("stored_api_key: %t\n", storedAPIKey)
	fmt.Printf("stored_account_token: %t\n", storedAccount)
	fmt.Printf("stored_account_refresh: %t\n", storedRefresh)
	fmt.Printf("stored_credentials: %d\n", len(credentials))
	if activeID != "" {
		fmt.Printf("active_credential_id: %s\n", activeID)
	}
	if cred != nil && cred.Type == "oauth" && !cred.ExpiresAt.IsZero() {
		fmt.Printf("account_expires_at: %s\n", cred.ExpiresAt.UTC().Format(time.RFC3339))
	}
	if cred != nil && cred.Type == "oauth" && strings.TrimSpace(cred.AccountID) != "" {
		fmt.Printf("account_id: %s\n", cred.AccountID)
	}
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

	provider := resolveProviderArg(args)
	if provider != "openai" {
		return fmt.Errorf("unsupported provider: %s (supported: openai)", provider)
	}

	credentials, activeID, err := auth.List(cwd, provider)
	if err != nil {
		return err
	}

	fmt.Println("Stored Credentials")
	fmt.Println("------------------")
	if len(credentials) == 0 {
		fmt.Println("No stored credentials found.")
		return nil
	}

	for _, cred := range credentials {
		marker := " "
		if cred.ID == activeID {
			marker = "*"
		}
		line := fmt.Sprintf("%s %s (%s)", marker, cred.ID, cred.Type)
		if cred.Email != "" {
			line += " " + cred.Email
		}
		if cred.AccountID != "" {
			line += " account=" + cred.AccountID
		}
		fmt.Println(line)
		for _, detail := range authCredentialDetails(cred, cred.ID == activeID) {
			fmt.Printf("    %s\n", detail)
		}
	}

	return nil
}

func authCredentialDetails(cred auth.Credential, active bool) []string {
	details := make([]string, 0, 5)
	if active {
		details = append(details, "status=active")
	}
	if !cred.CooldownUntil.IsZero() {
		if remaining := time.Until(cred.CooldownUntil); remaining > 0 {
			details = append(details, fmt.Sprintf("cooldown_until=%s (%s remaining)", cred.CooldownUntil.UTC().Format(time.RFC3339), remaining.Round(time.Second)))
		}
	}
	if !cred.LastUsedAt.IsZero() {
		details = append(details, fmt.Sprintf("last_used=%s", cred.LastUsedAt.UTC().Format(time.RFC3339)))
	}
	if strings.TrimSpace(cred.LastError) != "" {
		details = append(details, "last_error="+strings.TrimSpace(cred.LastError))
	}
	if cred.Type == "oauth" && !cred.ExpiresAt.IsZero() {
		details = append(details, fmt.Sprintf("expires_at=%s", cred.ExpiresAt.UTC().Format(time.RFC3339)))
	}
	return details
}

func runAuthUse(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	provider := resolveProviderArg(nil)
	if provider != "openai" {
		return fmt.Errorf("unsupported provider: %s (supported: openai)", provider)
	}
	credentialID := strings.TrimSpace(args[0])
	if err := auth.SetActive(cwd, provider, credentialID); err != nil {
		return err
	}

	fmt.Printf("Active credential set to %s for %s.\n", credentialID, provider)
	return nil
}

func runAuthRemove(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	provider := resolveProviderArg(nil)
	if provider != "openai" {
		return fmt.Errorf("unsupported provider: %s (supported: openai)", provider)
	}
	credentialID := strings.TrimSpace(args[0])
	if err := auth.RemoveCredential(cwd, provider, credentialID); err != nil {
		return err
	}

	fmt.Printf("Stored credential %s removed for %s.\n", credentialID, provider)
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
	if method == "oauth" || method == "browser" || method == "headless" {
		method = "account"
	}
	if method == "api" || method == "account" {
		return method, nil
	}

	if strings.TrimSpace(authAPIKeyFlag) != "" {
		return "api", nil
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

func resolveAccountFlow() (string, error) {
	flow := strings.ToLower(strings.TrimSpace(authFlowFlag))
	if flow == "" {
		flow = "auto"
	}

	if flow == "auto" {
		methodHint := strings.ToLower(strings.TrimSpace(authMethodFlag))
		switch methodHint {
		case "browser", "headless":
			flow = methodHint
		}
	}

	switch flow {
	case "auto", "browser", "headless":
		return flow, nil
	default:
		return "", fmt.Errorf("invalid account flow: %s (expected auto, browser, or headless)", flow)
	}
}
