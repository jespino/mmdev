package start

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
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
				app.Stop()
				
				// Send SIGTERM to client process
				if clientCmd != nil && clientCmd.Process != nil {
					if err := clientCmd.Process.Signal(syscall.SIGTERM); err != nil {
						fmt.Printf("Error sending SIGTERM to client process: %v\n", err)
					}
				}
				
				// Send SIGTERM to server process
				if serverCmd != nil && serverCmd.Process != nil {
					if err := serverCmd.Process.Signal(syscall.SIGTERM); err != nil {
						fmt.Printf("Error sending SIGTERM to server process: %v\n", err)
					}
				}
			}

			// Setup global key bindings at application level
			app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyRune {
					switch event.Rune() {
					case 'q':
						stopProcesses()
						return nil
					case 'h':
						flex.SetDirection(tview.FlexRow)
						app.Draw()
						return nil
					case 'v':
						flex.SetDirection(tview.FlexColumn)
						app.Draw()
						return nil
					}
				} else if event.Key() == tcell.KeyEsc {
					stopProcesses()
					return nil
				}
				return event
			})


			// Create channels to track process completion
			serverDone := make(chan error, 1)
			clientDone := make(chan error, 1)

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

			// Monitor process completion
			go func() {
				serverDone <- serverCmd.Wait()
			}()
			go func() {
				clientDone <- clientCmd.Wait()
			}()

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

			// Wait for both processes to complete
			select {
			case err := <-serverDone:
				if err != nil {
					fmt.Printf("Server process ended with error: %v\n", err)
				}
				clientCmd.Process.Signal(syscall.SIGTERM)
				<-clientDone
			case err := <-clientDone:
				if err != nil {
					fmt.Printf("Client process ended with error: %v\n", err)
				}
				serverCmd.Process.Signal(syscall.SIGTERM)
				<-serverDone
			}
			return nil
		},
	}


	return cmd
}
