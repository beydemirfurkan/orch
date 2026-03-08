// Package cmd - diff komutu.
package cmd

import (
	"fmt"
	"os"

	"github.com/furkanbeydemir/orch/internal/patch"
	"github.com/furkanbeydemir/orch/internal/runstore"
	"github.com/spf13/cobra"
)

// diffCmd, orch diff komutunu temsil eder.
var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Shows the generated patch",
	Long:  `Shows the unified diff patch from the latest run.`,
	RunE:  runDiff,
}

func init() {
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	rawDiff, err := runstore.LoadLatestPatch(cwd)
	if err != nil {
		fmt.Println("📄 Son uretilen patch:")
		fmt.Println("─────────────────────────────────────")
		fmt.Println("No generated patch found yet.")
		fmt.Println("Run 'orch run <task>' first.")
		return nil
	}

	p := patch.NewPipeline(10, 800)
	parsed, err := p.Process(rawDiff)
	if err != nil {
		return fmt.Errorf("patch parse/validation error: %w", err)
	}

	fmt.Println("📄 Latest generated patch:")
	fmt.Println("─────────────────────────────────────")
	fmt.Printf("Files: %d | Line limit: <= 800\n", len(parsed.Files))
	fmt.Println()
	fmt.Print(rawDiff)
	return nil
}
