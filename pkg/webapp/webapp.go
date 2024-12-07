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
	cmd := exec.Command("bash", "-c", "source ~/.nvm/nvm.sh && nvm use && npm run "+npmCmd)
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

	// Run ESLint once
	cmd := exec.Command("bash", "-c", "source ~/.nvm/nvm.sh && nvm use && npm check --no-cache")
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("eslint check failed: %w", err)
	}
	return nil
}

// Fix runs ESLint fix on the webapp code
func (m *Manager) Fix() error {
	if err := m.validateBaseDir(); err != nil {
		return err
	}

	// Install dependencies if needed
	if err := m.ensureDependencies(); err != nil {
		return fmt.Errorf("failed to ensure dependencies: %w", err)
	}

	// Run ESLint fix
	cmd := exec.Command("bash", "-c", "source ~/.nvm/nvm.sh && nvm use && npm run fix")
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
	// Install dependencies
	cmd := exec.Command("bash", "-c", "source ~/.nvm/nvm.sh && nvm use && npm install")
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	return cmd.Run()
}
