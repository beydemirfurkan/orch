package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/providers/openai"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Validate Orch runtime readiness",
	RunE:  runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("failed to load configuration (run 'orch init' first): %w", err)
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

	key := strings.TrimSpace(os.Getenv(cfg.Provider.OpenAI.APIKeyEnv))
	checks = append(checks, check{
		name:   "openai.api_key",
		ok:     key != "",
		detail: fmt.Sprintf("env=%s", cfg.Provider.OpenAI.APIKeyEnv),
	})

	checks = append(checks, check{name: "openai.model.planner", ok: strings.TrimSpace(cfg.Provider.OpenAI.Models.Planner) != "", detail: cfg.Provider.OpenAI.Models.Planner})
	checks = append(checks, check{name: "openai.model.coder", ok: strings.TrimSpace(cfg.Provider.OpenAI.Models.Coder) != "", detail: cfg.Provider.OpenAI.Models.Coder})
	checks = append(checks, check{name: "openai.model.reviewer", ok: strings.TrimSpace(cfg.Provider.OpenAI.Models.Reviewer) != "", detail: cfg.Provider.OpenAI.Models.Reviewer})

	if key != "" && cfg.Provider.Flags.OpenAIEnabled && defaultProvider == "openai" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		validateErr := openai.New(cfg.Provider.OpenAI).Validate(ctx)
		checks = append(checks, check{
			name:   "openai.auth",
			ok:     validateErr == nil,
			detail: errDetail(validateErr, "ok"),
		})
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
