package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/furkanbeydemir/orch/internal/storage"
	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage run sessions",
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sessions",
	RunE:  runSessionList,
}

var sessionCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionCreate,
}

var sessionRunsCmd = &cobra.Command{
	Use:   "runs [name-or-id]",
	Short: "List recent runs for a session",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSessionRuns,
}

var (
	sessionCreateWorktree string
	sessionRunsLimit      int
	sessionRunsStatus     string
	sessionRunsContains   string
)

var sessionSelectCmd = &cobra.Command{
	Use:   "select [name-or-id]",
	Short: "Select active session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionSelect,
}

var sessionCloseCmd = &cobra.Command{
	Use:   "close [name-or-id]",
	Short: "Close a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionClose,
}

var sessionCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show active session",
	RunE:  runSessionCurrent,
}

func init() {
	rootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionCreateCmd)
	sessionCmd.AddCommand(sessionSelectCmd)
	sessionCmd.AddCommand(sessionCloseCmd)
	sessionCmd.AddCommand(sessionCurrentCmd)
	sessionCmd.AddCommand(sessionRunsCmd)

	sessionCreateCmd.Flags().StringVar(&sessionCreateWorktree, "worktree-path", "", "Optional worktree path for session execution")
	sessionRunsCmd.Flags().IntVar(&sessionRunsLimit, "limit", 20, "Maximum number of runs to show")
	sessionRunsCmd.Flags().StringVar(&sessionRunsStatus, "status", "", "Filter runs by status")
	sessionRunsCmd.Flags().StringVar(&sessionRunsContains, "contains", "", "Filter runs by task text")
}

func runSessionList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	ctx, err := loadSessionContext(cwd)
	if err != nil {
		return err
	}
	defer ctx.Store.Close()

	sessions, err := ctx.Store.ListSessions(ctx.ProjectID)
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	for _, s := range sessions {
		marker := " "
		if s.IsActive {
			marker = "*"
		}
		if s.Worktree != "" {
			fmt.Printf("%s %s (%s) status=%s worktree=%s\n", marker, s.Name, s.ID, s.Status, s.Worktree)
		} else {
			fmt.Printf("%s %s (%s) status=%s\n", marker, s.Name, s.ID, s.Status)
		}
	}

	return nil
}

func runSessionCreate(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	ctx, err := loadSessionContext(cwd)
	if err != nil {
		return err
	}
	defer ctx.Store.Close()

	created, err := ctx.Store.CreateSessionWithWorktree(ctx.ProjectID, args[0], sessionCreateWorktree)
	if err != nil {
		if errors.Is(err, storage.ErrNameConflict) {
			return fmt.Errorf("session already exists: %s", args[0])
		}
		return err
	}

	if err := ctx.Store.SetActiveSession(ctx.ProjectID, created.ID); err != nil {
		return err
	}

	if created.Worktree != "" {
		fmt.Printf("Created and selected session: %s (%s) worktree=%s\n", created.Name, created.ID, created.Worktree)
	} else {
		fmt.Printf("Created and selected session: %s (%s)\n", created.Name, created.ID)
	}
	return nil
}

func runSessionSelect(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	ctx, err := loadSessionContext(cwd)
	if err != nil {
		return err
	}
	defer ctx.Store.Close()

	selected, err := ctx.Store.SelectSession(ctx.ProjectID, args[0])
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrSessionNotFound):
			return fmt.Errorf("session not found: %s", args[0])
		case errors.Is(err, storage.ErrSessionClosed):
			return fmt.Errorf("cannot select closed session: %s", args[0])
		default:
			return err
		}
	}

	fmt.Printf("Active session: %s (%s)\n", selected.Name, selected.ID)
	return nil
}

func runSessionClose(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	ctx, err := loadSessionContext(cwd)
	if err != nil {
		return err
	}
	defer ctx.Store.Close()

	if err := ctx.Store.CloseSession(ctx.ProjectID, args[0]); err != nil {
		if errors.Is(err, storage.ErrSessionNotFound) {
			return fmt.Errorf("session not found: %s", args[0])
		}
		return err
	}

	fmt.Printf("Closed session: %s\n", args[0])
	return nil
}

func runSessionCurrent(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	ctx, err := loadSessionContext(cwd)
	if err != nil {
		return err
	}
	defer ctx.Store.Close()

	fmt.Printf("Active session: %s (%s)\n", ctx.Session.Name, ctx.Session.ID)
	if ctx.Session.Worktree != "" {
		fmt.Printf("Worktree: %s\n", ctx.Session.Worktree)
	}
	return nil
}

func runSessionRuns(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	ctx, err := loadSessionContext(cwd)
	if err != nil {
		return err
	}
	defer ctx.Store.Close()

	target := ctx.Session
	if len(args) == 1 {
		target, err = ctx.Store.GetSession(ctx.ProjectID, args[0])
		if err != nil {
			switch {
			case errors.Is(err, storage.ErrSessionNotFound):
				return fmt.Errorf("session not found: %s", args[0])
			default:
				return err
			}
		}
	}

	runs, err := ctx.Store.ListRunsBySessionFiltered(target.ID, sessionRunsLimit, sessionRunsStatus, sessionRunsContains)
	if err != nil {
		return err
	}

	if len(runs) == 0 {
		fmt.Printf("No runs found for session %s (%s).\n", target.Name, target.ID)
		return nil
	}

	for _, run := range runs {
		line := fmt.Sprintf("- %s status=%s task=%q started=%s", run.ID, run.Status, run.Task, run.StartedAt.Format(time.RFC3339))
		if run.CompletedAt != nil {
			line += fmt.Sprintf(" completed=%s", run.CompletedAt.Format(time.RFC3339))
		}
		if run.Error != "" {
			line += fmt.Sprintf(" error=%q", run.Error)
		}
		fmt.Println(line)
	}

	return nil
}
