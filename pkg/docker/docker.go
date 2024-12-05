package docker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Service represents a docker service configuration
type Service struct {
	Name       string
	Enabled    bool
	DependsOn  []string
}

// Manager handles docker compose operations
type Manager struct {
	services []Service
	ctx     context.Context
}

// NewManager creates a new docker manager
func NewManager(ctx context.Context) *Manager {
	return &Manager{
		ctx: ctx,
		services: []Service{
			{Name: "mysql", Enabled: true},
			{Name: "postgres", Enabled: true},
			{Name: "minio", Enabled: true},
			{Name: "elasticsearch", Enabled: true},
			{Name: "openldap", Enabled: true},
			{Name: "inbucket", Enabled: true},
			{Name: "keycloak", Enabled: true},
		},
	}
}

// StartServices starts the specified docker services
func (m *Manager) StartServices(services ...string) error {
	args := []string{"compose", "-f", "docker-compose.yml"}
	
	// If no specific services are requested, start all enabled services
	if len(services) == 0 {
		for _, service := range m.services {
			if service.Enabled {
				services = append(services, service.Name)
			}
		}
	}
	
	args = append(args, "up", "-d")
	args = append(args, services...)

	cmd := exec.CommandContext(m.ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start services: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// StopServices stops the specified docker services
func (m *Manager) StopServices(services ...string) error {
	args := []string{"compose", "-f", "docker-compose.yml"}
	
	if len(services) == 0 {
		args = append(args, "down")
	} else {
		args = append(args, "stop")
		args = append(args, services...)
	}

	cmd := exec.CommandContext(m.ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop services: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// IsServiceRunning checks if a specific service is running
func (m *Manager) IsServiceRunning(service string) (bool, error) {
	cmd := exec.CommandContext(m.ctx, "docker", "compose", "-f", "docker-compose.yml", "ps", "--format", "json", service)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to check service status: %w", err)
	}

	return strings.Contains(string(output), `"State":"running"`), nil
}
