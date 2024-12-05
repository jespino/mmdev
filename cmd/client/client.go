package client

import (
	"fmt"
	"os"

	"github.com/jespino/mmdev/pkg/webapp"
	"github.com/spf13/cobra"
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
	cmd := &cobra.Command{
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
				fmt.Printf("Linting found issues: %v\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().Bool("watch", false, "Watch for changes and rebuild")
	return cmd
}

func StartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the client",
		RunE: func(cmd *cobra.Command, args []string) error {
			webappDir := "./webapp"
			if _, err := os.Stat(webappDir); os.IsNotExist(err) {
				return fmt.Errorf("webapp directory not found at %s", webappDir)
			}

			watch, _ := cmd.Flags().GetBool("watch")
			manager := webapp.NewManager(webappDir)
			if err := manager.Start(watch); err != nil {
				return fmt.Errorf("failed to run client: %w", err)
			}

			return nil
		},
	}
	cmd.Flags().Bool("watch", false, "Watch for changes and rebuild")
	return cmd
}
