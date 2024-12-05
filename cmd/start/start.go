package start

import (
	"bufio"
	"fmt"
	"os/exec"

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
			serverView := tview.NewTextView().
				SetDynamicColors(true).
				SetScrollable(true).
				SetTitle("Server").
				SetBorder(true)

			clientView := tview.NewTextView().
				SetDynamicColors(true).
				SetScrollable(true).
				SetTitle("Client").
				SetBorder(true)

			// Track if views are at bottom for auto-scroll
			serverAtBottom := true
			clientAtBottom := true

			serverView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				_, _, _, height := serverView.GetInnerRect()
				_, y := serverView.GetScrollOffset()
				serverAtBottom = (y <= height)
				return event
			})

			clientView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				_, _, _, height := clientView.GetInnerRect()
				_, y := clientView.GetScrollOffset()
				clientAtBottom = (y <= height)
				return event
			})

			// Create flex layout
			flex := tview.NewFlex()
			flex.SetDirection(tview.FlexRow).
				AddItem(serverView, 0, 1, false).
				AddItem(clientView, 0, 1, false)

			// Setup key bindings
			app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				switch event.Key() {
				case tcell.KeyEsc:
					fallthrough
				case tcell.KeyRune:
					switch event.Rune() {
					case 'q':
						app.Stop()
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

			// Start server process
			serverCmd := exec.Command("make", "run-server")
			serverCmd.Dir = "server"
			serverOut, err := serverCmd.StdoutPipe()
			if err != nil {
				return fmt.Errorf("failed to create server stdout pipe: %w", err)
			}
			serverCmd.Stderr = serverCmd.Stdout

			// Start client process
			clientCmd := exec.Command("make", "run")
			clientCmd.Dir = "webapp"
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

			// Handle server output
			go func() {
				scanner := bufio.NewScanner(serverOut)
				for scanner.Scan() {
					text := scanner.Text()
					app.QueueUpdateDraw(func() {
						serverView.Write([]byte(text + "\n"))
						if serverAtBottom {
							serverView.ScrollToEnd()
						}
					})
				}
			}()

			// Handle client output
			go func() {
				scanner := bufio.NewScanner(clientOut)
				for scanner.Scan() {
					text := scanner.Text()
					app.QueueUpdateDraw(func() {
						clientView.Write([]byte(text + "\n"))
						if clientAtBottom {
							clientView.ScrollToEnd()
						}
					})
				}
			}()

			// Run the application
			if err := app.SetRoot(flex, true).Run(); err != nil {
				return fmt.Errorf("application error: %w", err)
			}

			// Cleanup on exit
			cleanup := exec.Command("make", "stop-server")
			cleanup.Dir = "server"
			if err := cleanup.Run(); err != nil {
				fmt.Printf("Error during server cleanup: %v\n", err)
			}
			
			if err := clientCmd.Process.Kill(); err != nil {
				fmt.Printf("Error killing client process: %v\n", err)
			}
			
			return nil
		},
	}


	return cmd
}
