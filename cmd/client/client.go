package client

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/jespino/mmdev/pkg/webapp"
)

func ClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client",
		Short: "Client related commands",
	}

	cmd.AddCommand(
		StartCmd(),
		LintCmd(),
	)
	return cmd
}

func LintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "Run linting on the client code",
		RunE: func(cmd *cobra.Command, args []string) error {
			webappDir := "./webapp"
			if _, err := os.Stat(webappDir); os.IsNotExist(err) {
				return fmt.Errorf("webapp directory not found at %s", webappDir)
			}

			// Change to webapp directory
			if err := os.Chdir(webappDir); err != nil {
				return fmt.Errorf("failed to change to webapp directory: %w", err)
			}

			manager := webapp.NewManager(webappDir)
			if err := manager.Lint(); err != nil {
				return fmt.Errorf("failed to run linting: %w", err)
			}

			return nil
		},
	}
}

func StartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the client",
		RunE: func(cmd *cobra.Command, args []string) error {
			webappDir := "./webapp"
			if _, err := os.Stat(webappDir); os.IsNotExist(err) {
				return fmt.Errorf("webapp directory not found at %s", webappDir)
			}

			// Change to webapp directory
			if err := os.Chdir(webappDir); err != nil {
				return fmt.Errorf("failed to change to webapp directory: %w", err)
			}

			manager := webapp.NewManager(webappDir)
			if err := manager.Start(); err != nil {
				return fmt.Errorf("failed to run client: %w", err)
			}

			return nil
		},
	}
}
