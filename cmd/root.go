// Subcommands: init, plan, run, diff, apply, logs.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	verbose    bool
	configPath string
)

var rootCmd = &cobra.Command{
	Use:   "orch",
	Short: "AI-powered coding orchestrator",
	Long: `Orch is a CLI orchestration engine that uses AI agents to execute coding tasks inside a repository.

Pipeline: Task → Plan → Code → Test → Review → Patch

Usage examples:
  orch init                            # Repository analysis and configuration
  orch plan "add redis caching"        # Generate plan only
  orch run "fix auth bug"              # Run full pipeline
  orch diff                            # Show generated patch
  orch apply                           # Apply patch
  orch logs                            # Show execution trace`,
	Version: "0.1.0",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output mode")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Configuration file path")
}
