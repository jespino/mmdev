package webapp

import (
	"fmt"
	"os"

	"github.com/jespino/mmdev/pkg/webapp"
	"github.com/spf13/cobra"
)

func WebappCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webapp",
		Short: "Webapp related commands",
		Annotations: map[string]string{
			"requiresMMRepo": "true",
		},
	}

	cmd.AddCommand(
		StartCmd(),
		LintCmd(),
		FixCmd(),
	)
	return cmd
}

func FixCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix",
		Short: "Run auto-fix on the webapp code",
		RunE: func(cmd *cobra.Command, args []string) error {
			webappDir := "./webapp"
			if _, err := os.Stat(webappDir); os.IsNotExist(err) {
				return fmt.Errorf("webapp directory not found at %s", webappDir)
			}

			manager := webapp.NewManager(webappDir)
			if err := manager.Fix(); err != nil {
				fmt.Printf("Fix found issues: %v\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
	return cmd
}

func LintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Run linting on the webapp code",
		RunE: func(cmd *cobra.Command, args []string) error {
			webappDir := "./webapp"
			if _, err := os.Stat(webappDir); os.IsNotExist(err) {
				return fmt.Errorf("webapp directory not found at %s", webappDir)
			}

			manager := webapp.NewManager(webappDir)
			if err := manager.Lint(); err != nil {
				return fmt.Errorf("linting found issues: %v", err)
			}
			return nil
		},
	}
	return cmd
}

func StartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the webapp",
		RunE: func(cmd *cobra.Command, args []string) error {
			webappDir := "./webapp"
			if _, err := os.Stat(webappDir); os.IsNotExist(err) {
				return fmt.Errorf("webapp directory not found at %s", webappDir)
			}

			watch, _ := cmd.Flags().GetBool("watch")
			manager := webapp.NewManager(webappDir)
			if err := manager.Start(watch); err != nil {
				return fmt.Errorf("failed to run webapp: %w", err)
			}

			return nil
		},
	}
	cmd.Flags().Bool("watch", false, "Watch for changes and rebuild")
	return cmd
}
