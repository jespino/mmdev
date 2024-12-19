package e2e

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/jespino/mmdev/pkg/docker"
	"github.com/jespino/mmdev/pkg/e2e"
	"github.com/spf13/cobra"
)

func E2ECmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "e2e",
		Short: "E2E testing related commands",
		Annotations: map[string]string{
			"requiresMMRepo": "true",
		},
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
			// Change to playwright directory
			if err := os.Chdir("e2e-tests/playwright"); err != nil {
				return fmt.Errorf("failed to change to playwright directory: %w", err)
			}

			// Run npm install if needed
			if _, err := os.Stat("node_modules"); os.IsNotExist(err) {
				installCmd := exec.Command("bash", "-c", "source ~/.nvm/nvm.sh && nvm use && npm install")
				installCmd.Stdout = os.Stdout
				installCmd.Stderr = os.Stderr
				if err := installCmd.Run(); err != nil {
					return fmt.Errorf("failed to install dependencies: %w", err)
				}
			}

			// Run playwright UI
			runCmd := exec.Command("bash", "-c", "source ~/.nvm/nvm.sh && nvm use && npm run playwright-ui")
			runCmd.Stdout = os.Stdout
			runCmd.Stderr = os.Stderr
			return runCmd.Run()
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
		Short: "Cypress E2E testing commands",
	}

	cmd.AddCommand(
		CypressRunCmd(),
		CypressUICmd(),
		CypressReportCmd(),
	)
	return cmd
}

func CypressRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run Cypress E2E tests",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Change to cypress directory
			if err := os.Chdir("e2e-tests/cypress"); err != nil {
				return fmt.Errorf("failed to change to cypress directory: %w", err)
			}

			// Run npm install if needed
			if _, err := os.Stat("node_modules"); os.IsNotExist(err) {
				installCmd := exec.Command("bash", "-c", "source ~/.nvm/nvm.sh && nvm use && npm install")
				installCmd.Stdout = os.Stdout
				installCmd.Stderr = os.Stderr
				if err := installCmd.Run(); err != nil {
					return fmt.Errorf("failed to install dependencies: %w", err)
				}
			}

			// Run cypress tests
			runCmd := exec.Command("bash", "-c", "source ~/.nvm/nvm.sh && nvm use && npm run cypress:run")
			runCmd.Stdout = os.Stdout
			runCmd.Stderr = os.Stderr
			return runCmd.Run()
		},
	}
	return cmd
}

func CypressUICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Open Cypress UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Change to cypress directory
			if err := os.Chdir("e2e-tests/cypress"); err != nil {
				return fmt.Errorf("failed to change to cypress directory: %w", err)
			}

			// Run npm install if needed
			if _, err := os.Stat("node_modules"); os.IsNotExist(err) {
				installCmd := exec.Command("bash", "-c", "source ~/.nvm/nvm.sh && nvm use && npm install")
				installCmd.Stdout = os.Stdout
				installCmd.Stderr = os.Stderr
				if err := installCmd.Run(); err != nil {
					return fmt.Errorf("failed to install dependencies: %w", err)
				}
			}

			// Run cypress UI
			runCmd := exec.Command("bash", "-c", "source ~/.nvm/nvm.sh && nvm use && npm run cypress:open")
			runCmd.Stdout = os.Stdout
			runCmd.Stderr = os.Stderr
			return runCmd.Run()
		},
	}
	return cmd
}

func CypressReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Show Cypress test report",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Change to cypress directory
			if err := os.Chdir("e2e-tests/cypress"); err != nil {
				return fmt.Errorf("failed to change to cypress directory: %w", err)
			}

			// Check if report exists
			if _, err := os.Stat("results/mochawesome-report/mochawesome.html"); os.IsNotExist(err) {
				return fmt.Errorf("no test report found - please run the tests first")
			}

			// Open the report in default browser
			openCmd := exec.Command("xdg-open", "results/mochawesome-report/mochawesome.html")
			return openCmd.Run()
		},
	}
	return cmd
}
