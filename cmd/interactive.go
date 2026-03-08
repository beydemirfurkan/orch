package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type interactiveModel struct {
	viewport viewport.Model
	input    textinput.Model
	spinner  spinner.Model

	logs    []string
	running bool
	width   int
	height  int
}

type commandResultMsg struct {
	command string
	output  string
	err     error
}

func startInteractiveShell() error {
	m := newInteractiveModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newInteractiveModel() interactiveModel {
	input := textinput.New()
	input.Placeholder = "Type a task or command (/help)"
	input.Prompt = "orch> "
	input.Focus()
	input.CharLimit = 0

	sp := spinner.New()
	sp.Spinner = spinner.Line

	vp := viewport.New(80, 20)

	lines := []string{
		"Orch interactive mode",
		"Type plain text to run a task.",
		"Use /help to list commands.",
	}
	vp.SetContent(strings.Join(lines, "\n"))

	return interactiveModel{
		viewport: vp,
		input:    input,
		spinner:  sp,
		logs:     lines,
	}
}

func (m interactiveModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m interactiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := 3
		inputHeight := 3
		m.viewport.Width = msg.Width - 2
		m.viewport.Height = max(5, msg.Height-headerHeight-inputHeight)
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
		m.appendLog(fmt.Sprintf("$ %s", msg.command))
		if strings.TrimSpace(msg.output) != "" {
			m.appendLog(strings.TrimRight(msg.output, "\n"))
		}
		if msg.err != nil {
			m.appendLog(fmt.Sprintf("error: %v", msg.err))
		}
		m.appendLog("")
		m.viewport.GotoBottom()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
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
				m.appendLog(helpText())
				m.appendLog("")
				m.viewport.GotoBottom()
				return m, nil
			}

			args, err := parseInteractiveInput(raw)
			if err != nil {
				m.appendLog(fmt.Sprintf("error: %v", err))
				m.appendLog("")
				m.viewport.GotoBottom()
				return m, nil
			}

			m.running = true
			cmds = append(cmds, m.spinner.Tick, runCLICommandCmd(args))
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
	if m.running {
		status = m.spinner.View() + " running"
	}

	header := lipgloss.NewStyle().Bold(true).Render("Orch") + "  " + status + "\n"
	header += lipgloss.NewStyle().Faint(true).Render("/help /clear /exit | plain text runs `orch run \"...\"`") + "\n"

	return header + m.viewport.View() + "\n" + m.input.View()
}

func (m *interactiveModel) appendLog(line string) {
	m.logs = append(m.logs, line)
	m.viewport.SetContent(strings.Join(m.logs, "\n"))
}

func parseInteractiveInput(input string) ([]string, error) {
	if strings.HasPrefix(input, "/") {
		parts := strings.Fields(strings.TrimPrefix(input, "/"))
		if len(parts) == 0 {
			return nil, fmt.Errorf("empty command")
		}

		switch parts[0] {
		case "run", "plan":
			if len(parts) < 2 {
				return nil, fmt.Errorf("/%s requires a task", parts[0])
			}
			return []string{parts[0], strings.Join(parts[1:], " ")}, nil
		case "diff", "apply", "init":
			return []string{parts[0]}, nil
		case "logs":
			return append([]string{"logs"}, parts[1:]...), nil
		case "session":
			return append([]string{"session"}, parts[1:]...), nil
		default:
			return nil, fmt.Errorf("unknown command: %s", parts[0])
		}
	}

	return []string{"run", input}, nil
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

func helpText() string {
	return strings.Join([]string{
		"Commands:",
		"  /run <task>            Run full pipeline",
		"  /plan <task>           Generate plan only",
		"  /diff                  Show latest patch",
		"  /apply                 Apply latest patch (dry-run by default)",
		"  /logs [run-id]         Show logs",
		"  /session <subcommand>  Session operations",
		"  /init                  Initialize project",
		"  /clear                 Clear screen output",
		"  /exit                  Quit",
		"",
		"Tip: plain text input runs `orch run \"<text>\"`.",
	}, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
