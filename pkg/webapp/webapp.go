package webapp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Manager handles webapp operations
type Manager struct {
	baseDir string
}

// NewManager creates a new webapp manager
func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir: baseDir,
	}
}

// Start starts the webapp development server
func (m *Manager) Start(watch bool) error {
	if err := m.validateBaseDir(); err != nil {
		return err
	}

	// Install dependencies if needed
	if err := m.ensureDependencies(); err != nil {
		return fmt.Errorf("failed to ensure dependencies: %w", err)
	}

	// Start the development server or build
	npmCmd := "build"
	if watch {
		npmCmd = "run"
	}
	cmd := exec.Command("npm", "run", npmCmd)
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	return cmd.Run()
}

// Lint runs ESLint on the webapp code
func (m *Manager) Lint() error {
	if err := m.validateBaseDir(); err != nil {
		return err
	}

	// Install dependencies if needed
	if err := m.ensureDependencies(); err != nil {
		return fmt.Errorf("failed to ensure dependencies: %w", err)
	}

	// Run ESLint
	cmd := exec.Command("npm", "run", "check")
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	return cmd.Run()
}

func (m *Manager) validateBaseDir() error {
	packageJSON := filepath.Join(m.baseDir, "package.json")
	if _, err := os.Stat(packageJSON); os.IsNotExist(err) {
		return fmt.Errorf("webapp source not found at %s", packageJSON)
	}
	return nil
}

func (m *Manager) ensureDependencies() error {
	// Check if node_modules exists
	if _, err := os.Stat(filepath.Join(m.baseDir, "node_modules")); err == nil {
		return nil
	}

	// Run nvm use first
	nvmCmd := exec.Command("bash", "-c", "source ~/.nvm/nvm.sh && nvm use")
	nvmCmd.Dir = m.baseDir
	nvmCmd.Stdout = os.Stdout
	nvmCmd.Stderr = os.Stderr
	nvmCmd.Env = os.Environ()

	if err := nvmCmd.Run(); err != nil {
		return fmt.Errorf("failed to run nvm use: %w", err)
	}

	// Install dependencies
	cmd := exec.Command("npm", "install")
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	return cmd.Run()
}
