package start

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"github.com/jespino/mmdev/pkg/docker"
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

			clientView := tview.NewTextView()
			clientView.
				SetDynamicColors(true).
				SetScrollable(true).
				SetTitle("Client").
				SetBorder(true)

			// Set up input capture for views
			serverView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				return event
			})

			clientView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				return event
			})

			// Create flex layout
			flex := tview.NewFlex()
			flex.SetDirection(tview.FlexRow).
				AddItem(serverView, 0, 1, false).
				AddItem(clientView, 0, 1, false)

			var clientCmd *exec.Cmd

			// Setup key bindings
			app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				switch event.Key() {
				case tcell.KeyEsc:
					fallthrough
				case tcell.KeyRune:
					switch event.Rune() {
					case 'q':
						// Stop the application
						app.Stop()
						
						// Kill the client process if it exists
						if clientCmd != nil && clientCmd.Process != nil {
							if err := clientCmd.Process.Kill(); err != nil {
								fmt.Printf("Error killing client process: %v\n", err)
							}
						}
						
						// Stop server
						cleanup := exec.Command("pkill", "-f", "mattermost")
						if err := cleanup.Run(); err != nil {
							fmt.Printf("Error during server cleanup: %v\n", err)
						}

						// Stop docker services
						dockerManager, err := docker.NewManager()
						if err != nil {
							fmt.Printf("Error creating docker manager: %v\n", err)
						}
						if err := dockerManager.Stop(); err != nil {
							fmt.Printf("Error stopping docker services: %v\n", err)
						}
						
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
				}
				return event
			})

			// Start docker services
			dockerManager, err := docker.NewManager()
			if err != nil {
				return fmt.Errorf("failed to create docker manager: %w", err)
			}
			dockerManager.EnableService(docker.Minio)
			dockerManager.EnableService(docker.OpenLDAP)
			dockerManager.EnableService(docker.Elasticsearch)
			
			if err := dockerManager.Start(); err != nil {
				return fmt.Errorf("failed to start docker services: %w", err)
			}

			// Start server process using mmdev command
			serverCmd := exec.Command("mmdev", "server", "start", "--watch")
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
				return fmt.Errorf("failed to start client: %w", err)
			}

			// Function to handle output with auto-scroll
			handleOutput := func(view *tview.TextView, output *bufio.Scanner) {
				for output.Scan() {
					text := output.Text()
					app.QueueUpdateDraw(func() {
						_, _, _, height := view.GetInnerRect()
						row, _ := view.GetScrollOffset()
						content := view.GetText(false)
						lines := len(strings.Split(content, "\n"))
						
						fmt.Fprint(view, text+"\n")
						
						// Auto-scroll only if we're at the bottom
						if lines-row <= height {
							view.ScrollToEnd()
						}
					})
				}
			}

			// Handle server output
			go handleOutput(serverView, bufio.NewScanner(serverOut))

			// Handle client output
			go handleOutput(clientView, bufio.NewScanner(clientOut))

			// Run the application
			if err := app.SetRoot(flex, true).Run(); err != nil {
				return fmt.Errorf("application error: %w", err)
			}

			return nil
		},
	}


	return cmd
}
