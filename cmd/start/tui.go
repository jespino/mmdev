package start

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

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
	// Set up logging
	logFile, err := os.OpenFile("mmdev-debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		os.Exit(1)
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetOutput(logFile)
	log.Printf("=== Starting new mmdev session ===")

	commandInput := textinput.New()
	commandInput.Prompt = ": "

	m := model{
		selectedPane: "server",
		commandMode:  false,
		commandInput: commandInput,
	}
	
	log.Printf("Initializing model with selectedPane=%s", m.selectedPane)

	// Start server process
	m.serverCmd = exec.Command("mmdev", "server", "start", "--watch")
	serverOut, err := m.serverCmd.StdoutPipe()
	if err != nil {
		log.Printf("Error creating server stdout pipe: %v", err)
	}
	m.serverCmd.Stderr = m.serverCmd.Stdout

	// Start client process
	m.clientCmd = exec.Command("mmdev", "client", "start", "--watch")
	clientOut, err := m.clientCmd.StdoutPipe()
	if err != nil {
		log.Printf("Error creating client stdout pipe: %v", err)
	}
	m.clientCmd.Stderr = m.clientCmd.Stdout

	// Start processes
	log.Printf("Starting server process...")
	if err := m.serverCmd.Start(); err != nil {
		log.Printf("Error starting server: %v", err)
	}
	
	log.Printf("Starting client process...")
	if err := m.clientCmd.Start(); err != nil {
		log.Printf("Error starting client: %v", err)
	}

	// Handle output streams
	go handleOutput(serverOut, &m, "server")
	go handleOutput(clientOut, &m, "client")

	return m
}

func handleOutput(reader io.Reader, m *model, viewport string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		text := scanner.Text() + "\n"
		log.Printf("[%s] Output: %s", viewport, text)
		if viewport == "server" {
			m.serverLogs.WriteString(text)
			m.Update("update-server-viewport")
		} else {
			m.clientLogs.WriteString(text)
			m.Update("update-client-viewport")
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("[%s] Scanner error: %v", viewport, err)
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
	log.Printf("Update called with message type: %T", msg)
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
