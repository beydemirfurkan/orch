package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/furkanbeydemir/orch/internal/session"
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

var sessionMessagesCmd = &cobra.Command{
	Use:   "messages [name-or-id]",
	Short: "List recent messages for a session",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSessionMessages,
}

var (
	sessionCreateWorktree string
	sessionRunsLimit      int
	sessionRunsStatus     string
	sessionRunsContains   string
	sessionRunsJSON       bool
	sessionMessagesLimit  int
	sessionMessagesJSON   bool
	sessionListJSON       bool
	sessionCurrentJSON    bool
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
	sessionCmd.AddCommand(sessionMessagesCmd)

	sessionCreateCmd.Flags().StringVar(&sessionCreateWorktree, "worktree-path", "", "Optional worktree path for session execution")
	sessionListCmd.Flags().BoolVar(&sessionListJSON, "json", false, "Output sessions as JSON")
	sessionCurrentCmd.Flags().BoolVar(&sessionCurrentJSON, "json", false, "Output active session as JSON")
	sessionRunsCmd.Flags().IntVar(&sessionRunsLimit, "limit", 20, "Maximum number of runs to show")
	sessionRunsCmd.Flags().StringVar(&sessionRunsStatus, "status", "", "Filter runs by status")
	sessionRunsCmd.Flags().StringVar(&sessionRunsContains, "contains", "", "Filter runs by task text")
	sessionRunsCmd.Flags().BoolVar(&sessionRunsJSON, "json", false, "Output runs as JSON")
	sessionMessagesCmd.Flags().IntVar(&sessionMessagesLimit, "limit", 40, "Maximum number of messages to show")
	sessionMessagesCmd.Flags().BoolVar(&sessionMessagesJSON, "json", false, "Output messages as JSON")
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
		if sessionListJSON {
			fmt.Println("[]")
			return nil
		}
		fmt.Println("No sessions found.")
		return nil
	}

	if sessionListJSON {
		type jsonSession struct {
			ID        string     `json:"id"`
			ProjectID string     `json:"project_id"`
			Name      string     `json:"name"`
			Status    string     `json:"status"`
			Worktree  string     `json:"worktree,omitempty"`
			CreatedAt time.Time  `json:"created_at"`
			ClosedAt  *time.Time `json:"closed_at,omitempty"`
			IsActive  bool       `json:"is_active"`
		}
		payload := make([]jsonSession, 0, len(sessions))
		for _, s := range sessions {
			payload = append(payload, jsonSession{
				ID:        s.ID,
				ProjectID: s.ProjectID,
				Name:      s.Name,
				Status:    s.Status,
				Worktree:  s.Worktree,
				CreatedAt: s.CreatedAt,
				ClosedAt:  s.ClosedAt,
				IsActive:  s.IsActive,
			})
		}
		encoded, marshalErr := json.MarshalIndent(payload, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("failed to encode sessions json: %w", marshalErr)
		}
		fmt.Println(string(encoded))
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

	if sessionCurrentJSON {
		payload := map[string]any{
			"id":         ctx.Session.ID,
			"project_id": ctx.ProjectID,
			"name":       ctx.Session.Name,
			"status":     ctx.Session.Status,
			"worktree":   ctx.Session.Worktree,
			"is_active":  true,
		}
		encoded, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to encode current session json: %w", err)
		}
		fmt.Println(string(encoded))
		return nil
	}

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
		if sessionRunsJSON {
			fmt.Println("[]")
			return nil
		}
		fmt.Printf("No runs found for session %s (%s).\n", target.Name, target.ID)
		return nil
	}

	if sessionRunsJSON {
		type jsonRun struct {
			ID          string     `json:"id"`
			SessionID   string     `json:"session_id"`
			Status      string     `json:"status"`
			Task        string     `json:"task"`
			StartedAt   time.Time  `json:"started_at"`
			CompletedAt *time.Time `json:"completed_at,omitempty"`
			Error       string     `json:"error,omitempty"`
		}
		payload := make([]jsonRun, 0, len(runs))
		for _, run := range runs {
			payload = append(payload, jsonRun{
				ID:          run.ID,
				SessionID:   run.SessionID,
				Status:      run.Status,
				Task:        run.Task,
				StartedAt:   run.StartedAt,
				CompletedAt: run.CompletedAt,
				Error:       run.Error,
			})
		}
		encoded, marshalErr := json.MarshalIndent(payload, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("failed to encode runs json: %w", marshalErr)
		}
		fmt.Println(string(encoded))
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

func runSessionMessages(cmd *cobra.Command, args []string) error {
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

	svc := session.NewService(ctx.Store)
	messages, err := svc.ListMessagesWithParts(target.ID, sessionMessagesLimit)
	if err != nil {
		return err
	}

	if len(messages) == 0 {
		if sessionMessagesJSON {
			fmt.Println("[]")
			return nil
		}
		fmt.Printf("No messages found for session %s (%s).\n", target.Name, target.ID)
		return nil
	}

	if sessionMessagesJSON {
		type jsonPart struct {
			ID        string `json:"id"`
			Type      string `json:"type"`
			Compacted bool   `json:"compacted"`
			Rendered  string `json:"rendered"`
			Payload   string `json:"payload"`
		}
		type jsonMessage struct {
			ID           string     `json:"id"`
			SessionID    string     `json:"session_id"`
			Role         string     `json:"role"`
			ParentID     string     `json:"parent_id,omitempty"`
			ProviderID   string     `json:"provider_id,omitempty"`
			ModelID      string     `json:"model_id,omitempty"`
			FinishReason string     `json:"finish_reason,omitempty"`
			Error        string     `json:"error,omitempty"`
			CreatedAt    time.Time  `json:"created_at"`
			Parts        []jsonPart `json:"parts"`
		}

		payload := make([]jsonMessage, 0, len(messages))
		for _, item := range messages {
			parts := make([]jsonPart, 0, len(item.Parts))
			for _, part := range item.Parts {
				parts = append(parts, jsonPart{
					ID:        part.ID,
					Type:      part.Type,
					Compacted: part.Compacted,
					Rendered:  renderSessionPart(part),
					Payload:   part.Payload,
				})
			}
			payload = append(payload, jsonMessage{
				ID:           item.Message.ID,
				SessionID:    item.Message.SessionID,
				Role:         item.Message.Role,
				ParentID:     item.Message.ParentID,
				ProviderID:   item.Message.ProviderID,
				ModelID:      item.Message.ModelID,
				FinishReason: item.Message.FinishReason,
				Error:        item.Message.Error,
				CreatedAt:    item.Message.CreatedAt,
				Parts:        parts,
			})
		}

		encoded, marshalErr := json.MarshalIndent(payload, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("failed to encode session messages json: %w", marshalErr)
		}
		fmt.Println(string(encoded))
		return nil
	}

	for _, item := range messages {
		line := fmt.Sprintf("- %s role=%s at=%s", item.Message.ID, item.Message.Role, item.Message.CreatedAt.Format(time.RFC3339))
		if strings.TrimSpace(item.Message.ProviderID) != "" || strings.TrimSpace(item.Message.ModelID) != "" {
			line += fmt.Sprintf(" model=%s/%s", item.Message.ProviderID, item.Message.ModelID)
		}
		if strings.TrimSpace(item.Message.FinishReason) != "" {
			line += fmt.Sprintf(" finish=%s", item.Message.FinishReason)
		}
		if strings.TrimSpace(item.Message.Error) != "" {
			line += fmt.Sprintf(" error=%q", item.Message.Error)
		}
		fmt.Println(line)

		for _, part := range item.Parts {
			fmt.Printf("    - %s\n", renderSessionPart(part))
		}
	}

	return nil
}

func renderSessionPart(part storage.SessionPart) string {
	partType := strings.ToLower(strings.TrimSpace(part.Type))
	if partType == "" {
		partType = "unknown"
	}
	compactedSuffix := ""
	if part.Compacted {
		compactedSuffix = " compacted=true"
	}

	switch partType {
	case "text":
		text := strings.TrimSpace(session.ExtractTextPart(part))
		if text == "" {
			text = compactPayload(part.Payload, 140)
		}
		return fmt.Sprintf("part=text%s text=%q", compactedSuffix, text)
	case "stage":
		var payload map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(part.Payload)), &payload); err != nil {
			return fmt.Sprintf("part=stage%s payload=%q", compactedSuffix, compactPayload(part.Payload, 140))
		}
		actor := extractString(payload, "actor")
		step := extractString(payload, "step")
		message := extractString(payload, "message")
		timestamp := extractString(payload, "timestamp")
		if actor == "" && step == "" && message == "" {
			if status := extractString(payload, "status"); status != "" {
				runID := extractString(payload, "run_id")
				return fmt.Sprintf("part=stage%s run_id=%q status=%q", compactedSuffix, runID, status)
			}
			return fmt.Sprintf("part=stage%s payload=%q", compactedSuffix, compactPayload(part.Payload, 140))
		}
		if len(message) > 140 {
			message = message[:140] + "..."
		}
		return fmt.Sprintf("part=stage%s actor=%q step=%q message=%q at=%q", compactedSuffix, actor, step, message, timestamp)
	case "compaction":
		var payload map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(part.Payload)), &payload); err != nil {
			return fmt.Sprintf("part=compaction%s payload=%q", compactedSuffix, compactPayload(part.Payload, 140))
		}
		estimated, _ := payload["estimated_tokens"].(float64)
		usable, _ := payload["usable_input"].(float64)
		summary := extractString(payload, "summary")
		if len(summary) > 120 {
			summary = summary[:120] + "..."
		}
		return fmt.Sprintf("part=compaction%s estimated_tokens=%.0f usable_input=%.0f summary=%q", compactedSuffix, estimated, usable, summary)
	case "error":
		var payload map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(part.Payload)), &payload); err == nil {
			message := extractString(payload, "message")
			if message != "" {
				return fmt.Sprintf("part=error%s message=%q", compactedSuffix, message)
			}
		}
		return fmt.Sprintf("part=error%s payload=%q", compactedSuffix, compactPayload(part.Payload, 140))
	default:
		return fmt.Sprintf("part=%s%s payload=%q", partType, compactedSuffix, compactPayload(part.Payload, 140))
	}
}

func extractString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	if value, ok := payload[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func compactPayload(payload string, maxLen int) string {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" {
		return ""
	}
	if maxLen <= 0 {
		maxLen = 120
	}
	if len(trimmed) <= maxLen {
		return trimmed
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		if text, ok := parsed["text"].(string); ok {
			text = strings.TrimSpace(text)
			if len(text) > maxLen {
				return text[:maxLen] + "..."
			}
			return text
		}
	}

	return trimmed[:maxLen] + "..."
}
