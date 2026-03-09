package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/furkanbeydemir/orch/internal/auth"
	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/providers"
	"github.com/furkanbeydemir/orch/internal/providers/openai"
)

type interactiveModel struct {
	viewport viewport.Model
	input    textarea.Model
	spinner  spinner.Model

	logs    []string
	running bool
	width   int
	height  int

	providerLine string
	authLine     string
	modelsLine   string
	verboseMode  bool
	sessionID    string
	resumed      bool
}

type theme struct {
	header      lipgloss.Style
	accent      lipgloss.Style
	muted       lipgloss.Style
	success     lipgloss.Style
	warning     lipgloss.Style
	error       lipgloss.Style
	panel       lipgloss.Style
	command     lipgloss.Style
	timeline    lipgloss.Style
	statusRun   lipgloss.Style
	statusIdle  lipgloss.Style
	chip        lipgloss.Style
	chipMuted   lipgloss.Style
	composerTag lipgloss.Style
	userCard    lipgloss.Style
	assistant   lipgloss.Style
	errorCard   lipgloss.Style
}

var dracula = theme{
	header:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E2E8F0")),
	accent:      lipgloss.NewStyle().Foreground(lipgloss.Color("#7DD3FC")),
	muted:       lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")),
	success:     lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")),
	warning:     lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")),
	error:       lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")),
	panel:       lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#334155")).Padding(0, 1),
	command:     lipgloss.NewStyle().Foreground(lipgloss.Color("#F9A8D4")),
	timeline:    lipgloss.NewStyle().Foreground(lipgloss.Color("#93C5FD")),
	statusRun:   lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true),
	statusIdle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Bold(true),
	chip:        lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1")).Background(lipgloss.Color("#0F172A")).Padding(0, 1),
	chipMuted:   lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Background(lipgloss.Color("#111827")).Padding(0, 1),
	composerTag: lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8")).Bold(true),
	userCard: lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#0EA5E9")).
		Background(lipgloss.Color("#082F49")).
		Foreground(lipgloss.Color("#E0F2FE")).
		Padding(0, 1),
	assistant: lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#334155")).
		Background(lipgloss.Color("#0B1220")).
		Foreground(lipgloss.Color("#E2E8F0")).
		Padding(0, 1),
	errorCard: lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#EF4444")).
		Background(lipgloss.Color("#450A0A")).
		Foreground(lipgloss.Color("#FECACA")).
		Padding(0, 1),
}

type commandResultMsg struct {
	command string
	output  string
	err     error
}

type runExecutionMsg struct {
	command string
	result  *runExecutionResult
	err     error
}

type chatExecutionResult struct {
	Text    string
	Warning string
}

type chatExecutionMsg struct {
	prompt string
	result *chatExecutionResult
	err    error
}

func startInteractiveShell(resumeID string) error {
	m := newInteractiveModel(resumeID)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newInteractiveModel(resumeID string) interactiveModel {
	input := textarea.New()
	input.Placeholder = "Ask anything, or use /help"
	input.Prompt = "› "
	input.CharLimit = 0
	input.ShowLineNumbers = false
	input.SetHeight(3)
	input.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("ctrl+j"), key.WithHelp("ctrl+j", "newline"))
	input.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Line

	vp := viewport.New(80, 20)

	activeSession := strings.TrimSpace(resumeID)
	resumed := activeSession != ""
	if activeSession == "" {
		activeSession = generateSessionID()
	}

	lines := []string{
		"Orch workspace assistant",
		fmt.Sprintf("Session: %s", activeSession),
		fmt.Sprintf("Resume:  orch -s %s", activeSession),
		"",
		"Start with plain text to chat with Orch.",
		"Use /run when you want code pipeline execution.",
		"Use /help for command reference.",
		"Use /verbose on for deep execution logs.",
	}
	if resumed {
		lines = append(lines, "Resumed existing interactive session.")
	}
	vp.SetContent(strings.Join(lines, "\n"))

	providerLine, authLine, modelsLine := readRuntimeStatus()

	return interactiveModel{
		viewport:     vp,
		input:        input,
		spinner:      sp,
		logs:         lines,
		providerLine: providerLine,
		authLine:     authLine,
		modelsLine:   modelsLine,
		verboseMode:  false,
		sessionID:    activeSession,
		resumed:      resumed,
	}
}

func (m interactiveModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink)
}

func (m interactiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := 8
		inputHeight := 7
		m.viewport.Width = msg.Width - 2
		m.viewport.Height = max(5, msg.Height-headerHeight-inputHeight)
		m.input.SetWidth(max(20, msg.Width-8))
		m.input.SetHeight(3)
		m.viewport.SetContent(strings.Join(m.logs, "\n"))
		m.viewport.GotoBottom()
		return m, nil

	case spinner.TickMsg:
		if m.running {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case commandResultMsg:
		m.running = false
		m.providerLine, m.authLine, m.modelsLine = readRuntimeStatus()
		m.appendUserMessage(msg.command)
		if strings.TrimSpace(msg.output) != "" {
			m.appendAssistantMessage("Output", strings.Split(strings.TrimRight(msg.output, "\n"), "\n"))
		}
		if msg.err != nil {
			m.appendErrorMessage(fmt.Sprintf("error: %v", msg.err))
		}
		m.appendSpacer()
		m.viewport.GotoBottom()
		return m, nil

	case runExecutionMsg:
		m.running = false
		m.providerLine, m.authLine, m.modelsLine = readRuntimeStatus()
		m.appendUserMessage(msg.command)
		if msg.err != nil {
			m.appendErrorMessage(fmt.Sprintf("error: %v", msg.err))
			m.appendSpacer()
			m.viewport.GotoBottom()
			return m, nil
		}
		m.appendAssistantMessage("Orch", []string{naturalRunReply(msg.result)})
		m.appendAssistantMessage("Run Result", compactRunLines(msg.result, m.verboseMode))
		m.appendSpacer()
		m.viewport.GotoBottom()
		return m, nil

	case chatExecutionMsg:
		m.running = false
		m.providerLine, m.authLine, m.modelsLine = readRuntimeStatus()
		m.appendUserMessage(msg.prompt)
		if msg.err != nil {
			m.appendErrorMessage(fmt.Sprintf("error: %v", msg.err))
			m.appendSpacer()
			m.viewport.GotoBottom()
			return m, nil
		}
		if msg.result != nil {
			m.appendAssistantMessage("Orch", strings.Split(strings.TrimSpace(msg.result.Text), "\n"))
			if strings.TrimSpace(msg.result.Warning) != "" {
				m.appendAssistantMessage("Note", []string{msg.result.Warning})
			}
		}
		m.appendSpacer()
		m.viewport.GotoBottom()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if strings.TrimSpace(m.input.Value()) != "" {
				m.input.SetValue("")
			}
			return m, nil
		case "ctrl+l":
			m.logs = []string{}
			m.viewport.SetContent("")
			return m, nil
		case "shift+enter", "alt+enter":
			if m.running {
				return m, nil
			}
			m.input.InsertString("\n")
			return m, nil
		case "enter", "ctrl+m":
			if m.running {
				return m, nil
			}

			raw := strings.TrimSpace(m.input.Value())
			m.input.SetValue("")
			if raw == "" {
				return m, nil
			}

			if raw == "/exit" || raw == "/quit" {
				return m, tea.Quit
			}

			if raw == "/clear" {
				m.logs = []string{}
				m.viewport.SetContent("")
				return m, nil
			}

			if raw == "/help" {
				m.appendAssistantMessage("Commands", strings.Split(helpText(), "\n"))
				m.appendSpacer()
				m.viewport.GotoBottom()
				return m, nil
			}

			if strings.HasPrefix(raw, "/verbose") {
				parts := strings.Fields(raw)
				if len(parts) == 1 {
					m.verboseMode = !m.verboseMode
				} else {
					switch strings.ToLower(parts[1]) {
					case "on":
						m.verboseMode = true
					case "off":
						m.verboseMode = false
					default:
						m.appendErrorMessage("error: /verbose expects 'on' or 'off'")
						m.appendSpacer()
						m.viewport.GotoBottom()
						return m, nil
					}
				}
				m.appendAssistantMessage("Settings", []string{fmt.Sprintf("verbose mode: %t", m.verboseMode)})
				m.appendSpacer()
				m.viewport.GotoBottom()
				return m, nil
			}

			args, err := parseInteractiveInput(raw)
			if err != nil {
				m.appendErrorMessage(fmt.Sprintf("error: %v", err))
				m.appendSpacer()
				m.viewport.GotoBottom()
				return m, nil
			}

			m.running = true
			if len(args) > 1 && args[0] == "run" {
				cmds = append(cmds, m.spinner.Tick, runInProcessCmd(args[1]))
			} else if len(args) > 1 && args[0] == "chat" {
				cmds = append(cmds, m.spinner.Tick, runInProcessChatCmd(args[1]))
			} else {
				cmds = append(cmds, m.spinner.Tick, runCLICommandCmd(args))
			}
			return m, tea.Batch(cmds...)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m interactiveModel) View() string {
	status := "idle"
	statusView := dracula.statusIdle.Render("idle")
	if m.running {
		status = m.spinner.View() + " running"
		statusView = dracula.statusRun.Render(status)
	} else {
		statusView = dracula.statusIdle.Render(status)
	}

	providerState := "provider unconfigured"
	providerStyle := dracula.chipMuted
	if !strings.Contains(strings.ToLower(m.providerLine), "inactive") && !strings.Contains(strings.ToLower(m.providerLine), "unknown") {
		providerState = "provider configured"
		providerStyle = dracula.chip
	}

	authState := "auth disconnected"
	authStyle := dracula.chipMuted
	if strings.Contains(strings.ToLower(m.authLine), "connected") {
		authState = "auth configured"
		authStyle = dracula.chip
	}

	modelSummary := shortModelsLine(m.modelsLine)

	headerLines := []string{
		dracula.header.Render("Orch Interactive") + "  " + statusView + "  " + dracula.chipMuted.Render("session "+m.sessionID),
		providerStyle.Render(providerState) + " " + authStyle.Render(authState) + " " + dracula.chipMuted.Render(modelSummary),
		dracula.muted.Render("plain text = chat | /run = pipeline | /help /clear /verbose on|off | Enter send"),
	}
	header := strings.Join(headerLines, "\n") + "\n"

	body := dracula.panel.Width(max(20, m.viewport.Width)).Render(m.viewport.View())

	composerStatus := "Message"
	if m.running {
		composerStatus = "Running..."
	}
	stats := fmt.Sprintf("%d chars | %d lines", m.input.Length(), m.input.LineCount())
	composerLines := []string{
		dracula.composerTag.Render(composerStatus) + "  " + dracula.muted.Render(stats),
		m.input.View(),
		dracula.muted.Render("Esc clears draft | Ctrl+J newline | /exit quits"),
	}
	composer := dracula.panel.Width(max(20, m.viewport.Width)).Render(strings.Join(composerLines, "\n"))

	return header + body + "\n" + composer
}

func (m *interactiveModel) appendLog(line string) {
	m.logs = append(m.logs, line)
	m.viewport.SetContent(strings.Join(m.logs, "\n"))
}

func (m *interactiveModel) appendSpacer() {
	m.appendLog("")
}

func (m *interactiveModel) appendUserMessage(command string) {
	label := dracula.accent.Render("You") + "  " + dracula.command.Render(command)
	card := dracula.userCard.Render(label)
	width := max(20, m.viewport.Width)
	m.appendLog(lipgloss.PlaceHorizontal(width, lipgloss.Right, card))
}

func (m *interactiveModel) appendAssistantMessage(title string, lines []string) {
	body := make([]string, 0, len(lines)+1)
	body = append(body, dracula.accent.Render("Orch")+"  "+dracula.header.Render(title))
	body = append(body, lines...)
	m.appendLog(dracula.assistant.Render(strings.Join(body, "\n")))
}

func (m *interactiveModel) appendErrorMessage(message string) {
	body := dracula.error.Render("Error") + "\n" + message
	m.appendLog(dracula.errorCard.Render(body))
}

func parseInteractiveInput(input string) ([]string, error) {
	if strings.HasPrefix(input, "/") {
		parts := strings.Fields(strings.TrimPrefix(input, "/"))
		if len(parts) == 0 {
			return nil, fmt.Errorf("empty command")
		}

		switch parts[0] {
		case "run", "plan", "chat":
			if len(parts) < 2 {
				return nil, fmt.Errorf("/%s requires a task", parts[0])
			}
			return []string{parts[0], strings.Join(parts[1:], " ")}, nil
		case "diff", "apply", "init":
			return []string{parts[0]}, nil
		case "doctor", "provider", "model", "models", "auth":
			if parts[0] == "models" {
				parts[0] = "model"
			}
			return append([]string{parts[0]}, parts[1:]...), nil
		case "logs":
			return append([]string{"logs"}, parts[1:]...), nil
		case "explain":
			return append([]string{"explain"}, parts[1:]...), nil
		case "stats":
			return append([]string{"stats"}, parts[1:]...), nil
		case "session":
			return append([]string{"session"}, parts[1:]...), nil
		default:
			return nil, fmt.Errorf("unknown command: %s", parts[0])
		}
	}

	return []string{"chat", input}, nil
}

func runCLICommandCmd(args []string) tea.Cmd {
	commandLabel := "orch " + strings.Join(args, " ")
	return func() tea.Msg {
		cmd := exec.Command(os.Args[0], args...)
		output, err := cmd.CombinedOutput()
		return commandResultMsg{
			command: commandLabel,
			output:  string(output),
			err:     err,
		}
	}
}

func runInProcessCmd(task string) tea.Cmd {
	commandLabel := "orch run " + task
	return func() tea.Msg {
		result, err := executeRunTask(task)
		return runExecutionMsg{
			command: commandLabel,
			result:  result,
			err:     err,
		}
	}
}

func runInProcessChatCmd(prompt string) tea.Cmd {
	return func() tea.Msg {
		result, err := executeChatPrompt(prompt)
		return chatExecutionMsg{
			prompt: prompt,
			result: result,
			err:    err,
		}
	}
}

func executeChatPrompt(prompt string) (*chatExecutionResult, error) {
	cwd, err := getWorkingDirectory()
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(cwd)
	if err != nil {
		return nil, fmt.Errorf("provider unavailable: failed to load config")
	}

	if !cfg.Provider.Flags.OpenAIEnabled || strings.ToLower(strings.TrimSpace(cfg.Provider.Default)) != "openai" {
		return nil, fmt.Errorf("provider unavailable: OpenAI provider is disabled or not selected")
	}

	providerLine, authLine, _ := readRuntimeStatus()
	if strings.Contains(strings.ToLower(providerLine), "inactive") || strings.Contains(strings.ToLower(providerLine), "unknown") {
		return nil, fmt.Errorf("provider unavailable: %s", providerLine)
	}
	if strings.Contains(strings.ToLower(authLine), "disconnected") {
		return nil, fmt.Errorf("provider unavailable: %s", authLine)
	}

	client := openai.New(cfg.Provider.OpenAI)
	client.SetTokenResolver(func(ctx context.Context) (string, error) {
		_ = ctx
		state, loadErr := auth.Load(cwd)
		if loadErr != nil || state == nil {
			return "", loadErr
		}
		return state.AccessToken, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Provider.OpenAI.TimeoutSeconds)*time.Second)
	defer cancel()

	resp, chatErr := client.Chat(ctx, providers.ChatRequest{
		Role:         providers.RoleCoder,
		Model:        cfg.Provider.OpenAI.Models.Coder,
		SystemPrompt: "You are Orch interactive assistant. Be concise and practical.",
		UserPrompt:   prompt,
	})
	if chatErr != nil {
		return nil, fmt.Errorf("provider chat failed: %w", chatErr)
	}
	if strings.TrimSpace(resp.Text) == "" {
		return nil, fmt.Errorf("provider returned an empty response")
	}

	return &chatExecutionResult{Text: strings.TrimSpace(resp.Text)}, nil
}

func helpText() string {
	return strings.Join([]string{
		"Commands:",
		"  /chat <message>        Chat with Orch",
		"  /run <task>            Run full pipeline",
		"  /plan <task>           Generate plan only",
		"  /diff                  Show latest patch",
		"  /apply                 Apply latest patch (dry-run by default)",
		"  /doctor                Validate provider/runtime readiness",
		"  /provider              Show provider configuration",
		"  /provider set openai   Set default provider",
		"  /auth status            Show authentication status",
		"  /auth login --mode account --token <token>  Save account token",
		"  /auth login --mode api_key  Use API key mode",
		"  /auth logout            Remove stored account token",
		"  /model                 Show role model mapping",
		"  /models                Alias for /model",
		"  /model set <role> <model>  Set role model",
		"  /logs [run-id]         Show logs",
		"  /explain [run-id]      Explain a run using structured artifacts",
		"  /stats                 Show quality stats for recent runs",
		"  /session <subcommand>  Session operations",
		"  /init                  Initialize project",
		"  /verbose [on|off]      Toggle detailed run output",
		"  /clear                 Clear screen output",
		"  /exit                  Quit",
		"",
		"Tip: plain text input starts a chat message.",
		"Tip: use /run when you want code generation workflow.",
		"Tip: Shift+Enter or Ctrl+J inserts a newline in composer.",
	}, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func naturalRunReply(result *runExecutionResult) string {
	if result == nil || result.State == nil {
		return "Run could not be completed; details are in the result card below."
	}
	state := result.State
	if result.Err != nil || state.Status == models.StatusFailed {
		return "The task failed during execution; check details below."
	}

	if state.Patch == nil || len(state.Patch.Files) == 0 {
		if state.Review != nil && state.Review.Decision == models.ReviewAccept {
			return "Request completed and no code changes were required."
		}
		return "Request completed; see the run summary below."
	}

	fileCount := len(state.Patch.Files)
	if fileCount == 1 {
		return "Task completed; changes were prepared in 1 file."
	}
	return fmt.Sprintf("Task completed; changes were prepared in %d files.", fileCount)
}

func compactRunLines(result *runExecutionResult, verbose bool) []string {
	if result == nil {
		return []string{"run failed: no result"}
	}

	lines := make([]string, 0, 12)
	state := result.State
	if state == nil {
		return []string{"run failed: no state returned"}
	}

	lines = append(lines, dracula.accent.Render(fmt.Sprintf("Session %s | Project %s", result.SessionName, result.ProjectID)))

	providerSummary := providerSummaryFromLogs(state.Logs)
	if providerSummary != "" {
		lines = append(lines, dracula.accent.Render(providerSummary))
	}

	duration := "-"
	if state.CompletedAt != nil {
		duration = state.CompletedAt.Sub(state.StartedAt).Round(time.Millisecond).String()
	}

	if result.Err != nil {
		lines = append(lines, dracula.error.Render(fmt.Sprintf("Run failed (%s): %v", duration, result.Err)))
	} else {
		lines = append(lines, dracula.success.Render(fmt.Sprintf("Run completed (%s): %s", duration, state.Status)))
	}

	if state.Review != nil {
		reviewLine := fmt.Sprintf("Review: %s", state.Review.Decision)
		if len(state.Review.Comments) > 0 {
			reviewLine += " - " + state.Review.Comments[0]
		}
		lines = append(lines, reviewLine)
	}

	timeline := timelineFromLogs(state.Logs)
	for _, t := range timeline {
		lines = append(lines, dracula.timeline.Render(t))
	}

	if strings.TrimSpace(state.BestPatchSummary) != "" {
		lines = append(lines, "Patch: "+state.BestPatchSummary)
	}
	if strings.TrimSpace(state.TestResults) != "" {
		lines = append(lines, "Tests: completed")
	}

	lines = append(lines, fmt.Sprintf("Run ID: %s", state.ID))
	lines = append(lines, fmt.Sprintf("Log: .orch/runs/%s.json", state.ID))

	for _, warning := range result.Warnings {
		lines = append(lines, dracula.warning.Render("warning: "+warning))
	}

	if verbose {
		lines = append(lines, "--- details ---")
		for _, entry := range state.Logs {
			lines = append(lines, fmt.Sprintf("[%s] %s", entry.Actor, entry.Message))
		}
	}

	return lines
}

func providerSummaryFromLogs(entries []models.LogEntry) string {
	provider := ""
	for _, entry := range entries {
		if entry.Actor != "provider" {
			continue
		}
		if entry.Step == "status" {
			provider = entry.Message
			break
		}
	}
	if strings.TrimSpace(provider) == "" {
		return ""
	}
	return "Provider: " + provider
}

func timelineFromLogs(entries []models.LogEntry) []string {
	if len(entries) == 0 {
		return nil
	}

	stages := []struct {
		actor string
		label string
	}{
		{actor: "analyzer", label: "Analyze"},
		{actor: "planner", label: "Plan"},
		{actor: "coder", label: "Build"},
		{actor: "test", label: "Test"},
		{actor: "reviewer", label: "Review"},
	}

	lines := make([]string, 0, len(stages))
	for _, stage := range stages {
		start := findLogTime(entries, stage.actor)
		if start.IsZero() {
			continue
		}
		end := findNextTime(entries, start)
		delta := "-"
		if !end.IsZero() {
			delta = end.Sub(start).Round(time.Millisecond).String()
		}
		lines = append(lines, fmt.Sprintf("• %s  %s", stage.label, delta))
	}

	return lines
}

func findLogTime(entries []models.LogEntry, actor string) time.Time {
	for _, e := range entries {
		if e.Actor == actor {
			return e.Timestamp
		}
	}
	return time.Time{}
}

func findNextTime(entries []models.LogEntry, current time.Time) time.Time {
	for _, e := range entries {
		if e.Timestamp.After(current) {
			return e.Timestamp
		}
	}
	return time.Time{}
}

func shortModelsLine(modelsLine string) string {
	line := strings.TrimSpace(modelsLine)
	if line == "" {
		return "models: -"
	}
	line = strings.TrimPrefix(line, "Models: ")
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return "models: -"
	}

	val := map[string]string{}
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}
		val[kv[0]] = kv[1]
	}

	planner := val["planner"]
	coder := val["coder"]
	reviewer := val["reviewer"]
	if planner != "" && planner == coder && planner == reviewer {
		return "model " + planner
	}
	if planner == "" {
		planner = "-"
	}
	if coder == "" {
		coder = "-"
	}
	if reviewer == "" {
		reviewer = "-"
	}
	return fmt.Sprintf("models p:%s c:%s r:%s", planner, coder, reviewer)
}

func readRuntimeStatus() (providerLine, authLine, modelsLine string) {
	providerLine = "Provider: unknown"
	authLine = "Auth: disconnected"
	modelsLine = "Models: planner=- coder=- reviewer=-"

	cwd, err := os.Getwd()
	if err != nil {
		return providerLine, authLine, modelsLine
	}

	cfg, err := config.Load(cwd)
	if err != nil {
		return providerLine, authLine, modelsLine
	}

	providerLine = fmt.Sprintf("Provider: %s", cfg.Provider.Default)
	modelsLine = fmt.Sprintf("Models: planner=%s coder=%s reviewer=%s", cfg.Provider.OpenAI.Models.Planner, cfg.Provider.OpenAI.Models.Coder, cfg.Provider.OpenAI.Models.Reviewer)

	authMode := strings.ToLower(strings.TrimSpace(cfg.Provider.OpenAI.AuthMode))
	if authMode == "" {
		authMode = "api_key"
	}

	switch authMode {
	case "api_key":
		connected := strings.TrimSpace(os.Getenv(cfg.Provider.OpenAI.APIKeyEnv)) != ""
		if connected {
			authLine = "Auth: connected (api_key)"
		} else {
			authLine = "Auth: disconnected (api_key)"
		}
	case "account":
		if strings.TrimSpace(os.Getenv(cfg.Provider.OpenAI.AccountTokenEnv)) != "" {
			authLine = "Auth: connected (account env)"
			break
		}
		state, loadErr := auth.Load(cwd)
		if loadErr == nil && state != nil && strings.TrimSpace(state.AccessToken) != "" {
			authLine = "Auth: connected (account local)"
		} else {
			authLine = "Auth: disconnected (account)"
		}
	default:
		authLine = fmt.Sprintf("Auth: invalid mode (%s)", authMode)
	}

	return providerLine, authLine, modelsLine
}

func generateSessionID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("ses_%d", time.Now().UnixNano())
	}
	return "ses_" + hex.EncodeToString(buf)
}
