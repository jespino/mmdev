package server

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func ServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Server related commands",
	}

	cmd.AddCommand(StartCmd())
	return cmd
}

func StartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the server",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverDir := "./server"
			if _, err := os.Stat(serverDir); os.IsNotExist(err) {
				return fmt.Errorf("server directory not found at %s", serverDir)
			}

			// Change to server directory
			if err := os.Chdir(serverDir); err != nil {
				return fmt.Errorf("failed to change to server directory: %w", err)
			}

			// Run make run-server
			makeCmd := exec.Command("make", "run-server")
			makeCmd.Stdout = os.Stdout
			makeCmd.Stderr = os.Stderr
			makeCmd.Env = os.Environ()

			if err := makeCmd.Run(); err != nil {
				return fmt.Errorf("failed to run server: %w", err)
			}

			return nil
		},
	}
}
