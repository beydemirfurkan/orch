package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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
	"github.com/furkanbeydemir/orch/internal/session"
	"github.com/furkanbeydemir/orch/internal/storage"
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
	cwd          string

	showSuggestions bool
	suggestions     []commandEntry
	suggestionIdx   int

	// Pipeline stage tracking
	pipelineStage string
	activeAgent   string
	lastRunCost   string

	// New modal selection state
	activeModal *modalState
}

type modalType int

const (
	modalNone modalType = iota
	modalProvider
	modalAuth
)

type modalState struct {
	Type     modalType
	Title    string
	Choices  []choiceEntry
	Index    int
	Selected string // The value selected in the previous step (e.g. chosen provider)
}

type choiceEntry struct {
	ID   string
	Text string
	Sub  string
}

var providersList = []choiceEntry{
	{ID: "openai", Text: "OpenAI", Sub: "(ChatGPT Plus/Pro or API key)"},
	{ID: "github", Text: "GitHub Copilot", Sub: ""},
	{ID: "anthropic", Text: "Anthropic", Sub: "(Claude Max or API key)"},
	{ID: "google", Text: "Google", Sub: ""},
}

var authMethods = map[string][]choiceEntry{
	"openai": {
		{ID: "browser", Text: "ChatGPT Pro/Plus (browser)", Sub: ""},
		{ID: "headless", Text: "ChatGPT Pro/Plus (headless)", Sub: ""},
		{ID: "api_key", Text: "Manually enter API Key", Sub: ""},
	},
}

type commandEntry struct {
	Name string
	Desc string
}

var allCommands = []commandEntry{
	{Name: "/agents", Desc: "List active agents, models, and skills"},
	{Name: "/auth", Desc: "Login/Logout from provider"},
	{Name: "/connect", Desc: "Connect provider credentials"},
	{Name: "/clear", Desc: "Clear chat history"},
	{Name: "/cost", Desc: "Show token usage and estimated cost for recent runs"},
	{Name: "/exit", Desc: "Exit the app"},
	{Name: "/help", Desc: "Show help messages"},
	{Name: "/init", Desc: "Initialize or update project config"},
	{Name: "/model", Desc: "Switch active model"},
	{Name: "/plan", Desc: "Plan a complex task"},
	{Name: "/provider", Desc: "Select or switch provider"},
	{Name: "/run", Desc: "Execute a task with agents"},
	{Name: "/session", Desc: "Manage chat sessions"},
	{Name: "/stats", Desc: "Show usage statistics"},
	{Name: "/verbose", Desc: "Toggle verbose output (on/off)"},
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
	noteCard    lipgloss.Style
	errorCard   lipgloss.Style
	footer      lipgloss.Style

	menuBox      lipgloss.Style
	menuItem     lipgloss.Style
	menuSelected lipgloss.Style
	menuDesc     lipgloss.Style

	modalBox    lipgloss.Style
	modalTitle  lipgloss.Style
	modalKey    lipgloss.Style
	modalSearch lipgloss.Style
}

var dracula = theme{
	header:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E2E8F0")),
	accent:      lipgloss.NewStyle().Foreground(lipgloss.Color("#7DD3FC")),
	muted:       lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B")),
	success:     lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")),
	warning:     lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")),
	error:       lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171")),
	panel:       lipgloss.NewStyle().Padding(0, 1),
	command:     lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0")),
	timeline:    lipgloss.NewStyle().Foreground(lipgloss.Color("#93C5FD")),
	statusRun:   lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true),
	statusIdle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Bold(true),
	chip:        lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Background(lipgloss.Color("#0F172A")).Padding(0, 1),
	chipMuted:   lipgloss.NewStyle().Foreground(lipgloss.Color("#475569")).Background(lipgloss.Color("#0B1220")).Padding(0, 1),
	composerTag: lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8")).Bold(true),
	userCard: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E2E8F0")).
		Padding(0, 0, 0, 1).
		MarginBottom(1),
	assistant: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#94A3B8")).
		Padding(0, 0, 0, 1).
		MarginBottom(1),
	noteCard: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#CBD5E1")).
		Padding(0, 0, 0, 1).
		MarginBottom(1),
	errorCard: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FECACA")).
		Padding(0, 0, 0, 1).
		MarginBottom(1),
	footer: lipgloss.NewStyle().Foreground(lipgloss.Color("#475569")),
	menuBox: lipgloss.NewStyle().
		Background(lipgloss.Color("#1E1E2E")).
		Padding(0, 1).
		MarginBottom(0),
	menuItem: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8FAFC")).
		Bold(true),
	menuSelected: lipgloss.NewStyle().
		Background(lipgloss.Color("#F97316")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true),
	menuDesc: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#94A3B8")),
	modalBox: lipgloss.NewStyle().
		Background(lipgloss.Color("#111827")).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(1, 2),
	modalTitle: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8FAFC")).
		Bold(true),
	modalKey: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#64748B")).
		Italic(true),
	modalSearch: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F97316")),
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
	displayPrompt string
	inputNote     string
	result        *chatExecutionResult
	err           error
}

func startInteractiveShell(resumeID string) error {
	m := newInteractiveModel(resumeID)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newInteractiveModel(resumeID string) *interactiveModel {
	input := textarea.New()
	input.Placeholder = "Ask Orch anything..."
	input.Prompt = ""

	input.FocusedStyle.CursorLine = lipgloss.NewStyle()
	input.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	input.FocusedStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#F8FAFC"))
	input.FocusedStyle.Prompt = lipgloss.NewStyle()
	input.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0"))
	input.CharLimit = 0
	input.ShowLineNumbers = false
	input.SetHeight(2)
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

	cwd, _ := getWorkingDirectory()

	// Initialize with empty logs.
	lines := []string{}
	vp.SetContent("")

	providerLine, authLine, modelsLine := readRuntimeStatus()

	return &interactiveModel{
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
		cwd:          cwd,
	}
}

func (m *interactiveModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink)
}

func (m *interactiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := 2
		inputHeight := 5
		footerHeight := 3
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = max(5, msg.Height-headerHeight-inputHeight-footerHeight)

		contentWidth := max(40, min(80, m.viewport.Width))
		m.input.SetWidth(contentWidth)
		m.input.SetHeight(max(2, inputHeight-2))
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
			m.appendAssistantMessage("Orch", strings.Split(strings.TrimRight(msg.output, "\n"), "\n"))
		}
		if msg.err != nil {
			m.appendErrorMessage(fmt.Sprintf("error: %v", msg.err))
		}
		m.appendSpacer()
		m.viewport.GotoBottom()
		return m, nil

	case runExecutionMsg:
		m.running = false
		m.activeAgent = ""
		m.pipelineStage = ""
		m.providerLine, m.authLine, m.modelsLine = readRuntimeStatus()
		m.appendUserMessage(msg.command)
		if msg.err != nil {
			m.appendErrorMessage(fmt.Sprintf("error: %v", msg.err))
			m.appendSpacer()
			m.viewport.GotoBottom()
			return m, nil
		}
		if msg.result != nil && msg.result.State != nil {
			m.lastRunCost = formatRunCost(msg.result.State)
		}
		m.appendAssistantMessage("Orch", []string{naturalRunReply(msg.result)})
		m.appendAssistantMessage("Run Result", compactRunLines(msg.result, m.verboseMode))
		m.appendSpacer()
		m.viewport.GotoBottom()
		return m, nil

	case chatExecutionMsg:
		m.running = false
		m.providerLine, m.authLine, m.modelsLine = readRuntimeStatus()
		m.appendUserMessage(msg.displayPrompt)
		if strings.TrimSpace(msg.inputNote) != "" {
			m.appendNoteMessage("Input Transform", []string{msg.inputNote})
		}
		if msg.err != nil {
			m.appendErrorMessage(fmt.Sprintf("error: %v", msg.err))
			m.appendSpacer()
			m.viewport.GotoBottom()
			return m, nil
		}
		if msg.result != nil {
			m.appendAssistantMessage("Orch", strings.Split(strings.TrimSpace(msg.result.Text), "\n"))
			if strings.TrimSpace(msg.result.Warning) != "" {
				m.appendNoteMessage("Note", []string{msg.result.Warning})
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
			if m.activeModal != nil {
				m.activeModal = nil
				return m, nil
			}
			if m.showSuggestions {
				m.showSuggestions = false
				return m, nil
			}
			if strings.TrimSpace(m.input.Value()) != "" {
				m.input.SetValue("")
			}
			return m, nil
		case "up":
			if m.activeModal != nil {
				m.activeModal.Index = (m.activeModal.Index - 1 + len(m.activeModal.Choices)) % len(m.activeModal.Choices)
				return m, nil
			}
			if m.showSuggestions {
				m.suggestionIdx = (m.suggestionIdx - 1 + len(m.suggestions)) % len(m.suggestions)
				return m, nil
			}
		case "down":
			if m.activeModal != nil {
				m.activeModal.Index = (m.activeModal.Index + 1) % len(m.activeModal.Choices)
				return m, nil
			}
			if m.showSuggestions {
				m.suggestionIdx = (m.suggestionIdx + 1) % len(m.suggestions)
				return m, nil
			}
		case "tab":
			if m.showSuggestions && len(m.suggestions) > 0 {
				m.input.SetValue(m.suggestions[m.suggestionIdx].Name + " ")
				m.input.SetCursor(len(m.input.Value()))
				m.showSuggestions = false
				return m, nil
			}
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
			if m.activeModal != nil {
				active := m.activeModal
				choice := active.Choices[active.Index]

				if active.Type == modalProvider {
					// Move to auth step
					methods, ok := authMethods[choice.ID]
					if !ok {
						// Simple selection if no methods defined (future proofing)
						m.activeModal = nil
						m.input.SetValue("/provider " + choice.ID)
						return m.handleCommand()
					}
					m.activeModal = &modalState{
						Type:     modalAuth,
						Title:    "Select auth method",
						Choices:  methods,
						Selected: choice.ID,
					}
					return m, nil
				} else if active.Type == modalAuth {
					// Final selection
					provider := active.Selected
					method := choice.ID
					m.activeModal = nil

					if method == "browser" || method == "headless" {
						m.input.SetValue(fmt.Sprintf("/auth login --provider %s --method account --flow %s", provider, method))
					} else if method == "api_key" {
						m.input.SetValue(fmt.Sprintf("/auth login --provider %s --method api", provider))
					} else {
						m.input.SetValue(fmt.Sprintf("/auth login --provider %s --method %s", provider, method))
					}
					return m.handleCommand()
				}
				return m, nil
			}
			if m.showSuggestions && len(m.suggestions) > 0 {
				m.input.SetValue(m.suggestions[m.suggestionIdx].Name + " ")
				m.input.SetCursor(len(m.input.Value()))
				m.showSuggestions = false
				return m, nil
			}
			if m.running {
				return m, nil
			}

			raw := strings.TrimSpace(m.input.Value())
			if raw == "" {
				return m, nil
			}
			// handleCommand will clear the input
			return m.handleCommand()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	// Update suggestions
	val := m.input.Value()
	if strings.HasPrefix(val, "/") && !strings.Contains(val, " ") {
		m.suggestions = nil
		for _, c := range allCommands {
			if strings.HasPrefix(c.Name, val) {
				m.suggestions = append(m.suggestions, c)
			}
		}
		if len(m.suggestions) > 0 {
			m.showSuggestions = true
			if m.suggestionIdx >= len(m.suggestions) {
				m.suggestionIdx = 0
			}
		} else {
			m.showSuggestions = false
		}
	} else {
		m.showSuggestions = false
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *interactiveModel) handleCommand() (tea.Model, tea.Cmd) {
	raw := strings.TrimSpace(m.input.Value())
	m.input.SetValue("")

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

	if strings.HasPrefix(raw, "/provider") || strings.HasPrefix(raw, "/connect") {
		parts := strings.Fields(raw)
		if len(parts) == 1 {
			// Trigger interactive modal
			m.activeModal = &modalState{
				Type:    modalProvider,
				Title:   "Connect a provider",
				Choices: providersList,
				Index:   0,
			}
			return m, nil
		}
	}

	if strings.HasPrefix(raw, "/auth") {
		parts := strings.Fields(raw)
		if len(parts) == 1 {
			// If we have an active provider already, we can skip to auth selection
			m.activeModal = &modalState{
				Type:    modalProvider,
				Title:   "Select a provider to authenticate",
				Choices: providersList,
				Index:   0,
			}
			return m, nil
		}
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

	if raw == "/cost" {
		lines := m.buildCostLines()
		m.appendAssistantMessage("Token Cost", lines)
		m.appendSpacer()
		m.viewport.GotoBottom()
		return m, nil
	}

	if raw == "/agents" {
		lines := m.buildAgentsLines()
		m.appendAssistantMessage("Agents", lines)
		m.appendSpacer()
		m.viewport.GotoBottom()
		return m, nil
	}

	dispatch, err := prepareInteractiveDispatch(raw)
	if err != nil {
		m.appendErrorMessage(fmt.Sprintf("error: %v", err))
		m.appendSpacer()
		m.viewport.GotoBottom()
		return m, nil
	}

	// If transitioning from empty state to active state, resize the input to correct content width
	if len(m.logs) == 0 {
		contentWidth := max(40, min(80, m.viewport.Width))
		m.input.SetWidth(contentWidth)
	}

	var cmds []tea.Cmd
	m.running = true
	if len(dispatch.Args) > 1 && dispatch.Args[0] == "run" {
		cmds = append(cmds, m.spinner.Tick, runInProcessCmd(dispatch.Args[1]))
	} else if len(dispatch.Args) > 1 && dispatch.Args[0] == "chat" {
		cmds = append(cmds, m.spinner.Tick, runInProcessChatCmd(dispatch.DisplayInput, dispatch.Args[1], dispatch.InputNote))
	} else {
		cmds = append(cmds, m.spinner.Tick, runCLICommandCmd(dispatch.Args))
	}
	return m, tea.Batch(cmds...)
}

func (m *interactiveModel) View() string {
	shellWidth := max(40, m.width)
	shellHeight := max(10, m.height)

	var bg string
	if len(m.logs) == 0 {
		bg = m.renderEmptyState(shellWidth, shellHeight)
	} else {
		providerState := "unknown"
		if !strings.Contains(strings.ToLower(m.providerLine), "inactive") && !strings.Contains(strings.ToLower(m.providerLine), "unknown") {
			providerState = "provider configured"
		}
		authState := "disconnected"
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(m.authLine)), "auth: connected") {
			authState = "auth configured"
		}
		modelSummary := shortModelsLine(m.modelsLine)

		contentWidth := max(60, min(80, m.viewport.Width))

		headerParts := fmt.Sprintf("%s • %s • %s", providerState, authState, modelSummary)
		if m.lastRunCost != "" {
			headerParts += " • " + m.lastRunCost
		}
		if m.pipelineStage != "" && m.running {
			headerParts += " • " + dracula.statusRun.Render(m.pipelineStage)
		}
		headerInfo := dracula.muted.Render(headerParts)
		header := lipgloss.PlaceHorizontal(m.viewport.Width, lipgloss.Right, headerInfo) + "\n"

		bodyContent := dracula.panel.Width(contentWidth).Render(m.viewport.View())
		body := lipgloss.PlaceHorizontal(m.viewport.Width, lipgloss.Center, bodyContent)

		var composerInner string
		if m.running {
			agentLabel := m.activeAgent
			if agentLabel == "" {
				agentLabel = "orch"
			}
			composerInner = dracula.statusRun.Render(m.spinner.View()+" "+agentLabel) + dracula.muted.Render(" — working...")
		} else {
			composerInner = m.input.View()
		}
		composerContent := dracula.panel.Width(contentWidth).Render(composerInner)

		var suggestionsView string
		if m.showSuggestions {
			var lines []string
			for i, s := range m.suggestions {
				nameStyle := dracula.menuItem
				if i == m.suggestionIdx {
					nameStyle = dracula.menuSelected
				}
				line := nameStyle.Render(fmt.Sprintf(" %-12s ", s.Name)) + " " + dracula.menuDesc.Render(s.Desc)
				lines = append(lines, line)
			}
			suggestionsContent := dracula.menuBox.Width(contentWidth - 2).Render(strings.Join(lines, "\n"))
			suggestionsView = lipgloss.PlaceHorizontal(m.viewport.Width, lipgloss.Center, dracula.panel.Width(contentWidth).Render(suggestionsContent)) + "\n"
		}

		composer := lipgloss.PlaceHorizontal(m.viewport.Width, lipgloss.Center, composerContent)
		bg = header + body + "\n" + suggestionsView + composer + "\n"
	}

	if m.activeModal != nil {
		modal := m.renderModal(shellWidth, shellHeight)
		// Instead of clearing the background, we can return the modal separately.
		// However, Bubble Tea's View() returns the single final string.
		// To truly "overlay", we should join the background with the modal.
		// But centered modals usually replace the view or use a layered approach.
		// For verification, returning just the modal centered should be visible.
		return lipgloss.Place(shellWidth, shellHeight, lipgloss.Center, lipgloss.Center, modal)
	}

	return bg
}
func (m *interactiveModel) renderEmptyState(width, height int) string {
	logo := `
 ██████╗ ██████╗  ██████╗██╗  ██╗
██╔═══██╗██╔══██╗██╔════╝██║  ██║
██║   ██║██████╔╝██║     ███████║
██║   ██║██╔══██╗██║     ██╔══██║
╚██████╔╝██║  ██║╚██████╗██║  ██║
 ╚═════╝ ╚═╝  ╚═╝ ╚═════╝╚═╝  ╚═╝`
	logoStr := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#94A3B8")).
		Render(logo)

	modelSummary := shortModelsLine(m.modelsLine)
	providerState := "unknown"
	if !strings.Contains(strings.ToLower(m.providerLine), "inactive") && !strings.Contains(strings.ToLower(m.providerLine), "unknown") {
		providerState = "provider configured"
	}
	authState := "disconnected"
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(m.authLine)), "auth: connected") {
		authState = "auth configured"
	}

	// Status line underneath the input
	statusStr := dracula.muted.Render(fmt.Sprintf("%s • %s", providerState, authState))
	statsStr := dracula.muted.Render(modelSummary)

	contentWidth := max(40, min(80, width))

	// The composer wrapping
	inputBox := dracula.panel.Width(contentWidth).Render(m.input.View())

	// Suggestions overlay
	var suggestionsView string
	if m.showSuggestions {
		var lines []string
		for i, s := range m.suggestions {
			name := s.Name
			desc := s.Desc
			nameStyle := dracula.menuItem
			if i == m.suggestionIdx {
				nameStyle = dracula.menuSelected
			}
			line := nameStyle.Render(fmt.Sprintf(" %-12s ", name)) + " " + dracula.menuDesc.Render(desc)
			lines = append(lines, line)
		}
		suggestionsContent := dracula.menuBox.Width(contentWidth - 2).Render(strings.Join(lines, "\n"))
		suggestionsView = dracula.panel.Width(contentWidth).Render(suggestionsContent)
	}

	helpLine := dracula.warning.Render("• Tip") + dracula.muted.Render(" Use /help for commands. Plain text for chat. /run for tasks.")

	// Assemble the center block
	centerItems := []string{
		logoStr,
		"\n",
	}
	if suggestionsView != "" {
		centerItems = append(centerItems, suggestionsView)
	}
	centerItems = append(centerItems,
		lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, inputBox),
		"\n",
		lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, statusStr+"  |  "+statsStr),
		"\n\n\n",
		lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, helpLine),
	)

	centerBlock := lipgloss.JoinVertical(lipgloss.Center, centerItems...)

	if m.activeModal != nil {
		modal := m.renderModal(width, height)
		// We can use lipgloss.Place to put the modal on top of the empty state
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, modal)
	}

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, centerBlock)
}

func (m *interactiveModel) renderModal(width, height int) string {
	modal := m.activeModal
	if modal == nil {
		return ""
	}

	modalWidth := max(40, min(60, width-10))

	title := dracula.modalTitle.Render(modal.Title)
	escLabel := dracula.modalKey.Render("esc")

	header := lipgloss.JoinHorizontal(lipgloss.Left,
		title,
		strings.Repeat(" ", max(0, modalWidth-lipgloss.Width(title)-lipgloss.Width(escLabel))),
		escLabel)

	search := "\n" + dracula.modalSearch.Render("S") + dracula.muted.Render("earch") + "\n"

	var lines []string
	lines = append(lines, header, search)

	for i, choice := range modal.Choices {
		text := choice.Text
		sub := choice.Sub

		var line string
		if i == modal.Index {
			content := text
			if sub != "" {
				content += " " + dracula.muted.Render(sub)
			}
			line = dracula.menuSelected.Width(modalWidth).Render(content)
		} else {
			content := text
			if sub != "" {
				content += " " + dracula.muted.Render(sub)
			}
			line = dracula.menuItem.Render(content)
		}
		lines = append(lines, line)
	}

	return dracula.modalBox.Width(modalWidth).Render(strings.Join(lines, "\n"))
}

func (m *interactiveModel) appendLog(line string) {
	m.logs = append(m.logs, line)
	m.viewport.SetContent(strings.Join(m.logs, "\n"))
	m.viewport.GotoBottom()
}

func (m *interactiveModel) appendSpacer() {
	m.appendLog("")
}

func (m *interactiveModel) appendUserMessage(command string) {
	// A simple cyan dot indicator for user messages
	indicator := dracula.accent.Render("● ")
	body := indicator + dracula.header.Render("You") + "\n" + dracula.command.Render(command)
	card := dracula.userCard.Width(m.cardWidth()).Render(body)
	m.appendLog(card)
}

func (m *interactiveModel) appendAssistantMessage(title string, lines []string) {
	indicator := dracula.muted.Render("○ ")

	header := ""
	if title == "Orch" || title == "Output" || title == "Commands" {
		header = dracula.header.Render("Orch Output") + "\n"
	} else {
		header = dracula.header.Render(title) + "\n"
	}

	m.appendLog(dracula.assistant.Width(m.cardWidth()).Render(indicator + header + strings.Join(lines, "\n")))
}

func (m *interactiveModel) appendNoteMessage(title string, lines []string) {
	body := make([]string, 0, len(lines)+1)
	body = append(body, dracula.warning.Render("Note")+"  "+dracula.header.Render(title))
	body = append(body, lines...)
	m.appendLog(dracula.noteCard.Width(m.cardWidth()).Render(strings.Join(body, "\n")))
}

func (m *interactiveModel) appendErrorMessage(message string) {
	body := dracula.error.Render("Error") + "\n" + message
	m.appendLog(dracula.errorCard.Width(m.cardWidth()).Render(body))
}

func (m interactiveModel) cardWidth() int {
	return m.viewport.Width - 4
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
		case "doctor", "provider", "model", "models", "auth", "connect":
			if parts[0] == "models" {
				parts[0] = "model"
			}
			if parts[0] == "connect" {
				parts[0] = "auth"
				parts = append([]string{"auth", "login"}, parts[1:]...)
				return parts, nil
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

func runInProcessChatCmd(displayPrompt, prompt, inputNote string) tea.Cmd {
	return func() tea.Msg {
		result, err := executeChatPrompt(prompt)
		return chatExecutionMsg{
			displayPrompt: displayPrompt,
			inputNote:     inputNote,
			result:        result,
			err:           err,
		}
	}
}

func executeChatPrompt(prompt string) (*chatExecutionResult, error) {
	cwd, err := getWorkingDirectory()
	if err != nil {
		return nil, err
	}

	sessionCtx, err := loadSessionContext(cwd)
	if err != nil {
		return nil, fmt.Errorf("session unavailable: %w", err)
	}
	defer sessionCtx.Store.Close()
	svc := session.NewService(sessionCtx.Store)
	compactionNote := ""

	cfg, err := config.Load(cwd)
	if err != nil {
		return nil, fmt.Errorf("provider unavailable: failed to load config")
	}

	if !cfg.Provider.Flags.OpenAIEnabled || strings.ToLower(strings.TrimSpace(cfg.Provider.Default)) != "openai" {
		return nil, fmt.Errorf("provider unavailable: OpenAI provider is disabled or not selected")
	}

	status := runtimeStatusSnapshot()
	if !status.providerConfigured {
		return nil, fmt.Errorf("provider unavailable: %s", status.providerLine)
	}
	if !status.authConnected {
		return nil, fmt.Errorf("provider unavailable: %s", status.authLine)
	}

	if compacted, note, compactErr := svc.MaybeCompact(sessionCtx.Session.ID, cfg.Provider.OpenAI.Models.Coder); compactErr == nil && compacted {
		compactionNote = note
	}

	userMsg, err := svc.AppendText(session.MessageInput{
		SessionID:  sessionCtx.Session.ID,
		Role:       "user",
		ProviderID: cfg.Provider.Default,
		ModelID:    cfg.Provider.OpenAI.Models.Coder,
		Text:       prompt,
	})
	if err != nil {
		return nil, fmt.Errorf("session write failed: %w", err)
	}

	client := openai.New(cfg.Provider.OpenAI)
	var accountSession *auth.AccountSession
	if strings.ToLower(strings.TrimSpace(cfg.Provider.OpenAI.AuthMode)) == "account" && strings.TrimSpace(os.Getenv(cfg.Provider.OpenAI.AccountTokenEnv)) == "" {
		accountSession = auth.NewAccountSession(cwd, "openai")
		client.SetAccountFailoverHandler(func(ctx context.Context, err error) (string, bool, error) {
			return accountSession.Failover(ctx, openai.AccountFailoverCooldown(err), err.Error())
		})
		client.SetAccountSuccessHandler(func(ctx context.Context) {
			accountSession.MarkSuccess(ctx)
		})
	}
	client.SetTokenResolver(func(ctx context.Context) (string, error) {
		mode := strings.ToLower(strings.TrimSpace(cfg.Provider.OpenAI.AuthMode))
		if mode == "api_key" {
			cred, credErr := auth.Get(cwd, "openai")
			if credErr != nil || cred == nil {
				return "", credErr
			}
			if strings.ToLower(strings.TrimSpace(cred.Type)) == "api" {
				return strings.TrimSpace(cred.Key), nil
			}
			return "", nil
		}
		if accountSession == nil {
			return "", nil
		}
		return accountSession.ResolveToken(ctx)
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
		errorPayload, _ := json.Marshal(map[string]string{"message": chatErr.Error()})
		_, _ = svc.AppendMessage(session.MessageInput{
			SessionID:    sessionCtx.Session.ID,
			Role:         "assistant",
			ParentID:     userMsg.Message.ID,
			ProviderID:   cfg.Provider.Default,
			ModelID:      cfg.Provider.OpenAI.Models.Coder,
			FinishReason: "error",
			Error:        chatErr.Error(),
		}, []storage.SessionPart{{Type: "error", Payload: string(errorPayload)}})
		return nil, fmt.Errorf("provider chat failed: %w", chatErr)
	}
	if strings.TrimSpace(resp.Text) == "" {
		errorPayload, _ := json.Marshal(map[string]string{"message": "provider returned an empty response"})
		_, _ = svc.AppendMessage(session.MessageInput{
			SessionID:    sessionCtx.Session.ID,
			Role:         "assistant",
			ParentID:     userMsg.Message.ID,
			ProviderID:   cfg.Provider.Default,
			ModelID:      cfg.Provider.OpenAI.Models.Coder,
			FinishReason: "error",
			Error:        "provider returned an empty response",
		}, []storage.SessionPart{{Type: "error", Payload: string(errorPayload)}})
		return nil, fmt.Errorf("provider returned an empty response")
	}

	assistantMsg, appendErr := svc.AppendText(session.MessageInput{
		SessionID:    sessionCtx.Session.ID,
		Role:         "assistant",
		ParentID:     userMsg.Message.ID,
		ProviderID:   cfg.Provider.Default,
		ModelID:      cfg.Provider.OpenAI.Models.Coder,
		FinishReason: "stop",
		Text:         strings.TrimSpace(resp.Text),
	})
	warning := ""
	if appendErr != nil {
		warning = fmt.Sprintf("session write warning: %v", appendErr)
	}
	if strings.TrimSpace(compactionNote) != "" {
		if warning == "" {
			warning = compactionNote
		} else {
			warning = warning + "; " + compactionNote
		}
	}
	if accountSession != nil {
		if notice := strings.TrimSpace(accountSession.ConsumeNotice()); notice != "" {
			if warning == "" {
				warning = notice
			} else {
				warning = warning + "; " + notice
			}
		}
	}
	if appendErr == nil {
		turnCount := 1
		if metrics, metricsErr := sessionCtx.Store.GetSessionMetrics(sessionCtx.Session.ID); metricsErr == nil && metrics != nil {
			turnCount = metrics.TurnCount + 1
		}
		_ = sessionCtx.Store.UpsertSessionMetrics(storage.SessionMetrics{
			SessionID:     sessionCtx.Session.ID,
			TurnCount:     turnCount,
			LastMessageID: assistantMsg.Message.ID,
		})
	}

	return &chatExecutionResult{Text: strings.TrimSpace(resp.Text), Warning: warning}, nil
}

func helpText() string {
	return strings.Join([]string{
		"Commands:",
		"  /chat <message>        Chat with Orch",
		"  /run <task>            Run full pipeline",
		"  /plan <task>           Generate plan only",
		"  ?quick <message>       Local concise chat transform",
		"  /diff                  Show latest patch",
		"  /apply                 Apply latest patch (dry-run by default)",
		"  /doctor                Validate provider/runtime readiness",
		"  /provider              Show provider configuration",
		"  /connect               Open provider auth flow",
		"  /provider set openai   Set default provider",
		"  /auth status            Show authentication status",
		"  /auth login [provider] --method account|api --flow auto|browser|headless",
		"  /auth list              List stored credentials",
		"  /auth logout [provider] Remove stored credential",
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
		"Tip: use ?quick when you want a concise local prompt transform without another LLM hop.",
		"Tip: Shift+Enter or Ctrl+J inserts a newline in composer.",
	}, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func shortInteractivePath(path string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	if trimmed == "" {
		return "."
	}
	parts := strings.Split(trimmed, "/")
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		filtered = append(filtered, part)
	}
	if len(filtered) <= 3 {
		return trimmed
	}
	return ".../" + strings.Join(filtered[len(filtered)-3:], "/")
}

// formatRunCost formats the total token usage of a run into a compact display string.
func formatRunCost(state *models.RunState) string {
	if state == nil || len(state.TokenUsages) == 0 {
		return ""
	}
	var totalIn, totalOut int
	var totalCost float64
	for _, u := range state.TokenUsages {
		totalIn += u.InputTokens
		totalOut += u.OutputTokens
		totalCost += u.EstimatedCost
	}
	if totalIn+totalOut == 0 {
		return ""
	}
	return fmt.Sprintf("↑%s/↓%s tokens ~$%.4f", formatInt(totalIn), formatInt(totalOut), totalCost)
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

type runtimeStatus struct {
	providerLine       string
	authLine           string
	modelsLine         string
	providerConfigured bool
	authConnected      bool
}

func runtimeStatusSnapshot() runtimeStatus {
	status := runtimeStatus{
		providerLine:       "Provider: unknown",
		authLine:           "Auth: disconnected",
		modelsLine:         "Models: planner=- coder=- reviewer=-",
		providerConfigured: false,
		authConnected:      false,
	}

	cwd, err := os.Getwd()
	if err != nil {
		return status
	}

	providerState, err := providers.ReadState(cwd)
	if err != nil {
		return status
	}

	cfg, err := config.Load(cwd)
	if err != nil {
		return status
	}

	status.providerLine = fmt.Sprintf("Provider: %s", cfg.Provider.Default)
	status.modelsLine = fmt.Sprintf("Models: planner=%s coder=%s reviewer=%s", cfg.Provider.OpenAI.Models.Planner, cfg.Provider.OpenAI.Models.Coder, cfg.Provider.OpenAI.Models.Reviewer)
	status.providerConfigured = cfg.Provider.Flags.OpenAIEnabled && strings.ToLower(strings.TrimSpace(cfg.Provider.Default)) == "openai"
	status.authConnected = providerState.OpenAI.Connected
	if providerState.OpenAI.Connected {
		status.authLine = fmt.Sprintf("Auth: connected (%s %s)", providerState.OpenAI.Mode, providerState.OpenAI.Source)
	} else {
		mode := providerState.OpenAI.Mode
		if mode == "" {
			mode = "unknown"
		}
		if strings.TrimSpace(providerState.OpenAI.Reason) != "" {
			status.authLine = fmt.Sprintf("Auth: disconnected (%s) - %s", mode, providerState.OpenAI.Reason)
		} else {
			status.authLine = fmt.Sprintf("Auth: disconnected (%s)", mode)
		}
	}

	return status
}

func readRuntimeStatus() (providerLine, authLine, modelsLine string) {
	status := runtimeStatusSnapshot()
	return status.providerLine, status.authLine, status.modelsLine
}

// buildCostLines queries storage for the last 20 runs and returns a per-run cost table.
func (m *interactiveModel) buildCostLines() []string {
	cwd := m.cwd
	if cwd == "" {
		var err error
		cwd, err = getWorkingDirectory()
		if err != nil {
			return []string{"error: could not determine working directory"}
		}
	}
	ctx, err := loadSessionContext(cwd)
	if err != nil {
		return []string{fmt.Sprintf("error loading storage: %v", err)}
	}
	defer ctx.Store.Close()

	states, err := ctx.Store.ListRunStatesByProject(ctx.ProjectID, 20)
	if err != nil {
		return []string{fmt.Sprintf("error reading runs: %v", err)}
	}
	if len(states) == 0 {
		return []string{"No runs found. Execute a task with /run first."}
	}

	lines := []string{}
	var grandIn, grandOut int
	var grandCost float64
	hasAny := false

	for _, state := range states {
		if state == nil || len(state.TokenUsages) == 0 {
			continue
		}
		hasAny = true
		var in, out int
		var cost float64
		for _, u := range state.TokenUsages {
			in += u.InputTokens
			out += u.OutputTokens
			cost += u.EstimatedCost
		}
		grandIn += in
		grandOut += out
		grandCost += cost
		shortID := state.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		lines = append(lines, fmt.Sprintf("  %s  in:%-7s out:%-7s ~$%.4f  [%s]",
			shortID,
			formatInt(in),
			formatInt(out),
			cost,
			strings.ToLower(string(state.Status)),
		))
	}

	if !hasAny {
		return []string{"No token usage data yet. Token tracking requires model calls to complete."}
	}

	lines = append(lines, "  ─────────────────────────────────────────────")
	lines = append(lines, fmt.Sprintf("  TOTAL          in:%-7s out:%-7s ~$%.4f",
		formatInt(grandIn), formatInt(grandOut), grandCost))
	return lines
}

// buildAgentsLines returns a summary of each agent, its model, token budget, and skills.
func (m *interactiveModel) buildAgentsLines() []string {
	cwd := m.cwd
	if cwd == "" {
		var err error
		cwd, err = getWorkingDirectory()
		if err != nil {
			return []string{"error: could not determine working directory"}
		}
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		return []string{fmt.Sprintf("error loading config: %v", err)}
	}

	roleModel := func(role string) string {
		if v, ok := cfg.Provider.RoleAssignments[role]; ok && v != "" {
			return v
		}
		switch role {
		case "planner":
			return cfg.Provider.OpenAI.Models.Planner
		case "coder":
			return cfg.Provider.OpenAI.Models.Coder
		case "reviewer":
			return cfg.Provider.OpenAI.Models.Reviewer
		case "explorer":
			if cfg.Provider.OpenAI.Models.Explorer != "" {
				return cfg.Provider.OpenAI.Models.Explorer
			}
			return "(default)"
		case "oracle":
			if cfg.Provider.OpenAI.Models.Oracle != "" {
				return cfg.Provider.OpenAI.Models.Oracle
			}
			return "(default)"
		case "fixer":
			if cfg.Provider.OpenAI.Models.Fixer != "" {
				return cfg.Provider.OpenAI.Models.Fixer
			}
			return "(default)"
		}
		return "unknown"
	}

	agentSkills := func(name string) string {
		global := cfg.Skills.GlobalSkills
		local := cfg.Skills.AgentSkills[name]
		all := append(global, local...)
		if len(all) == 0 {
			return "none"
		}
		return strings.Join(all, ", ")
	}

	type agentRow struct {
		name    string
		model   string
		budget  int
		enabled bool
	}

	rows := []agentRow{
		{"planner", roleModel("planner"), cfg.Budget.PlannerMaxTokens, true},
		{"coder", roleModel("coder"), cfg.Budget.CoderMaxTokens, true},
		{"reviewer", roleModel("reviewer"), cfg.Budget.ReviewerMaxTokens, true},
		{"explorer", roleModel("explorer"), 4096, cfg.Safety.FeatureFlags.ExplorerEnabled},
		{"oracle", roleModel("oracle"), 4096, cfg.Safety.FeatureFlags.OracleEnabled},
		{"fixer", roleModel("fixer"), cfg.Budget.FixerMaxTokens, cfg.Safety.FeatureFlags.FixerEnabled},
	}

	lines := []string{}
	for _, r := range rows {
		status := "enabled"
		if !r.enabled {
			status = "disabled"
		}
		skills := agentSkills(r.name)
		lines = append(lines, fmt.Sprintf("  %-10s  model:%-30s  budget:%-6d  skills:%-20s  [%s]",
			r.name, r.model, r.budget, skills, status))
	}
	return lines
}

func generateSessionID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("ses_%d", time.Now().UnixNano())
	}
	return "ses_" + hex.EncodeToString(buf)
}
