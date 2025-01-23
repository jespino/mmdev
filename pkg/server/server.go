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
	baseDir           string
	enterpriseEnabled bool
	enterpriseDir     string
}

// NewManager creates a new server manager
func NewManager(baseDir string) *Manager {
	enterpriseDir := filepath.Join(baseDir, "..", "enterprise")
	_, err := os.Stat(enterpriseDir)
	enterpriseEnabled := err == nil

	return &Manager{
		baseDir:           baseDir,
		enterpriseEnabled: enterpriseEnabled,
		enterpriseDir:     enterpriseDir,
	}
}

// Start starts the Mattermost server and returns the command
func (m *Manager) Start() (*exec.Cmd, error) {
	if err := m.validateBaseDir(); err != nil {
		return nil, err
	}

	// Ensure webapp client dist exists
	distDir := filepath.Join(m.baseDir, "..", "webapp", "channels", "dist")
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("webapp dist directory not found at %s - please build the webapp first", distDir)
	}

	// Create symlink to client directory if it doesn't exist
	clientLink := filepath.Join(m.baseDir, "client")
	if _, err := os.Stat(clientLink); os.IsNotExist(err) {
		if err := os.Symlink(distDir, clientLink); err != nil {
			return nil, fmt.Errorf("failed to create client symlink: %w", err)
		}
	}

	// Ensure required directories exist
	for _, dir := range []string{
		filepath.Join(m.baseDir, "prepackaged_plugins"),
		filepath.Join(m.baseDir, "bin"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Get git hashes
	buildHash := "dev"
	buildHashEnterprise := "none"
	if hash, err := exec.Command("git", "rev-parse", "HEAD").Output(); err == nil {
		buildHash = strings.TrimSpace(string(hash))
	}
	if m.enterpriseEnabled {
		if hash, err := exec.Command("git", "-C", m.enterpriseDir, "rev-parse", "HEAD").Output(); err == nil {
			buildHashEnterprise = strings.TrimSpace(string(hash))
		}
	}

	// Set build flags
	ldflags := []string{
		"-X github.com/mattermost/mattermost/server/public/model.BuildNumber=dev",
		"-X github.com/mattermost/mattermost/server/public/model.BuildDate=dev",
		"-X github.com/mattermost/mattermost/server/public/model.BuildHash=" + buildHash,
		"-X github.com/mattermost/mattermost/server/public/model.BuildHashEnterprise=" + buildHashEnterprise,
		"-X github.com/mattermost/mattermost/server/public/model.BuildEnterpriseReady=" + fmt.Sprintf("%t", m.enterpriseEnabled),
	}

	buildTags := []string{"debug"}
	if m.enterpriseEnabled {
		buildTags = append(buildTags, "enterprise")
	}

	fmt.Println("Compiling...")

	// Build the server binary
	buildCmd := exec.Command("go", "build",
		"-ldflags", strings.Join(ldflags, " "),
		"-tags", strings.Join(buildTags, " "),
		"-o", "bin/mattermost",
		"./cmd/mattermost")
	buildCmd.Dir = m.baseDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to build server: %w", err)
	}

	// Run the compiled binary
	cmd := exec.Command("./bin/mattermost")

	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"MM_SERVICESETTINGS_SITEURL=http://localhost:8065",
		"MM_SERVICESETTINGS_LISTENADDRESS=:8065",
		"MM_SQLSETTINGS_DATASOURCE=postgres://mmuser:mostest@localhost/mattermost_test?sslmode=disable\u0026connect_timeout=10\u0026binary_parameters=yes",
		"MM_SQLSETTINGS_DRIVERNAME=postgres",
		"MM_LOGSETTINGS_ENABLECONSOLE=true",
		"MM_LOGSETTINGS_CONSOLELEVEL=DEBUG",
		"MM_LOGSETTINGS_ENABLEFILE=false",
		"MM_LOGSETTINGS_ENABLECOLOR=true",
		"MM_LOGSETTINGS_CONSOLEJSON=false",
		"MM_FILESETTINGS_DIRECTORY=data/",
		"MM_PLUGINSETTINGS_DIRECTORY=plugins",
		"MM_PLUGINSETTINGS_CLIENTDIRECTORY=client/plugins",
	)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}
	return cmd, nil
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
