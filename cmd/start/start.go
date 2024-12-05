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

			// Create flex layout
			flex := tview.NewFlex()
			flex.SetDirection(tview.FlexRow).
				AddItem(serverView, 0, 1, false).
				AddItem(clientView, 0, 1, false)

			// Setup key bindings
			app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyEsc {
					app.Stop()
					return nil
				}
				return event
			})

			// Start server process
			serverCmd := exec.Command("make", "run-server")
			serverCmd.Dir = "./server"
			serverOut, err := serverCmd.StdoutPipe()
			if err != nil {
				return fmt.Errorf("failed to create server stdout pipe: %w", err)
			}
			serverCmd.Stderr = serverCmd.Stdout

			// Start client process
			clientCmd := exec.Command("make", "run")
			clientCmd.Dir = "./webapp"
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
						fmt.Fprintln(serverView, text)
					})
				}
			}()

			// Handle client output
			go func() {
				scanner := bufio.NewScanner(clientOut)
				for scanner.Scan() {
					text := scanner.Text()
					app.QueueUpdateDraw(func() {
						fmt.Fprintln(clientView, text)
					})
				}
			}()

			// Run the application
			if err := app.SetRoot(flex, true).Run(); err != nil {
				return fmt.Errorf("application error: %w", err)
			}

			// Cleanup on exit
			cleanup := exec.Command("make", "stop-server")
			cleanup.Dir = "./server"
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
