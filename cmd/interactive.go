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
	cwd          string
	
	showSuggestions bool
	suggestions     []commandEntry
	suggestionIdx   int

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
	{Name: "/agents", Desc: "Switch agent"},
	{Name: "/auth", Desc: "Login/Logout from provider"},
	{Name: "/clear", Desc: "Clear chat history"},
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

	modalBox     lipgloss.Style
	modalTitle   lipgloss.Style
	modalKey     lipgloss.Style
	modalSearch  lipgloss.Style
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
					
					if method == "browser" {
						m.input.SetValue(fmt.Sprintf("/auth %s login", provider))
					} else if method == "api_key" {
						m.input.SetValue(fmt.Sprintf("/auth %s key", provider))
					} else {
						m.input.SetValue(fmt.Sprintf("/auth %s %s", provider, method))
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

	if strings.HasPrefix(raw, "/provider") {
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
		if strings.Contains(strings.ToLower(m.authLine), "connected") {
			authState = "auth configured"
		}
		modelSummary := shortModelsLine(m.modelsLine)

		contentWidth := max(60, min(80, m.viewport.Width))

		headerInfo := dracula.muted.Render(fmt.Sprintf("%s • %s • %s", providerState, authState, modelSummary))
		header := lipgloss.PlaceHorizontal(m.viewport.Width, lipgloss.Right, headerInfo) + "\n"

		bodyContent := dracula.panel.Width(contentWidth).Render(m.viewport.View())
		body := lipgloss.PlaceHorizontal(m.viewport.Width, lipgloss.Center, bodyContent)

		composerContent := dracula.panel.Width(contentWidth).Render(m.input.View())
		
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
	if strings.Contains(strings.ToLower(m.authLine), "connected") {
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
		"  ?quick <message>       Local concise chat transform",
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
