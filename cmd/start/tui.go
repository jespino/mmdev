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

	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF69B4")).
			Bold(true)

	suggestionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	commands = []string{
		"quit",
		"client-restart",
		"server-restart",
	}

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
	suggestion     string

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
	windowWidth       int
	windowHeight      int
}

func initialModel() model {
	commandInput := textinput.New()
	commandInput.Prompt = ": "

	m := model{
		selectedPane:   "server",
		commandMode:    false,
		commandInput:   commandInput,
		serverAtBottom: true,
		clientAtBottom: true,
		splitVertical:  false,
	}

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
	if err := m.serverCmd.Start(); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
	}

	if err := m.clientCmd.Start(); err != nil {
		fmt.Printf("Error starting client: %v\n", err)
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

func wrapLine(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	
	var lines []string
	remaining := text
	
	for len(remaining) > width {
		idx := width
		// Try to break at last space before width
		for i := idx; i >= 0; i-- {
			if remaining[i] == ' ' {
				idx = i
				break
			}
		}
		if idx == width {
			// No space found, force break at width
			lines = append(lines, remaining[:width])
			remaining = remaining[width:]
		} else {
			lines = append(lines, remaining[:idx])
			remaining = remaining[idx+1:] // Skip the space
		}
	}
	if remaining != "" {
		lines = append(lines, remaining)
	}
	return lines
}

func handleOutput(reader io.Reader, m *model, viewport string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		text := scanner.Text()
		width := m.windowWidth
		if m.splitVertical {
			width = m.windowWidth / 2
		}
		
		// Wrap the line
		wrappedLines := wrapLine(text, width-2) // -2 for padding
		for _, line := range wrappedLines {
			if viewport == "server" {
				m.serverLogs.WriteString(line + "\n")
			} else {
				m.clientLogs.WriteString(line + "\n")
			}
			viewportChan <- NewViewportLine{Viewport: viewport, Line: line + "\n"}
		}
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
	// Clear viewport content
	m.serverLogs.Reset()
	m.serverViewContent.Reset()
	m.serverViewport.SetContent("")

	// Check if process exists and is running
	if m.serverCmd != nil && m.serverCmd.Process != nil {
		// Try to send signal 0 to check if process is running
		if err := m.serverCmd.Process.Signal(syscall.Signal(0)); err == nil {
			// Process exists and we have permission to signal it
			if err := m.serverCmd.Process.Signal(syscall.SIGUSR1); err != nil {
				fmt.Printf("Error sending SIGUSR1 to server: %v\n", err)
			}
			return
		}
		// Process is not running or we don't have permission
		m.serverCmd = nil
	}

	// Server is not running, start it
	m.serverCmd = exec.Command("mmdev", "server", "start")
	serverOutR, serverOutW, err := os.Pipe()
	if err != nil {
		fmt.Printf("Error creating server pipe: %v\n", err)
		return
	}
	m.serverCmd.Stdout = serverOutW
	m.serverCmd.Stderr = serverOutW

	if err := m.serverCmd.Start(); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		return
	}

	// Handle output streams
	go handleOutput(serverOutR, m, "server")
	go func() {
		if err := m.serverCmd.Wait(); err != nil {
			fmt.Printf("Server process ended with error: %v\n", err)
		}
		serverOutW.Close()
	}()
}

func (m *model) runCommand(cmd string) (tea.Model, tea.Cmd) {
	// Handle command execution here
	switch cmd {
	case "q", "quit":
		m.quitting = true

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
	case "server-restart":
		m.restartServer()
		return m, nil
	case "client-restart":
		if m.clientCmd != nil && m.clientCmd.Process != nil {
			log.Printf("Terminating client process (PID %d)", m.clientCmd.Process.Pid)
			if err := m.clientCmd.Process.Signal(syscall.SIGTERM); err != nil {
				log.Printf("Error sending SIGTERM to client: %v", err)
			}
			if err := m.clientCmd.Wait(); err != nil {
				log.Printf("Error waiting for client to terminate: %v", err)
			}
		}

		// Clear client viewport and content
		m.clientLogs.Reset()
		m.clientViewContent.Reset()
		m.clientViewport.SetContent("")

		// Start new client process
		m.clientCmd = exec.Command("mmdev", "webapp", "start", "--watch")
		clientOutR, clientOutW, err := os.Pipe()
		if err != nil {
			log.Printf("Error creating client pipe: %v", err)
			return m, nil
		}
		m.clientCmd.Stdout = clientOutW
		m.clientCmd.Stderr = clientOutW

		log.Printf("Starting new client process with command: %v", m.clientCmd.Args)
		if err := m.clientCmd.Start(); err != nil {
			log.Printf("Error starting client: %v", err)
			return m, nil
		}
		log.Printf("New client process started successfully with PID %d", m.clientCmd.Process.Pid)

		// Handle output streams
		go handleOutput(clientOutR, m, "client")
		go func() {
			if err := m.clientCmd.Wait(); err != nil {
				log.Printf("Client process ended with error: %v", err)
			}
			clientOutW.Close()
		}()

		return m, nil
	}
	return m, nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		serverCmd tea.Cmd
		clientCmd tea.Cmd
		cmdCmd    tea.Cmd
	)

	switch msg := msg.(type) {
	case NewViewportLine:
		if msg.Viewport == "server" {
			m.serverViewContent.WriteString(msg.Line)
			m.serverViewport.SetContent(m.serverViewContent.String())
			if m.serverAtBottom {
				m.serverViewport.GotoBottom()
			}
		} else {
			m.clientViewContent.WriteString(msg.Line)
			m.clientViewport.SetContent(m.clientViewContent.String())
			if m.clientAtBottom {
				m.clientViewport.GotoBottom()
			}
		}
		if msg.Quit {
			return m, tea.Quit
		}
		return m, listenForUpdates
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionMotion {
			// Calculate viewport positions
			serverHeight := (m.windowHeight / 2) - 3
			if m.splitVertical {
				serverHeight = m.windowHeight - 4
			}

			if m.splitVertical {
				// Vertical split - check if mouse is in left or right half
				if msg.X < m.windowWidth/2 {
					m.selectedPane = "server"
				} else {
					m.selectedPane = "client"
				}
			} else {
				// Horizontal split - check if mouse is in top or bottom half
				if msg.Y < serverHeight+2 { // +2 for title and padding
					m.selectedPane = "server"
				} else {
					m.selectedPane = "client"
				}
			}
			return m, nil
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.selectedPane == "server" {
				m.serverViewport.LineUp(3)
				m.serverAtBottom = m.serverViewport.AtBottom()
			} else {
				m.clientViewport.LineUp(3)
				m.clientAtBottom = m.clientViewport.AtBottom()
			}
		case tea.MouseButtonWheelDown:
			if m.selectedPane == "server" {
				m.serverViewport.LineDown(3)
				m.serverAtBottom = m.serverViewport.AtBottom()
			} else {
				m.clientViewport.LineDown(3)
				m.clientAtBottom = m.clientViewport.AtBottom()
			}
		}
		return m, nil
	case tea.KeyMsg:
		if m.commandMode {
			switch msg.String() {
			case "ctrl+c":
				m.commandMode = false
				m.commandInput.SetValue("")
				m.suggestion = ""
				return m, nil
			case "enter":
				m.commandMode = false
				value := m.commandInput.Value()
				m.commandInput.SetValue("")
				m.suggestion = ""
				return m.runCommand(value)
			case "tab":
				if m.suggestion != "" {
					m.commandInput.SetValue(m.suggestion)
					m.commandInput.CursorEnd()
					m.suggestion = ""
				}
				return m, nil
			case "esc":
				m.commandMode = false
				m.commandInput.SetValue("")
				return m, nil
			default:
				m.commandInput, cmdCmd = m.commandInput.Update(msg)
				// Find suggestion
				input := m.commandInput.Value()
				m.suggestion = ""
				if input != "" {
					for _, cmd := range commands {
						if strings.HasPrefix(cmd, input) && cmd != input {
							m.suggestion = cmd
							break
						}
					}
				}
				return m, cmdCmd
			}
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
			case "d":
				width := m.windowWidth
				if m.splitVertical {
					width = m.windowWidth / 2
				}
				divider := dividerStyle.Render(strings.Repeat("=", width-2)) + "\n"
				
				// Add divider to both viewports
				m.serverViewContent.WriteString(divider)
				m.serverViewport.SetContent(m.serverViewContent.String())
				if m.serverAtBottom {
					m.serverViewport.GotoBottom()
				}
				
				m.clientViewContent.WriteString(divider)
				m.clientViewport.SetContent(m.clientViewContent.String())
				if m.clientAtBottom {
					m.clientViewport.GotoBottom()
				}
				return m, nil
			case "s":
				m.splitVertical = !m.splitVertical
				m.serverViewport.GotoBottom()
				m.clientViewport.GotoBottom()
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		if !m.ready {
			m.windowWidth = msg.Width
			m.windowHeight = msg.Height
			log.Printf("Initializing viewports with width=%d height=%d", msg.Width/2, msg.Height-4)

			m.serverViewport = viewport.New(msg.Width/2, msg.Height-4)
			m.clientViewport = viewport.New(msg.Width/2, msg.Height-4)
			m.ready = true
			log.Printf("Viewports initialized successfully")
		} else {
			log.Printf("Window size changed to width=%d height=%d", msg.Width, msg.Height)
		}
	}

	// Only process viewport updates if we're not in command mode
	if !m.commandMode {
		if m.selectedPane == "server" {
			m.serverViewport, serverCmd = m.serverViewport.Update(msg)
			m.serverAtBottom = m.serverViewport.AtBottom()
		} else {
			m.clientViewport, clientCmd = m.clientViewport.Update(msg)
			m.clientAtBottom = m.clientViewport.AtBottom()
		}
	}

	return m, tea.Batch(serverCmd, clientCmd, cmdCmd)
}

func (m *model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var commandArea string
	if m.commandMode {
		if m.suggestion != "" {
			commandArea = m.commandInput.View() + suggestionStyle.Render(m.suggestion[len(m.commandInput.Value()):])
		} else {
			commandArea = m.commandInput.View()
		}
	} else {
		commandArea = helpStyle.Render("↑/↓: scroll • q: quit • r: restart server • s: toggle split • tab: switch • d: divider • :: command")
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
		m.serverViewport.Height = m.windowHeight - 4
		m.serverViewport.Width = m.windowWidth / 2
		m.clientViewport.Height = m.windowHeight - 4
		m.clientViewport.Width = m.windowWidth / 2
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
		m.serverViewport.Height = (m.windowHeight / 2) - 3
		m.serverViewport.Width = m.windowWidth
		m.clientViewport.Height = (m.windowHeight / 2) - 3
		m.clientViewport.Width = m.windowWidth
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
		commandArea,
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
