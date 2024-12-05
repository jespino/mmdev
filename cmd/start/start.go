package start

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

func StartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start both client and server",
		RunE: func(cmd *cobra.Command, args []string) error {
			app := tview.NewApplication()

			// Create help modal
			helpModal := tview.NewModal().
				SetText("Keyboard Shortcuts:\n\n" +
					"q or ESC - Quit application\n" +
					"h - Horizontal split layout\n" +
					"v - Vertical split layout\n" +
					"TAB - Switch focus between views\n" +
					"PgUp/PgDn - Scroll current view\n" +
					"? - Show/hide this help").
				AddButtons([]string{"Close"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					app.SetRoot(flex, true)
				})

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

			// Create flex layout and track direction
			flex := tview.NewFlex()
			currentDirection := tview.FlexRow
			flex.SetDirection(currentDirection).
				AddItem(serverView, 0, 1, false).
				AddItem(clientView, 0, 1, false)

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
						go stopProcesses()
						return nil
					case '?':
						app.SetRoot(helpModal, false)
						return nil
					case 'h':
						currentDirection = tview.FlexRow
						flex.SetDirection(currentDirection)
						return nil
					case 'v':
						currentDirection = tview.FlexColumn
						flex.SetDirection(currentDirection)
						return nil
					case '\t':
						// Cycle focus between views
						if app.GetFocus() == serverView {
							app.SetFocus(clientView)
						} else {
							app.SetFocus(serverView)
						}
						return nil
					}
				} else if event.Key() == tcell.KeyEsc || event.Key() == tcell.KeyEnter {
					if app.GetRoot() == helpModal {
						app.SetRoot(flex, true)
					} else {
						go stopProcesses()
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
