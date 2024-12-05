package start

import (
	"bufio"
	"fmt"
	"os/exec"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

var horizontal bool

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

			// Create flex layout
			flex := tview.NewFlex()
			if horizontal {
				flex.SetDirection(tview.FlexRow).
					AddItem(serverView, 0, 1, false).
					AddItem(clientView, 0, 1, false)
			} else {
				flex.SetDirection(tview.FlexColumn).
					AddItem(serverView, 0, 1, false).
					AddItem(clientView, 0, 1, false)
			}

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
						serverView.Write([]byte(text + "\n"))
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
					})
				}
			}()

			// Run the application
			if err := app.SetRoot(flex, true).Run(); err != nil {
				return fmt.Errorf("application error: %w", err)
			}

			// Cleanup on exit
			serverCmd.Process.Kill()
			clientCmd.Process.Kill()
			
			return nil
		},
	}

	cmd.Flags().BoolVarP(&horizontal, "horizontal", "h", false, "Split screen horizontally")
	cmd.Flags().BoolP("vertical", "v", true, "Split screen vertically (default)")

	return cmd
}
