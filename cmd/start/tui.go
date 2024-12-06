package start

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
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

var viewportChan = make(chan NewViewportLine)

type NewViewportLine struct {
	Viewport string
	Line     string
}

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
	m.serverCmd = exec.Command("mmdev", "server", "start")
	serverOutR, serverOutW, err := os.Pipe()
	if err != nil {
		log.Printf("Error creating server pipe: %v", err)
	}
	m.serverCmd.Stdout = serverOutW
	m.serverCmd.Stderr = serverOutW

	// Start client process
	m.clientCmd = exec.Command("mmdev", "client", "start", "--watch")
	clientOutR, clientOutW, err := os.Pipe()
	if err != nil {
		log.Printf("Error creating client pipe: %v", err)
	}
	m.clientCmd.Stdout = clientOutW
	m.clientCmd.Stderr = clientOutW

	// Start processes
	log.Printf("Starting server process with command: %v", m.serverCmd.Args)
	if err := m.serverCmd.Start(); err != nil {
		log.Printf("Error starting server: %v", err)
	} else {
		log.Printf("Server process started successfully with PID %d", m.serverCmd.Process.Pid)
	}

	log.Printf("Starting client process with command: %v", m.clientCmd.Args)
	if err := m.clientCmd.Start(); err != nil {
		log.Printf("Error starting client: %v", err)
	} else {
		log.Printf("Client process started successfully with PID %d", m.clientCmd.Process.Pid)
	}

	// Handle output streams
	go handleOutput(serverOutR, &m, "server")
	go func() {
		if err := m.serverCmd.Wait(); err != nil {
			log.Printf("Server process ended with error: %v", err)
		}
		serverOutW.Close()
	}()

	go handleOutput(clientOutR, &m, "client")
	go func() {
		if err := m.clientCmd.Wait(); err != nil {
			log.Printf("Client process ended with error: %v", err)
		}
		clientOutW.Close()
	}()

	return m
}

func handleOutput(reader io.Reader, m *model, viewport string) {
	log.Printf("Starting output handler for %s viewport", viewport)
	scanner := bufio.NewScanner(reader)
	lineCount := 0
	for scanner.Scan() {
		text := scanner.Text() + "\n"
		lineCount++
		log.Printf("[%s][line %d] Output: %s", viewport, lineCount, text)
		if viewport == "server" {
			m.serverLogs.WriteString(text)
		} else {
			m.clientLogs.WriteString(text)
		}
		viewportChan <- NewViewportLine{Viewport: viewport, Line: text}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("[%s] Scanner error: %v", viewport, err)
	}
}

func listenForUpdates() tea.Msg {
	return <-viewportChan
}

func (m model) Init() tea.Cmd {
	return listenForUpdates
}

func (m *model) restartServer() {
	// Kill existing server process in a goroutine
	if m.serverCmd != nil && m.serverCmd.Process != nil {
		oldCmd := m.serverCmd
		go func() {
			log.Printf("Sending SIGTERM to server process (PID %d)", oldCmd.Process.Pid)
			if err := oldCmd.Process.Signal(syscall.SIGTERM); err != nil {
				log.Printf("Error sending SIGTERM to server: %v", err)
			}
			if err := oldCmd.Wait(); err != nil {
				log.Printf("Server process wait error: %v", err)
			}
		}()
	}

	// Start new server process immediately
	m.serverCmd = exec.Command("mmdev", "server", "start")
	serverOutR, serverOutW, err := os.Pipe()
	if err != nil {
		log.Printf("Error creating server pipe: %v", err)
		return
	}
	m.serverCmd.Stdout = serverOutW
	m.serverCmd.Stderr = serverOutW

	log.Printf("Starting server process with command: %v", m.serverCmd.Args)
	if err := m.serverCmd.Start(); err != nil {
		log.Printf("Error starting server: %v", err)
		return
	}
	log.Printf("Server process started successfully with PID %d", m.serverCmd.Process.Pid)

	// Clear server viewport
	m.serverLogs.Reset()
	m.serverViewport.SetContent("")

	// Handle output streams
	go handleOutput(serverOutR, m, "server")
	go func() {
		if err := m.serverCmd.Wait(); err != nil {
			log.Printf("Server process ended with error: %v", err)
		}
		serverOutW.Close()
	}()
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

	switch msg := msg.(type) {
	case NewViewportLine:
		if msg.Viewport == "server" {
			log.Printf("Update server logs: %s", m.serverViewport.View()+msg.Line)
			m.serverViewport.SetContent(m.serverViewport.View() + msg.Line)
			m.serverViewport.GotoBottom()
		} else {
			log.Printf("Update client logs: %s", m.clientViewport.View()+msg.Line)
			m.clientViewport.SetContent(m.clientViewport.View() + msg.Line)
			m.clientViewport.GotoBottom()
		}
		return m, listenForUpdates
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
				log.Printf("Quit requested, gracefully stopping processes...")

				// Send SIGTERM to both processes
				if m.clientCmd != nil && m.clientCmd.Process != nil {
					log.Printf("Sending SIGTERM to client process (PID %d)", m.clientCmd.Process.Pid)
					if err := m.clientCmd.Process.Signal(syscall.SIGTERM); err != nil {
						log.Printf("Error sending SIGTERM to client: %v", err)
					}
				}

				if m.serverCmd != nil && m.serverCmd.Process != nil {
					log.Printf("Sending SIGTERM to server process (PID %d)", m.serverCmd.Process.Pid)
					if err := m.serverCmd.Process.Signal(syscall.SIGTERM); err != nil {
						log.Printf("Error sending SIGTERM to server: %v", err)
					}
				}

				// Wait for processes to finish in a goroutine
				go func() {
					if m.clientCmd != nil {
						if err := m.clientCmd.Wait(); err != nil {
							log.Printf("Client process wait error: %v", err)
						}
						log.Printf("Client process terminated")
					}

					if m.serverCmd != nil {
						if err := m.serverCmd.Wait(); err != nil {
							log.Printf("Server process wait error: %v", err)
						}
						log.Printf("Server process terminated")
					}

					// Send quit message through the viewport channel
					viewportChan <- NewViewportLine{Viewport: "server", Line: "Server stopped\n"}
					viewportChan <- NewViewportLine{Viewport: "client", Line: "Client stopped\n"}
				}()

				// Return immediately to keep the UI responsive
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
			case "r":
				if m.selectedPane == "server" {
					m.restartServer()
				}
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		if !m.ready {
			viewportWidth := msg.Width / 2
			log.Printf("Initializing viewports with width=%d height=%d", viewportWidth, msg.Height-2)

			m.serverViewport = viewport.New(viewportWidth, msg.Height-2)
			m.clientViewport = viewport.New(viewportWidth, msg.Height-2)
			m.ready = true
			log.Printf("Viewports initialized successfully")
		} else {
			log.Printf("Window size changed to width=%d height=%d", msg.Width, msg.Height)
		}
	}

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
		bottomBar = helpStyle.Render("↑/↓: scroll • q: quit • r: restart • tab: switch • :: command")
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
