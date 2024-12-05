package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	// Ensure webapp client dist exists
	distDir := filepath.Join(m.baseDir, "..", "webapp", "channels", "dist")
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		return fmt.Errorf("webapp dist directory not found at %s - please build the webapp first", distDir)
	}

	// Create symlink to client directory if it doesn't exist
	clientLink := filepath.Join(m.baseDir, "client")
	if _, err := os.Stat(clientLink); os.IsNotExist(err) {
		if err := os.Symlink(distDir, clientLink); err != nil {
			return fmt.Errorf("failed to create client symlink: %w", err)
		}
	}

	// Ensure prepackaged plugins directory exists
	pluginsDir := filepath.Join(m.baseDir, "prepackaged_plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("failed to create prepackaged plugins directory: %w", err)
	}

	// Set build flags
	ldflags := []string{
		"-X github.com/mattermost/mattermost/server/public/model.BuildNumber=dev",
		"-X github.com/mattermost/mattermost/server/public/model.BuildDate=dev",
		"-X github.com/mattermost/mattermost/server/public/model.BuildHash=dev",
		"-X github.com/mattermost/mattermost/server/public/model.BuildHashEnterprise=none",
		"-X github.com/mattermost/mattermost/server/public/model.BuildEnterpriseReady=false",
	}

	// Run the server
	cmd := exec.Command("go", append([]string{
		"run",
		"-ldflags", strings.Join(ldflags, " "),
		"-tags", "debug",
		"./cmd/mattermost",
	})...)
	
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"MM_SERVICESETTINGS_SITEURL=http://localhost:8065",
		"MM_SERVICESETTINGS_LISTENADDRESS=:8065",
		"MM_SQLSETTINGS_DATASOURCE=mmuser:mostest@tcp(localhost:3306)/mattermost_test?charset=utf8mb4,utf8&readTimeout=30s&writeTimeout=30s&timeout=30s",
		"MM_SQLSETTINGS_DRIVERNAME=mysql",
		"MM_LOGSETTINGS_ENABLECONSOLE=true",
		"MM_LOGSETTINGS_CONSOLELEVEL=DEBUG",
		"MM_LOGSETTINGS_ENABLEFILE=false",
		"MM_FILESETTINGS_DIRECTORY=data/",
		"MM_PLUGINSETTINGS_DIRECTORY=plugins",
		"MM_PLUGINSETTINGS_CLIENTDIRECTORY=client/plugins",
	)

	return cmd.Run()
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
