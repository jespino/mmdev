package start

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

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

			// Create text views for server and client output
			serverView := tview.NewTextView()
			serverView.
				SetDynamicColors(true).
				SetScrollable(true).
				SetTitle("Server").
				SetBorder(true)
			serverView.SetWrap(true)

			clientView := tview.NewTextView()
			clientView.
				SetDynamicColors(true).
				SetScrollable(true).
				SetTitle("Client").
				SetBorder(true)
			clientView.SetWrap(true)

			// Create flex layout
			flex := tview.NewFlex()
			flex.SetDirection(tview.FlexRow).
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
						if err := clientCmd.Cancel(); err != nil {
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
						if err := serverCmd.Cancel(); err != nil {
							fmt.Fprintf(serverView, "[red]Error sending SIGTERM to server process: %v[white]\n", err)
							serverCmd.Cancel()
						}
					}
					serverCmd.Process.Wait()
				}()

				wg.Wait()

				// Wait for processes to start shutting down
				time.Sleep(5 * time.Second)
				app.Stop()
			}

			// Setup global key bindings at application level
			app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyRune {
					switch event.Rune() {
					case 'q':
						go stopProcesses()
						return nil
					case 'h':
						flex.SetDirection(tview.FlexRow)
						return nil
					case 'v':
						flex.SetDirection(tview.FlexColumn)
						return nil
					}
				} else if event.Key() == tcell.KeyEsc {
					go stopProcesses()
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
						_, _, _, height := view.GetInnerRect()
						row, _ := view.GetScrollOffset()
						content := view.GetText(false)
						lines := len(strings.Split(content, "\n"))

						fmt.Fprintln(writer, text)

						// Auto-scroll only if we're at the bottom
						if lines-row <= height {
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

			serverCmd.Wait()
			clientCmd.Wait()

			fmt.Fprintf(serverView, "[green]All processes stopped successfully[white]\n")
			fmt.Fprintf(clientView, "[green]All processes stopped successfully[white]\n")
			return nil
		},
	}

	return cmd
}
