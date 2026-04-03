package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/furkanbeydemir/orch/internal/auth"
	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/providers"
	"github.com/furkanbeydemir/orch/internal/providers/openai"
	"github.com/spf13/cobra"
)

var doctorProbeFlag bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Validate Orch runtime readiness",
	RunE:  runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().BoolVar(&doctorProbeFlag, "probe", false, "Run a live provider chat probe")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	type check struct {
		name   string
		ok     bool
		detail string
	}

	checks := make([]check, 0)

	defaultProvider := strings.TrimSpace(cfg.Provider.Default)
	checks = append(checks, check{
		name:   "provider.default",
		ok:     defaultProvider == "openai",
		detail: fmt.Sprintf("value=%q", defaultProvider),
	})

	checks = append(checks, check{
		name:   "provider.flags.openaiEnabled",
		ok:     cfg.Provider.Flags.OpenAIEnabled,
		detail: fmt.Sprintf("value=%t", cfg.Provider.Flags.OpenAIEnabled),
	})

	authMode := strings.ToLower(strings.TrimSpace(cfg.Provider.OpenAI.AuthMode))
	if authMode == "" {
		authMode = "api_key"
	}
	checks = append(checks, check{name: "openai.auth_mode", ok: authMode == "api_key" || authMode == "account", detail: authMode})

	key := strings.TrimSpace(os.Getenv(cfg.Provider.OpenAI.APIKeyEnv))
	accountToken := strings.TrimSpace(os.Getenv(cfg.Provider.OpenAI.AccountTokenEnv))

	storedCred, credErr := auth.Get(cwd, "openai")
	storedAPIKey := credErr == nil && storedCred != nil && storedCred.Type == "api" && strings.TrimSpace(storedCred.Key) != ""
	storedAccount := credErr == nil && storedCred != nil && storedCred.Type == "oauth" && strings.TrimSpace(storedCred.AccessToken) != ""
	storedRefresh := credErr == nil && storedCred != nil && storedCred.Type == "oauth" && strings.TrimSpace(storedCred.RefreshToken) != ""
	if credErr != nil {
		checks = append(checks, check{name: "openai.stored_credential", ok: false, detail: credErr.Error()})
	}

	checks = append(checks, check{
		name:   "openai.api_key",
		ok:     key != "" || storedAPIKey || authMode == "account",
		detail: fmt.Sprintf("env=%s", cfg.Provider.OpenAI.APIKeyEnv),
	})

	checks = append(checks, check{
		name:   "openai.account_token",
		ok:     accountToken != "" || storedAccount || authMode == "api_key",
		detail: fmt.Sprintf("env=%s stored=%t refresh=%t", cfg.Provider.OpenAI.AccountTokenEnv, storedAccount, storedRefresh),
	})

	checks = append(checks, check{
		name:   "openai.account_refresh",
		ok:     authMode != "account" || accountToken != "" || !storedAccount || storedRefresh || (storedCred != nil && storedCred.ExpiresAt.IsZero()) || time.Now().UTC().Before(storedCred.ExpiresAt),
		detail: fmt.Sprintf("required_when_expired=%t", authMode == "account" && accountToken == ""),
	})

	checks = append(checks, check{name: "openai.model.planner", ok: strings.TrimSpace(cfg.Provider.OpenAI.Models.Planner) != "", detail: cfg.Provider.OpenAI.Models.Planner})
	checks = append(checks, check{name: "openai.model.coder", ok: strings.TrimSpace(cfg.Provider.OpenAI.Models.Coder) != "", detail: cfg.Provider.OpenAI.Models.Coder})
	checks = append(checks, check{name: "openai.model.reviewer", ok: strings.TrimSpace(cfg.Provider.OpenAI.Models.Reviewer) != "", detail: cfg.Provider.OpenAI.Models.Reviewer})

	if cfg.Provider.Flags.OpenAIEnabled && defaultProvider == "openai" {
		client := newDoctorOpenAIClient(cwd, cfg.Provider.OpenAI, authMode, storedCred)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		validateErr := client.Validate(ctx)
		checks = append(checks, check{
			name:   "openai.auth.local",
			ok:     validateErr == nil,
			detail: errDetail(validateErr, "ok"),
		})

		if doctorProbeFlag {
			probeCtx, probeCancel := context.WithTimeout(context.Background(), doctorProbeTimeout(cfg.Provider.OpenAI.TimeoutSeconds))
			defer probeCancel()
			probeErr := runOpenAIProbe(probeCtx, client, cfg.Provider.OpenAI.Models.Coder)
			checks = append(checks, check{
				name:   "openai.auth.probe",
				ok:     probeErr == nil,
				detail: errDetail(probeErr, "ok"),
			})
		}
	}

	failed := 0
	fmt.Println("Orch Doctor")
	fmt.Println("-----------")
	for _, c := range checks {
		status := "PASS"
		if !c.ok {
			status = "FAIL"
			failed++
		}
		fmt.Printf("%-6s %-32s %s\n", status, c.name, c.detail)
	}

	if failed > 0 {
		return fmt.Errorf("doctor failed: %d checks failed", failed)
	}

	fmt.Println("All checks passed.")
	return nil
}

func errDetail(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	return err.Error()
}

func newDoctorOpenAIClient(cwd string, cfg config.OpenAIProviderConfig, authMode string, storedCred *auth.Credential) *openai.Client {
	client := openai.New(cfg)
	var accountSession *auth.AccountSession
	if authMode == "account" && strings.TrimSpace(os.Getenv(cfg.AccountTokenEnv)) == "" {
		accountSession = auth.NewAccountSession(cwd, "openai")
		client.SetAccountFailoverHandler(func(ctx context.Context, err error) (string, bool, error) {
			return accountSession.Failover(ctx, openai.AccountFailoverCooldown(err), err.Error())
		})
		client.SetAccountSuccessHandler(func(ctx context.Context) {
			accountSession.MarkSuccess(ctx)
		})
	}
	client.SetTokenResolver(func(ctx context.Context) (string, error) {
		if authMode == "api_key" {
			if storedCred != nil && strings.TrimSpace(storedCred.Key) != "" {
				return strings.TrimSpace(storedCred.Key), nil
			}
			return "", nil
		}
		if accountSession == nil {
			return "", nil
		}
		return accountSession.ResolveToken(ctx)
	})
	return client
}

func runOpenAIProbe(ctx context.Context, client *openai.Client, model string) error {
	_, err := client.Chat(ctx, providers.ChatRequest{
		Role:            providers.RoleCoder,
		Model:           strings.TrimSpace(model),
		SystemPrompt:    "Reply with OK only.",
		UserPrompt:      "ping",
		ReasoningEffort: "low",
	})
	return err
}

func doctorProbeTimeout(timeoutSeconds int) time.Duration {
	if timeoutSeconds <= 0 {
		return 20 * time.Second
	}
	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout > 20*time.Second {
		return 20 * time.Second
	}
	return timeout
}
