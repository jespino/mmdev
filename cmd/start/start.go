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
)

type model struct {
	serverView   viewport.Model
	clientView   viewport.Model
	cmdInput     textinput.Model
	serverCmd    *exec.Cmd
	clientCmd    *exec.Cmd
	showHelp     bool
	serverOutput strings.Builder
	clientOutput strings.Builder
	err          error
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
		serverView: sv,
		clientView: cv,
		cmdInput:   cmdInput,
		showHelp:   false,
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
	if err := m.clientCmd.Start(); err != nil {
		m.serverCmd.Process.Kill()
		return errMsg{err}
	}

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
		program.Send(outputMsg{text: text, src: source})
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, m.shutdown
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "enter":
			if m.cmdInput.Focused() {
				cmd := m.cmdInput.Value()
				m.cmdInput.SetValue("")
				return m, m.executeCommand(cmd)
			}
		case ":":
			m.cmdInput.Focus()
			return m, textinput.Blink
		case "tab", "esc":
			if m.cmdInput.Focused() {
				m.cmdInput.Blur()
			} else {
				m.cmdInput.Focus()
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		verticalSpace := msg.Height - 3 // Reserve space for command input
		m.serverView.Height = verticalSpace / 2
		m.serverView.Width = msg.Width
		m.clientView.Height = verticalSpace / 2
		m.clientView.Width = msg.Width
		return m, nil

	case outputMsg:
		if msg.src == "server" {
			m.serverView.SetContent(m.serverOutput.String())
			m.serverView.GotoBottom()
		} else {
			m.clientView.SetContent(m.clientOutput.String())
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
  q, ctrl+c  Quit
  ?          Toggle help
  :          Focus command input
  tab        Toggle input focus
  ↑/↓        Scroll output
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

	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("Server Output: ")+serverIndicator,
		m.serverView.View(),
		titleStyle.Render("Client Output: ")+clientIndicator,
		m.clientView.View(),
		infoStyle.Render("Command (Enter to execute):"),
		m.cmdInput.View(),
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

func (m model) executeCommand(cmd string) tea.Cmd {
	return func() tea.Msg {
		// Split the command into parts
		parts := strings.Fields(cmd)
		if len(parts) == 0 {
			return nil
		}

		switch parts[0] {
		case "clear":
			m.serverOutput.Reset()
			m.clientOutput.Reset()
			return outputMsg{text: "", src: "both"}
		case "help":
			m.showHelp = true
			return nil
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
