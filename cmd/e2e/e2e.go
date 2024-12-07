package e2e

import (
	"fmt"
	"github.com/jespino/mmdev/pkg/docker"
	"github.com/jespino/mmdev/pkg/e2e"
	"github.com/spf13/cobra"
)

func E2ECmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "e2e",
		Short: "E2E testing related commands",
	}

	cmd.AddCommand(
		PlaywrightCmd(),
		CypressCmd(),
	)
	return cmd
}

func PlaywrightCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "playwright",
		Short: "Playwright E2E testing commands",
	}

	cmd.AddCommand(
		PlaywrightRunCmd(),
		PlaywrightUICmd(),
		PlaywrightReportCmd(),
	)
	return cmd
}

func PlaywrightRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run Playwright E2E tests",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Ensure Docker image is available
			dockerManager, err := docker.NewManager()
			if err != nil {
				return fmt.Errorf("failed to create docker manager: %w", err)
			}
			if err := dockerManager.EnsurePlaywrightImage(); err != nil {
				return fmt.Errorf("failed to ensure playwright image: %w", err)
			}

			// Create and run the tests
			runner, err := e2e.NewPlaywrightRunner(".", "run")
			if err != nil {
				return fmt.Errorf("failed to create playwright runner: %w", err)
			}
			return runner.RunTests()
		},
	}
	return cmd
}

func PlaywrightUICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Open Playwright UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Ensure Docker image is available
			dockerManager, err := docker.NewManager()
			if err != nil {
				return fmt.Errorf("failed to create docker manager: %w", err)
			}
			if err := dockerManager.EnsurePlaywrightImage(); err != nil {
				return fmt.Errorf("failed to ensure playwright image: %w", err)
			}

			// Create and run the UI
			runner, err := e2e.NewPlaywrightRunner(".", "ui")
			if err != nil {
				return fmt.Errorf("failed to create playwright runner: %w", err)
			}
			return runner.RunTests()
		},
	}
	return cmd
}

func PlaywrightReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Show Playwright test report",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Ensure Docker image is available
			dockerManager, err := docker.NewManager()
			if err != nil {
				return fmt.Errorf("failed to create docker manager: %w", err)
			}
			if err := dockerManager.EnsurePlaywrightImage(); err != nil {
				return fmt.Errorf("failed to ensure playwright image: %w", err)
			}

			// Create and show the report
			runner, err := e2e.NewPlaywrightRunner(".", "report")
			if err != nil {
				return fmt.Errorf("failed to create playwright runner: %w", err)
			}
			return runner.RunTests()
		},
	}
	return cmd
}

func CypressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cypress", 
		Short: "Run Cypress E2E tests",
		RunE: func(cmd *cobra.Command, args []string) error {
			panic("Cypress tests not yet implemented")
		},
	}
	return cmd
}
