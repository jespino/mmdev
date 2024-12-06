package start

import (
	"bufio"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("241"))
	titleSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("170"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

type model struct {
	serverViewport viewport.Model
	clientViewport viewport.Model
	commandInput   textinput.Model
	ready          bool
	selectedPane   string
	commandMode    bool

	serverCmd    *exec.Cmd
	clientCmd    *exec.Cmd
	serverWriter io.Writer
	clientWriter io.Writer
	quitting     bool
	serverLogs   strings.Builder
	clientLogs   strings.Builder
}

func initialModel() model {
	commandInput := textinput.New()
	commandInput.Prompt = ": "

	m := model{
		selectedPane: "server",
		commandMode:  false,
		commandInput: commandInput,
	}

	// Start server process
	m.serverCmd = exec.Command("mmdev", "server", "start", "--watch")
	serverOut, _ := m.serverCmd.StdoutPipe()
	m.serverCmd.Stderr = m.serverCmd.Stdout

	// Start client process
	m.clientCmd = exec.Command("mmdev", "client", "start", "--watch")
	clientOut, _ := m.clientCmd.StdoutPipe()
	m.clientCmd.Stderr = m.clientCmd.Stdout

	// Start processes
	m.serverCmd.Start()
	m.clientCmd.Start()

	// Handle output streams
	go handleOutput(serverOut, &m, "server")
	go handleOutput(clientOut, &m, "client")

	return m
}

func handleOutput(reader io.Reader, m *model, viewport string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		text := scanner.Text() + "\n"
		if viewport == "server" {
			m.serverLogs.WriteString(text)
			m.Update("update-server-viewport")
		} else {
			m.clientLogs.WriteString(text)
			m.Update("update-client-viewport")
		}
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) runCommand(cmd string) (tea.Model, tea.Cmd) {
	// Handle command execution here
	if cmd == "q" || cmd == "quit" {
		return m, tea.Quit
	}
	return m, nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quitting {
		return m, tea.Quit
	}
	var (
		serverCmd tea.Cmd
		clientCmd tea.Cmd
		cmdCmd    tea.Cmd
	)

	if msg == "update-server-viewport" {
		m.serverViewport.SetContent(m.serverLogs.String())
	}
	if msg == "update-client-viewport" {
		m.clientViewport.SetContent(m.serverLogs.String())
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.commandMode {
			switch msg.String() {
			case "ctrl+c":
				m.commandMode = false
				m.commandInput.SetValue("")
				return m, nil
			case "enter":
				m.commandMode = false
				value := m.commandInput.Value()
				m.commandInput.SetValue("")
				return m.runCommand(value)
			case "esc":
				m.commandMode = false
				m.commandInput.SetValue("")
				return m, nil
			}
			m.commandInput, cmdCmd = m.commandInput.Update(msg)
		} else {
			switch msg.String() {
			case "q":
				m.quitting = true
				// Gracefully stop processes
				var wg sync.WaitGroup
				wg.Add(2)

				go func() {
					defer wg.Done()
					if m.clientCmd != nil && m.clientCmd.Process != nil {
						m.clientCmd.Process.Signal(syscall.SIGTERM)
					}
				}()

				go func() {
					defer wg.Done()
					if m.serverCmd != nil && m.serverCmd.Process != nil {
						m.serverCmd.Process.Signal(syscall.SIGTERM)
					}
				}()

				wg.Wait()
				return m, tea.Quit
			case "ctrl+c":
				return m, tea.Quit
			case ":":
				m.commandMode = true
				m.commandInput.SetValue("")
				m.commandInput.Focus()
				return m, nil
			case "tab":
				if m.selectedPane == "server" {
					m.selectedPane = "client"
				} else {
					m.selectedPane = "server"
				}
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		if !m.ready {
			viewportWidth := msg.Width / 2

			m.serverViewport = viewport.New(viewportWidth, msg.Height-2)
			m.clientViewport = viewport.New(viewportWidth, msg.Height-2)
			m.ready = true
		}
	}

	m.serverViewport.GotoBottom()
	m.clientViewport.GotoBottom()

	if m.selectedPane == "server" {
		m.serverViewport, serverCmd = m.serverViewport.Update(msg)
	} else {
		m.clientViewport, clientCmd = m.clientViewport.Update(msg)
	}

	return m, tea.Batch(serverCmd, clientCmd, cmdCmd)
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var bottomBar string
	if m.commandMode {
		bottomBar = m.commandInput.View()
	} else {
		bottomBar = helpStyle.Render("↑/↓: scroll • q: quit • a: add content • tab: switch • :: command")
	}

	titleServer := titleStyle.Render("Server")
	titleClient := titleStyle.Render("Client")
	if m.selectedPane == "server" {
		titleServer = titleSelectedStyle.Render("Server")
	} else {
		titleClient = titleSelectedStyle.Render("Client")
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.JoinVertical(lipgloss.Left,
				titleServer,
				m.serverViewport.View(),
			),
			lipgloss.JoinVertical(lipgloss.Left,
				titleClient,
				m.clientViewport.View(),
			),
		),
		bottomBar,
	)
}

func StartTUI() error {
	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
	)

	_, err := p.Run()
	return err
}
