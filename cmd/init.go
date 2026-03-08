// Package cmd - init komutu.
package cmd

import (
	"fmt"
	"os"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/repo"
	"github.com/furkanbeydemir/orch/internal/storage"
	"github.com/spf13/cobra"
)

// initCmd, orch init komutunu temsil eder.
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Analyze repository and create configuration",
	Long:  `Analyzes repository and creates .orch/ and configuration files.`,
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	fmt.Println("🔍 Repository analiz ediliyor...")

	if err := config.EnsureOrchDir(cwd); err != nil {
		return err
	}

	cfg := config.DefaultConfig()
	if err := config.Save(cwd, cfg); err != nil {
		return err
	}
	fmt.Println("✅ .orch/config.json created")

	// Repository analizi
	analyzer := repo.NewAnalyzer(cwd)
	repoMap, err := analyzer.Analyze()
	if err != nil {
		return fmt.Errorf("repository analysis failed: %w", err)
	}

	fmt.Printf("✅ .orch/repo-map.json created (%d files scanned)\n", len(repoMap.Files))
	fmt.Printf("📋 Language: %s | Package Manager: %s | Test: %s\n",
		repoMap.Language, repoMap.PackageManager, repoMap.TestFramework)

	store, err := storage.Open(cwd)
	if err != nil {
		return fmt.Errorf("failed to initialize sqlite storage: %w", err)
	}
	defer store.Close()

	projectID, err := store.GetOrCreateProject()
	if err != nil {
		return fmt.Errorf("failed to resolve project: %w", err)
	}

	session, err := store.EnsureDefaultSession(projectID)
	if err != nil {
		return fmt.Errorf("failed to resolve default session: %w", err)
	}

	fmt.Printf("✅ SQLite storage ready (.orch/orch.db), active session: %s (%s)\n", session.Name, session.ID)
	fmt.Println("\n🎯 Orch configured successfully!")

	return nil
}
