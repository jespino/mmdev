package generator

import (
	"fmt"
	"os"
	"os/exec"
)

// Manager handles code generation operations
type Manager struct {
	baseDir string
}

// NewManager creates a new generator manager
func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir: baseDir,
	}
}

// GenerateAppLayers generates the app layer interfaces
func (m *Manager) GenerateAppLayers() error {
	// Install struct2interface
	installCmd := exec.Command("go", "install", "github.com/reflog/struct2interface@v0.6.1")
	installCmd.Env = os.Environ()
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install struct2interface: %w", err)
	}

	// Generate app interface
	cmd := exec.Command("struct2interface",
		"-f", "channels/app",
		"-o", "channels/app/app_iface.go",
		"-p", "app",
		"-s", "App",
		"-i", "AppIface",
		"-t", "./channels/app/layer_generators/app_iface.go.tmpl")
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate app interface: %w", err)
	}

	// Generate opentracing layer
	cmd = exec.Command("go", "run",
		"./channels/app/layer_generators",
		"-in", "./channels/app/app_iface.go",
		"-out", "./channels/app/opentracing/opentracing_layer.go",
		"-template", "./channels/app/layer_generators/opentracing_layer.go.tmpl")
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate opentracing layer: %w", err)
	}

	return nil
}

// GenerateStoreLayers generates the store layer code
func (m *Manager) GenerateStoreLayers() error {
	cmd := exec.Command("go", "generate", "./channels/store")
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate store layers: %w", err)
	}
	return nil
}

// GeneratePluginAPI generates plugin API and hooks code
func (m *Manager) GeneratePluginAPI() error {
	cmd := exec.Command("go", "generate", "./public/plugin")
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate plugin API: %w", err)
	}
	return nil
}

// GenerateMocks generates all mock files
func (m *Manager) GenerateMocks() error {
	// Install mockery
	installCmd := exec.Command("go", "install", "github.com/vektra/mockery/v2/...@v2.42.2")
	installCmd.Env = os.Environ()
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install mockery: %w", err)
	}

	configs := []struct {
		name string
		path string
	}{
		{"store", "channels/store/.mockery.yaml"},
		{"cache", "platform/services/cache/.mockery.yaml"},
		{"telemetry", "platform/services/telemetry/.mockery.yaml"},
		{"filestore", "platform/shared/filestore/.mockery.yaml"},
		{"plugin", "public/plugin/.mockery.yaml"},
		{"einterfaces", "einterfaces/.mockery.yaml"},
		{"searchengine", "platform/services/searchengine/.mockery.yaml"},
		{"sharedchannel", "platform/services/sharedchannel/.mockery.yaml"},
		{"misc", "channels/utils/.mockery.yaml"},
		{"email", "channels/app/email/.mockery.yaml"},
		{"platform", "channels/app/platform/.mockery.yaml"},
	}

	for _, config := range configs {
		cmd := exec.Command("mockery", "--config", config.path)
		cmd.Dir = m.baseDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to generate %s mocks: %w", config.name, err)
		}
	}

	// Generate MMCTL mocks
	installCmd = exec.Command("go", "install", "github.com/golang/mock/mockgen@v1.6.0")
	installCmd.Env = os.Environ()
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install mockgen: %w", err)
	}

	cmd := exec.Command("mockgen",
		"-destination=cmd/mmctl/mocks/client_mock.go",
		"-copyright_file=cmd/mmctl/mocks/copyright.txt",
		"-package=mocks",
		"github.com/mattermost/mattermost/server/v8/cmd/mmctl/client",
		"Client")
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate mmctl mocks: %w", err)
	}

	return nil
}
