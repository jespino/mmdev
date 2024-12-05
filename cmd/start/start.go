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

	return model{
		serverView: viewport.New(0, 0),
		clientView: viewport.New(0, 0),
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
		case ":":
			m.cmdInput.Focus()
			return m, textinput.Blink
		case "tab":
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

	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("Server Output:"),
		m.serverView.View(),
		titleStyle.Render("Client Output:"),
		m.clientView.View(),
		infoStyle.Render("Command:"),
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
			app := tview.NewApplication()
			var cmdInput *tview.InputField

			// Create text views for server and client output
			serverView := tview.NewTextView()
			serverView.SetDynamicColors(true)
			serverView.SetScrollable(true)
			serverView.SetTitle("Server")
			serverView.SetBorder(true)
			serverView.SetMaxLines(2000)
			serverView.SetWrap(true)
			serverView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				switch event.Key() {
				case tcell.KeyPgUp:
					row, _ := serverView.GetScrollOffset()
					serverView.ScrollTo(row-5, 0)
					return nil
				case tcell.KeyPgDn:
					row, _ := serverView.GetScrollOffset()
					serverView.ScrollTo(row+5, 0)
					return nil
				}
				return event
			})

			clientView := tview.NewTextView()
			clientView.SetDynamicColors(true)
			clientView.SetScrollable(true)
			clientView.SetTitle("Client")
			clientView.SetBorder(true)
			clientView.SetMaxLines(2000)
			clientView.SetWrap(true)
			clientView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				switch event.Key() {
				case tcell.KeyPgUp:
					row, _ := clientView.GetScrollOffset()
					clientView.ScrollTo(row-5, 0)
					return nil
				case tcell.KeyPgDn:
					row, _ := clientView.GetScrollOffset()
					clientView.ScrollTo(row+5, 0)
					return nil
				}
				return event
			})

			// Create command input field
			cmdInput = tview.NewInputField().
				SetLabel("Command: ").
				SetDoneFunc(func(key tcell.Key) {
					if key == tcell.KeyEnter {
						cmd := cmdInput.GetText()
						if cmd == "noop" {
							cmdInput.SetText("")
						}
					}
				})
			cmdInput.SetBorder(true)
			cmdInput.SetTitle("Command")

			// Create help indicator
			helpIndicator := tview.NewTextView().
				SetText("Press ? for help").
				SetTextColor(tcell.ColorYellow)
			helpIndicator.SetTextAlign(tview.AlignRight)

			// Create flex layout and track direction
			flex := tview.NewFlex()
			topFlex := tview.NewFlex().
				AddItem(cmdInput, 0, 4, false).
				AddItem(helpIndicator, 0, 1, false)

			// Create help modal
			helpModal := tview.NewModal().
				SetText("Keyboard Shortcuts:\n\n" +
					"q or ESC - Quit application\n" +
					"h - Horizontal split layout\n" +
					"v - Vertical split layout\n" +
					"TAB - Switch focus between views\n" +
					"PgUp/PgDn - Scroll current view\n" +
					": - Focus command input\n" +
					"? - Show/hide this help").
				AddButtons([]string{"Close"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					app.SetRoot(flex, true)
				})
			currentDirection := tview.FlexRow
			isHelpOpen := false

			mainFlex := tview.NewFlex().
				SetDirection(currentDirection).
				AddItem(serverView, 0, 1, false).
				AddItem(clientView, 0, 1, false)

			flex.SetDirection(tview.FlexColumn).
				AddItem(tview.NewFlex().
					SetDirection(tview.FlexRow).
					AddItem(topFlex, 1, 0, false).
					AddItem(mainFlex, 0, 1, false), 0, 1, false)

			var clientCmd, serverCmd *exec.Cmd

			// Function to gracefully stop all processes
			stopProcesses := func() {
				fmt.Fprintf(serverView, "\n[yellow]Initiating shutdown process...[white]\n")
				fmt.Fprintf(clientView, "\n[yellow]Initiating shutdown process...[white]\n")

				wg := sync.WaitGroup{}

				wg.Add(2)

				go func() {
					defer wg.Done()
					// Send SIGTERM to client process
					if clientCmd != nil && clientCmd.Process != nil {
						fmt.Fprintf(clientView, "[yellow]Sending SIGTERM to client process...[white]\n")
						if err := clientCmd.Process.Signal(syscall.SIGTERM); err != nil {
							fmt.Fprintf(clientView, "[red]Error sending SIGTERM to client process: %v[white]\n", err)
							clientCmd.Process.Kill()
						}
					}
					clientCmd.Process.Wait()
				}()

				go func() {
					defer wg.Done()
					// Send SIGTERM to server process
					if serverCmd != nil && serverCmd.Process != nil {
						fmt.Fprintf(serverView, "[yellow]Sending SIGTERM to server process...[white]\n")
						if err := serverCmd.Process.Signal(syscall.SIGTERM); err != nil {
							fmt.Fprintf(serverView, "[red]Error sending SIGTERM to server process: %v[white]\n", err)
							serverCmd.Cancel()
						}
					}
					serverCmd.Process.Wait()
				}()

				wg.Wait()

				app.Stop()
			}

			// Setup global key bindings at application level
			// Set initial focus to command input
			app.SetFocus(cmdInput)
			app.EnableMouse(true)
			app.SetMouseCapture(func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
				x, y := event.Position()
				_, _, width, _ := flex.GetRect()
				if currentDirection == tview.FlexRow {
					_, _, _, serverHeight := serverView.GetRect()
					if y < serverHeight {
						app.SetFocus(serverView)
					} else {
						app.SetFocus(clientView)
					}
				} else { // FlexColumn
					halfWidth := width / 2
					if x < halfWidth {
						app.SetFocus(serverView)
					} else {
						app.SetFocus(clientView)
					}
				}
				return event, action
			})
			app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyRune {
					switch event.Rune() {
					case 'q':
						if isHelpOpen {
							app.SetRoot(flex, true)
							isHelpOpen = false
							return nil
						} else {
							go stopProcesses()
						}
						return nil
					case '?':
						app.SetRoot(helpModal, false)
						isHelpOpen = true
						return nil
					case 'h':
						currentDirection = tview.FlexRow
						mainFlex.SetDirection(currentDirection)
						return nil
					case 'v':
						currentDirection = tview.FlexColumn
						mainFlex.SetDirection(currentDirection)
						return nil
					case ':':
						app.SetFocus(cmdInput)
						return nil
					}
				} else if event.Key() == tcell.KeyTab {
					// Cycle focus between views
					if app.GetFocus() == serverView {
						app.SetFocus(clientView)
					} else {
						app.SetFocus(serverView)
					}
					return nil
				} else if event.Key() == tcell.KeyEsc {
					if isHelpOpen {
						app.SetRoot(flex, true)
						isHelpOpen = false
						return nil
					} else {
						go stopProcesses()
					}
				} else if event.Key() == tcell.KeyEnter {
					if isHelpOpen {
						app.SetRoot(flex, true)
						isHelpOpen = false
						return nil
					}
					return nil
				}
				return event
			})

			// Start server process using mmdev command
			serverCmd = exec.Command("mmdev", "server", "start", "--watch")
			serverOut, err := serverCmd.StdoutPipe()
			if err != nil {
				return fmt.Errorf("failed to create server stdout pipe: %w", err)
			}
			serverCmd.Stderr = serverCmd.Stdout

			// Start client process using mmdev command
			clientCmd = exec.Command("mmdev", "client", "start", "--watch")
			clientOut, err := clientCmd.StdoutPipe()
			if err != nil {
				return fmt.Errorf("failed to create client stdout pipe: %w", err)
			}
			clientCmd.Stderr = clientCmd.Stdout

			// Start both processes
			if err := serverCmd.Start(); err != nil {
				return fmt.Errorf("failed to start server: %w", err)
			}
			if err := clientCmd.Start(); err != nil {
				serverCmd.Process.Kill()
				return fmt.Errorf("failed to start client: %w", err)
			}

			// Function to handle output with auto-scroll using ANSIWriter
			handleOutput := func(view *tview.TextView, reader io.Reader) {
				writer := tview.ANSIWriter(view)
				scanner := bufio.NewScanner(reader)
				for scanner.Scan() {
					text := scanner.Text()
					app.QueueUpdateDraw(func() {
						row, _ := view.GetScrollOffset()
						content := view.GetText(false)
						lines := len(strings.Split(content, "\n"))
						_, _, _, viewHeight := view.GetInnerRect()

						fmt.Fprintln(writer, text)

						// Auto-scroll only if we're at the bottom
						if lines-row <= viewHeight {
							view.ScrollToEnd()
						}
					})
				}
			}

			// Handle server output
			go handleOutput(serverView, serverOut)

			// Handle client output
			go handleOutput(clientView, clientOut)

			// Run the application
			if err := app.SetRoot(flex, true).Run(); err != nil {
				return fmt.Errorf("application error: %w", err)
			}

			fmt.Fprintf(serverView, "[green]All processes stopped successfully[white]\n")
			fmt.Fprintf(clientView, "[green]All processes stopped successfully[white]\n")
			return nil
		},
	}

	return cmd
}
