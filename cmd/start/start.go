package start

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170"))

	infoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	// Subtle style for scroll indicators
	subtleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	// Status indicators
	statusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("86"))

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))
)

var availableCommands = []string{
	"clear",
	"help",
	"restart",
	"status",
}

type model struct {
	serverView    viewport.Model
	clientView    viewport.Model
	cmdInput      textinput.Model
	serverCmd     *exec.Cmd
	clientCmd     *exec.Cmd
	showHelp      bool
	serverOutput  strings.Builder
	clientOutput  strings.Builder
	err           error
	serverRunning bool
	clientRunning bool
	lastCommand   string
	cmdHistory    []string
	cmdHistoryPos int
	statusMsg     string
	suggestions   []string
}

func initialModel() model {
	cmdInput := textinput.New()
	cmdInput.Placeholder = "Enter command..."
	cmdInput.Focus()

	sv := viewport.New(0, 0)
	cv := viewport.New(0, 0)
	
	sv.Style = lipgloss.NewStyle().Margin(0, 0, 1, 0)
	cv.Style = lipgloss.NewStyle().Margin(0, 0, 1, 0)
	
	return model{
		serverView:    sv,
		clientView:    cv,
		cmdInput:      cmdInput,
		showHelp:      false,
		serverRunning: false,
		clientRunning: false,
		cmdHistory:    make([]string, 0),
		cmdHistoryPos: -1,
		statusMsg:     "Ready to start processes",
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

type errMsg struct{ error }
type outputMsg struct {
	text string
	src  string
}

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
		switch msg.Type {
		case tea.MouseWheelUp:
			if msg.Y < m.serverView.Height {
				m.serverView.LineUp(3)
			} else {
				m.clientView.LineUp(3)
			}
		case tea.MouseWheelDown:
			if msg.Y < m.serverView.Height {
				m.serverView.LineDown(3)
			} else {
				m.clientView.LineDown(3)
			}
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, m.shutdown
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
			if m.cmdInput.Focused() {
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
				m.suggestions = []string{}
				for _, cmd := range availableCommands {
					if strings.HasPrefix(cmd, currentInput) {
						m.suggestions = append(m.suggestions, cmd)
					}
				}
				
				if len(m.suggestions) > 0 {
					m.cmdInput.SetValue(m.suggestions[0])
				}
				return m, nil
			}
			m.cmdInput.Focus()
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

	if m.showHelp {
		return `
Keyboard shortcuts:
  q, ctrl+c    Quit
  ?, h         Toggle help
  :            Focus command input
  tab, esc     Toggle input focus
  ↑/↓          Scroll output
  ctrl+l       Clear output
  
Commands:
  clear        Clear all output
  help         Show this help
  restart      Restart both processes
  status       Show process status
Press any key to close help
`
	}

	var serverIndicator, clientIndicator string
	if m.serverView.ScrollPercent() < 1.0 {
		serverIndicator = subtleStyle.Render("↓")
	}
	if m.clientView.ScrollPercent() < 1.0 {
		clientIndicator = subtleStyle.Render("↓")
	}

	serverStatus := statusStyle.Render("●")
	if !m.serverRunning {
		serverStatus = errorStyle.Render("○")
	}
	
	clientStatus := statusStyle.Render("●")
	if !m.clientRunning {
		clientStatus = errorStyle.Render("○")
	}

	statusBar := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render(fmt.Sprintf("%s | ? for help | : for command | q to quit", m.statusMsg))

	serverPane := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("Server Output: ")+serverStatus+" "+serverIndicator,
		m.serverView.View(),
	)

	clientPane := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("Client Output: ")+clientStatus+" "+clientIndicator,
		m.clientView.View(),
	)

	panes := lipgloss.JoinHorizontal(lipgloss.Top, serverPane, clientPane)

	return lipgloss.JoinVertical(lipgloss.Left,
		panes,
		infoStyle.Render("Command (Enter to execute):"),
		m.cmdInput.View(),
		statusBar,
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
			return tea.Batch(
				func() tea.Msg {
					return outputMsg{text: "Restarting processes...\n", src: "both"}
				},
				m.shutdown,
				m.startProcesses,
			)
		case "status":
			status := fmt.Sprintf("Server: %v\nClient: %v\n", 
				m.serverRunning, m.clientRunning)
			return outputMsg{text: status, src: "both"}
		default:
			return outputMsg{
				text: fmt.Sprintf("Unknown command: %s\n", cmd),
				src: "server",
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
