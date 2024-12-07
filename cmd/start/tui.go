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

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#383838")).
			Padding(0, 1).
			MarginBottom(1)

	titleSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#FF69B4")).
				Padding(0, 1).
				MarginBottom(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

var viewportChan = make(chan NewViewportLine)

type NewViewportLine struct {
	Viewport string
	Line     string
	Quit     bool
}

type model struct {
	serverViewport viewport.Model
	clientViewport viewport.Model
	commandInput   textinput.Model
	ready          bool
	selectedPane   string
	commandMode    bool
	serverAtBottom bool
	clientAtBottom bool
	splitVertical  bool

	serverCmd         *exec.Cmd
	clientCmd         *exec.Cmd
	serverWriter      io.Writer
	clientWriter      io.Writer
	quitting          bool
	serverLogs        strings.Builder
	clientLogs        strings.Builder
	serverViewContent strings.Builder
	clientViewContent strings.Builder
	shutdownWg        sync.WaitGroup
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
		selectedPane:   "server",
		commandMode:    false,
		commandInput:   commandInput,
		serverAtBottom: true,
		clientAtBottom: true,
		splitVertical:  true,
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
	m.clientCmd = exec.Command("mmdev", "webapp", "start", "--watch")
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
	if m.serverCmd != nil && m.serverCmd.Process != nil {
		log.Printf("Sending SIGUSR1 to server process (PID %d)", m.serverCmd.Process.Pid)
		if err := m.serverCmd.Process.Signal(syscall.SIGUSR1); err != nil {
			log.Printf("Error sending SIGUSR1 to server: %v", err)
		}
		// Clear server viewport and content
		m.serverLogs.Reset()
		m.serverViewContent.Reset()
		m.serverViewport.SetContent("")
	}
}

func (m *model) runCommand(cmd string) (tea.Model, tea.Cmd) {
	// Handle command execution here
	if cmd == "q" || cmd == "quit" {
		return m, tea.Quit
	}
	return m, nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Printf("Update called with message type: %T", msg)
	var (
		serverCmd tea.Cmd
		clientCmd tea.Cmd
		cmdCmd    tea.Cmd
	)

	switch msg := msg.(type) {
	case NewViewportLine:
		if msg.Viewport == "server" {
			m.serverViewContent.WriteString(msg.Line)
			log.Printf("Update server logs: %s", msg.Line)
			m.serverViewport.SetContent(m.serverViewContent.String())
			if m.serverAtBottom {
				m.serverViewport.GotoBottom()
			}
		} else {
			m.clientViewContent.WriteString(msg.Line)
			log.Printf("Update client logs: %s", msg.Line)
			m.clientViewport.SetContent(m.clientViewContent.String())
			if m.clientAtBottom {
				m.clientViewport.GotoBottom()
			}
		}
		if msg.Quit {
			log.Printf("Received quit message, shutting down application")
			return m, tea.Quit
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

				// Add to wait group for server process
				if m.serverCmd != nil && m.serverCmd.Process != nil {
					m.shutdownWg.Add(1)
				}

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
						m.shutdownWg.Done()
					}

					// Send quit message through the viewport channel
					viewportChan <- NewViewportLine{Viewport: "server", Line: "Server stopped\n"}
					viewportChan <- NewViewportLine{Viewport: "client", Line: "Client stopped\n"}
				}()

				// Wait for server to finish before quitting
				go func() {
					m.shutdownWg.Wait()
					viewportChan <- NewViewportLine{Viewport: "server", Line: "Shutdown complete\n"}
					viewportChan <- NewViewportLine{Viewport: "server", Line: "Exiting...\n"}
					// Send final quit message through the viewport channel
					viewportChan <- NewViewportLine{Quit: true}
				}()

				return m, nil
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
			case "s":
				m.splitVertical = !m.splitVertical
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		if !m.ready {
			viewportWidth := msg.Width / 2
			log.Printf("Initializing viewports with width=%d height=%d", viewportWidth, msg.Height-4)

			m.serverViewport = viewport.New(viewportWidth, msg.Height-4)
			m.clientViewport = viewport.New(viewportWidth, msg.Height-4)
			m.ready = true
			log.Printf("Viewports initialized successfully")
		} else {
			log.Printf("Window size changed to width=%d height=%d", msg.Width, msg.Height)
		}
	}

	if m.selectedPane == "server" {
		m.serverViewport, serverCmd = m.serverViewport.Update(msg)
		m.serverAtBottom = m.serverViewport.AtBottom()
	} else {
		m.clientViewport, clientCmd = m.clientViewport.Update(msg)
		m.clientAtBottom = m.clientViewport.AtBottom()
	}

	return m, tea.Batch(serverCmd, clientCmd, cmdCmd)
}

func (m *model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var bottomBar string
	if m.commandMode {
		bottomBar = m.commandInput.View()
	} else {
		bottomBar = helpStyle.Render("↑/↓: scroll • q: quit • r: restart server • s: toggle split • tab: switch • :: command")
	}

	serverScrollPct := fmt.Sprintf("%d%%", int(m.serverViewport.ScrollPercent()*100))
	clientScrollPct := fmt.Sprintf("%d%%", int(m.clientViewport.ScrollPercent()*100))

	titleServer := titleStyle.Render(fmt.Sprintf("Server [%s]", serverScrollPct))
	titleClient := titleStyle.Render(fmt.Sprintf("Client [%s]", clientScrollPct))
	if m.selectedPane == "server" {
		titleServer = titleSelectedStyle.Render(fmt.Sprintf("Server [%s]", serverScrollPct))
	} else {
		titleClient = titleSelectedStyle.Render(fmt.Sprintf("Client [%s]", clientScrollPct))
	}

	var content string
	if m.splitVertical {
		content = lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.JoinVertical(lipgloss.Left,
				titleServer,
				m.serverViewport.View(),
			),
			lipgloss.JoinVertical(lipgloss.Left,
				titleClient,
				m.clientViewport.View(),
			),
		)
	} else {
		content = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.JoinVertical(lipgloss.Left,
				titleServer,
				m.serverViewport.View(),
			),
			lipgloss.JoinVertical(lipgloss.Left,
				titleClient,
				m.clientViewport.View(),
			),
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		content,
		bottomBar,
	)
}

func StartTUI() error {
	initial := initialModel()
	p := tea.NewProgram(
		&initial,
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
	)

	_, err := p.Run()
	return err
}
