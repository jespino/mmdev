package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Manager handles server operations
type Manager struct {
	baseDir string
}

// NewManager creates a new server manager
func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir: baseDir,
	}
}

// Start starts the Mattermost server
func (m *Manager) Start() error {
	if err := m.validateBaseDir(); err != nil {
		return err
	}

	// Build the server
	buildCmd := exec.Command("go", "run", 
		"-ldflags", 
		"-X github.com/mattermost/mattermost/server/public/model.BuildNumber=dev " +
		"-X github.com/mattermost/mattermost/server/public/model.BuildDate=dev " +
		"-X github.com/mattermost/mattermost/server/public/model.BuildHash=dev " +
		"-X github.com/mattermost/mattermost/server/public/model.BuildHashEnterprise=none " +
		"-X github.com/mattermost/mattermost/server/public/model.BuildEnterpriseReady=false",
		"-tags", "debug",
		"./cmd/mattermost",
	)
	buildCmd.Dir = m.baseDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	buildCmd.Env = os.Environ()

	return buildCmd.Run()
}

// Stop stops the Mattermost server
func (m *Manager) Stop() error {
	if err := m.validateBaseDir(); err != nil {
		return err
	}

	// Find and kill the server process
	cmd := exec.Command("pkill", "-f", "mattermost")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Ignore error since it might just mean no process was found
		return nil
	}

	return nil
}

// Lint runs golangci-lint on the server code
func (m *Manager) Lint() error {
	if err := m.validateBaseDir(); err != nil {
		return err
	}

	// Install golangci-lint if not present
	installCmd := exec.Command("go", "install", "github.com/golangci/golangci-lint/cmd/golangci-lint@v1.57.1")
	installCmd.Env = os.Environ()
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install golangci-lint: %w", err)
	}

	// Run golangci-lint
	lintCmd := exec.Command("golangci-lint", "run", "./...")
	lintCmd.Dir = m.baseDir
	lintCmd.Stdout = os.Stdout
	lintCmd.Stderr = os.Stderr
	lintCmd.Env = os.Environ()

	if err := lintCmd.Run(); err != nil {
		return fmt.Errorf("linting failed: %w", err)
	}

	return nil
}

func (m *Manager) validateBaseDir() error {
	mainGo := filepath.Join(m.baseDir, "cmd", "mattermost", "main.go")
	if _, err := os.Stat(mainGo); os.IsNotExist(err) {
		return fmt.Errorf("server source not found at %s", mainGo)
	}
	return nil
}
