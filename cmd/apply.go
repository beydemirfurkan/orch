// Package cmd implements the apply command.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/patch"
	"github.com/spf13/cobra"
)

var (
	forceApply         bool
	approveDestructive bool
)

// applyCmd represents the `orch apply` command.
var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Applies generated patch",
	Long: `Applies the generated patch to the working tree.
Defaults to dry-run mode. Use --force to apply changes.`,
	RunE: runApply,
}

func init() {
	applyCmd.Flags().BoolVar(&forceApply, "force", false, "Skip dry-run mode and apply changes")
	applyCmd.Flags().BoolVar(&approveDestructive, "approve-destructive", false, "Explicitly approve destructive apply operations")
	rootCmd.AddCommand(applyCmd)
}

func runApply(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	ctx, err := loadSessionContext(cwd)
	if err != nil {
		return err
	}
	defer ctx.Store.Close()

	rawDiff, err := ctx.Store.LoadLatestPatchBySession(ctx.Session.ID)
	if err != nil {
		fmt.Println("No patch available to apply yet.")
		fmt.Println("Run 'orch run <task>' first.")
		return nil
	}

	pipeline := patch.NewPipeline(cfg.Patch.MaxFiles, cfg.Patch.MaxLines)
	parsedPatch, err := pipeline.Process(rawDiff)
	if err != nil {
		return fmt.Errorf("invalid patch: %w", err)
	}

	dryRun := cfg.Safety.DryRun
	if forceApply {
		dryRun = false
	}

	if cfg.Safety.RequireDestructiveApproval && !dryRun && !approveDestructive {
		return fmt.Errorf("destructive apply blocked: rerun with --approve-destructive")
	}

	if dryRun {
		fmt.Println("🔍 Dry-run mode (patch check)...")
		fmt.Println("   For real apply: orch apply --force")
	} else {
		fmt.Println("⚡ Applying patch (force mode)...")
	}

	if err := pipeline.Apply(parsedPatch, cwd, dryRun); err != nil {
		var conflictErr *patch.ConflictError
		if errors.As(err, &conflictErr) {
			fmt.Println("❌ Patch conflict detected.")
			for _, file := range conflictErr.Files {
				fmt.Printf("   - %s\n", file)
			}
			if conflictErr.BestPatchSummary != "" {
				fmt.Printf("🩹 Best Patch Summary: %s\n", conflictErr.BestPatchSummary)
			}
			return fmt.Errorf("patch conflict: %s", conflictErr.Reason)
		}

		var invalidPatchErr *patch.InvalidPatchError
		if errors.As(err, &invalidPatchErr) {
			if invalidPatchErr.BestPatchSummary != "" {
				fmt.Printf("🩹 Best Patch Summary: %s\n", invalidPatchErr.BestPatchSummary)
			}
			return fmt.Errorf("invalid diff: %s", invalidPatchErr.Reason)
		}

		return fmt.Errorf("patch apply failed: %w", err)
	}

	if dryRun {
		fmt.Println("✅ Dry-run succeeded. Patch is applicable.")
	} else {
		fmt.Println("✅ Patch applied successfully.")
	}

	return nil
}
