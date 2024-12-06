package start

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/yourusername/yourproject/cmd/start/widgets"
)

type Command struct {
	Name        string
	Subcommands []string
	Description string
}

var availableCommands = []Command{
	{
		Name:        "clear",
		Description: "Clear all output",
	},
	{
		Name:        "help",
		Description: "Show help information",
	},
	{
		Name: "restart",
		Subcommands: []string{
			"server",
			"client",
			"all",
		},
		Description: "Restart processes (server|client|all)",
	},
	{
		Name:        "status",
		Description: "Show process status",
	},
}

import (
	"github.com/yourusername/yourproject/cmd/start/widgets"
)

type model struct {
	serverPane   widgets.ServerPane
	clientPane   widgets.ClientPane
	commandLine  widgets.CommandLine
	helpWindow   widgets.HelpWindow
	selectedPane string // "server" or "client"
	serverCmd    *exec.Cmd
	clientCmd    *exec.Cmd
	serverOutput strings.Builder
	clientOutput strings.Builder
	err          error
}

func initialModel() model {
	return model{
		serverPane:   widgets.NewServerPane(),
		clientPane:   widgets.NewClientPane(),
		commandLine:  widgets.NewCommandLine(),
		helpWindow:   widgets.NewHelpWindow(),
		selectedPane: "server",
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.startProcesses,
	)
}

func (m *model) startProcesses() tea.Msg {
	// Start server process
	m.serverCmd = exec.Command("mmdev", "server", "start", "--watch")
	serverOut, err := m.serverCmd.StdoutPipe()
	if err != nil {
		return errMsg{err}
	}
	m.serverCmd.Stderr = m.serverCmd.Stdout

	// Start client process
	m.clientCmd = exec.Command("mmdev", "client", "start", "--watch")
	clientOut, err := m.clientCmd.StdoutPipe()
	if err != nil {
		return errMsg{err}
	}
	m.clientCmd.Stderr = m.clientCmd.Stdout

	if err := m.serverCmd.Start(); err != nil {
		return errMsg{err}
	}
	m.serverRunning = true

	if err := m.clientCmd.Start(); err != nil {
		m.serverCmd.Process.Kill()
		m.serverRunning = false
		return errMsg{err}
	}
	m.clientRunning = true

	// Handle output in goroutines
	go m.handleOutput(serverOut, &m.serverOutput, "server")
	go m.handleOutput(clientOut, &m.clientOutput, "client")

	return nil
}

type (
	errMsg    struct{ error }
	outputMsg struct {
		text string
		src  string
	}
)

func (m *model) handleOutput(reader io.Reader, buffer *strings.Builder, source string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		text := scanner.Text() + "\n"
		buffer.WriteString(text)
		if program != nil {
			program.Send(outputMsg{text: buffer.String(), src: source})
		}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if msg.Y < m.serverView.Height {
				m.serverView.LineUp(3)
			} else {
				m.clientView.LineUp(3)
			}
		case tea.MouseButtonWheelDown:
			if msg.Y < m.serverView.Height {
				m.serverView.LineDown(3)
			} else {
				m.clientView.LineDown(3)
			}
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.showHelp {
				m.showHelp = !m.showHelp
			} else {
				return m, m.shutdown
			}
		case "?", "h":
			m.showHelp = !m.showHelp
			return m, nil
		case "ctrl+l":
			m.serverOutput.Reset()
			m.clientOutput.Reset()
			return m, func() tea.Msg {
				return outputMsg{text: "", src: "both"}
			}
		case "enter":
			if m.cmdInput.Focused() {
				cmd := m.cmdInput.Value()
				m.cmdInput.SetValue("")
				return m, m.executeCommand(cmd)
			}
		case ":":
			m.cmdInput.Focus()
			return m, textinput.Blink
		case "up":
			if m.cmdInput.Focused() && len(m.cmdHistory) > 0 {
				if m.cmdHistoryPos < len(m.cmdHistory)-1 {
					m.cmdHistoryPos++
					m.cmdInput.SetValue(m.cmdHistory[len(m.cmdHistory)-1-m.cmdHistoryPos])
				}
			}
			return m, nil
		case "down":
			if m.cmdInput.Focused() {
				if m.cmdHistoryPos > 0 {
					m.cmdHistoryPos--
					m.cmdInput.SetValue(m.cmdHistory[len(m.cmdHistory)-1-m.cmdHistoryPos])
				} else if m.cmdHistoryPos == 0 {
					m.cmdHistoryPos--
					m.cmdInput.SetValue("")
				}
			}
			return m, nil
		case "tab":
			if !m.cmdInput.Focused() {
				// Toggle between panes when not in command input
				if m.selectedPane == "server" {
					m.selectedPane = "client"
				} else {
					m.selectedPane = "server"
				}
				return m, nil
			}
			// Command input tab completion
			currentInput := m.cmdInput.Value()
			if currentInput == "" {
				return m, nil
			}

			// If we have suggestions, cycle through them
			if len(m.suggestions) > 0 {
				currentSuggestion := m.suggestions[0]
				m.suggestions = append(m.suggestions[1:], currentSuggestion)
				m.cmdInput.SetValue(currentSuggestion)
				return m, nil
			}

			// Generate new suggestions
			parts := strings.Fields(currentInput)
			m.suggestions = []string{}

			if len(parts) == 1 {
				// Suggest main commands
				for _, cmd := range availableCommands {
					if strings.HasPrefix(cmd.Name, parts[0]) {
						m.suggestions = append(m.suggestions, cmd.Name)
					}
				}
			} else if len(parts) == 2 {
				// Suggest subcommands
				for _, cmd := range availableCommands {
					if cmd.Name == parts[0] && len(cmd.Subcommands) > 0 {
						for _, sub := range cmd.Subcommands {
							if strings.HasPrefix(sub, parts[1]) {
								m.suggestions = append(m.suggestions, fmt.Sprintf("%s %s", parts[0], sub))
							}
						}
					}
				}
			}

			if len(m.suggestions) > 0 {
				m.cmdInput.SetValue(m.suggestions[0])
			}
			return m, nil
		case "pgup":
			if !m.cmdInput.Focused() {
				if m.selectedPane == "server" {
					m.serverView.ViewUp()
				} else {
					m.clientView.ViewUp()
				}
			}
			return m, nil
		case "pgdown":
			if !m.cmdInput.Focused() {
				if m.selectedPane == "server" {
					m.serverView.ViewDown()
				} else {
					m.clientView.ViewDown()
				}
			}
			return m, nil
		case "esc":
			if m.cmdInput.Focused() {
				m.cmdInput.Blur()
				m.suggestions = nil
			} else {
				m.cmdInput.Focus()
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		verticalSpace := msg.Height - 3 // Reserve space for command input
		m.serverView.Height = verticalSpace
		m.serverView.Width = msg.Width / 2
		m.clientView.Height = verticalSpace
		m.clientView.Width = msg.Width / 2
		return m, nil

	case outputMsg:
		if msg.src == "server" || msg.src == "both" {
			m.serverView.SetContent(msg.text)
			m.serverView.GotoBottom()
		}
		if msg.src == "client" || msg.src == "both" {
			m.clientView.SetContent(msg.text)
			m.clientView.GotoBottom()
		}
		return m, nil

	case errMsg:
		m.err = msg.error
		return m, tea.Quit
	}

	m.cmdInput, cmd = m.cmdInput.Update(msg)
	cmds = append(cmds, cmd)

	m.serverView, cmd = m.serverView.Update(msg)
	cmds = append(cmds, cmd)

	m.clientView, cmd = m.clientView.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	if helpView := m.helpWindow.View(); helpView != "" {
		return helpView
	}

	m.serverPane.SetSelected(m.selectedPane == "server")
	m.clientPane.SetSelected(m.selectedPane == "client")

	panes := lipgloss.JoinHorizontal(lipgloss.Top,
		m.serverPane.View(),
		m.clientPane.View(),
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		panes,
		m.commandLine.View(),
	)
}

func (m model) shutdown() tea.Msg {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if m.clientCmd != nil && m.clientCmd.Process != nil {
			m.clientCmd.Process.Signal(syscall.SIGTERM)
			m.clientCmd.Process.Wait()
		}
	}()

	go func() {
		defer wg.Done()
		if m.serverCmd != nil && m.serverCmd.Process != nil {
			m.serverCmd.Process.Signal(syscall.SIGTERM)
			m.serverCmd.Process.Wait()
		}
	}()

	wg.Wait()
	return tea.Quit()
}

func (m *model) executeCommand(cmd string) tea.Cmd {
	m.suggestions = nil
	return func() tea.Msg {
		// Split the command into parts
		parts := strings.Fields(cmd)
		if len(parts) == 0 {
			return nil
		}

		m.lastCommand = cmd
		m.cmdHistory = append(m.cmdHistory, cmd)
		m.cmdHistoryPos = -1
		switch parts[0] {
		case "clear":
			m.serverOutput.Reset()
			m.clientOutput.Reset()
			return outputMsg{text: "", src: "both"}
		case "help":
			m.showHelp = true
			return nil
		case "restart":
			if len(parts) == 1 || parts[1] == "all" {
				return tea.Batch(
					func() tea.Msg {
						return outputMsg{text: "Restarting all processes...\n", src: "both"}
					},
					m.shutdown,
					m.startProcesses,
				)
			} else if parts[1] == "server" {
				if m.serverCmd != nil && m.serverCmd.Process != nil {
					m.serverCmd.Process.Signal(syscall.SIGTERM)
					m.serverRunning = false
					return tea.Batch(
						func() tea.Msg {
							return outputMsg{text: "Restarting server...\n", src: "server"}
						},
						func() tea.Msg {
							m.serverCmd = exec.Command("mmdev", "server", "start", "--watch")
							serverOut, _ := m.serverCmd.StdoutPipe()
							m.serverCmd.Stderr = m.serverCmd.Stdout
							m.serverCmd.Start()
							m.serverRunning = true
							go m.handleOutput(serverOut, &m.serverOutput, "server")
							return nil
						},
					)
				}
			} else if parts[1] == "client" {
				if m.clientCmd != nil && m.clientCmd.Process != nil {
					m.clientCmd.Process.Signal(syscall.SIGTERM)
					m.clientRunning = false
					return tea.Batch(
						func() tea.Msg {
							return outputMsg{text: "Restarting client...\n", src: "client"}
						},
						func() tea.Msg {
							m.clientCmd = exec.Command("mmdev", "client", "start", "--watch")
							clientOut, _ := m.clientCmd.StdoutPipe()
							m.clientCmd.Stderr = m.clientCmd.Stdout
							m.clientCmd.Start()
							m.clientRunning = true
							go m.handleOutput(clientOut, &m.clientOutput, "client")
							return nil
						},
					)
				}
			}
			return func() tea.Msg {
				return outputMsg{text: "Invalid restart command. Use: restart [server|client|all]\n", src: "both"}
			}
		case "status":
			status := fmt.Sprintf("Server: %v\nClient: %v\n",
				m.serverRunning, m.clientRunning)
			return outputMsg{text: status, src: "both"}
		default:
			return outputMsg{
				text: fmt.Sprintf("Unknown command: %s\n", cmd),
				src:  "server",
			}
		}
	}
}

var program *tea.Program

func StartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start both client and server",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := initialModel()
			p := tea.NewProgram(m,
				tea.WithAltScreen(),
				tea.WithMouseCellMotion(),
			)
			program = p

			if _, err := p.Run(); err != nil {
				return fmt.Errorf("error running program: %w", err)
			}
			return nil
		},
	}
}
