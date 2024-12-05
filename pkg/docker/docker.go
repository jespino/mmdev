package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Service represents a Docker service that can be enabled
type Service string

const (
	Minio          Service = "minio"
	OpenLDAP       Service = "openldap"
	Elasticsearch  Service = "elasticsearch"
	MySQLReplica   Service = "mysql-read-replica"
)

// Manager handles Docker operations
type Manager struct {
	baseDir     string
	composeFile string
	services    []Service
}

// NewManager creates a new Docker manager
func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir:     baseDir,
		composeFile: "docker-compose.makefile.yml",
		services:    make([]Service, 0),
	}
}

// EnableService adds a service to be started
func (m *Manager) EnableService(service Service) {
	m.services = append(m.services, service)
}

// Start starts the Docker services
func (m *Manager) Start() error {
	if err := m.validateBaseDir(); err != nil {
		return err
	}

	// Generate services list
	servicesList := make([]string, len(m.services))
	for i, svc := range m.services {
		servicesList[i] = string(svc)
	}

	// Build docker-compose command
	args := []string{
		"compose",
		"-f", m.composeFile,
	}

	// Add override file if it exists
	overridePath := filepath.Join(m.baseDir, "docker-compose.override.yaml")
	if _, err := os.Stat(overridePath); err == nil {
		args = append(args, "-f", "docker-compose.override.yaml")
	}

	// Add services and up command
	args = append(args, "up", "-d")
	args = append(args, servicesList...)

	cmd := exec.Command("docker", args...)
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start docker services: %w", err)
	}

	// Handle post-start configuration
	if err := m.postStartConfig(); err != nil {
		return fmt.Errorf("post-start configuration failed: %w", err)
	}

	return nil
}

// Stop stops all Docker services
func (m *Manager) Stop() error {
	if err := m.validateBaseDir(); err != nil {
		return err
	}

	cmd := exec.Command("docker", "compose", "-f", m.composeFile, "stop")
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop docker services: %w", err)
	}

	return nil
}

// Clean removes all Docker containers and volumes
func (m *Manager) Clean() error {
	if err := m.validateBaseDir(); err != nil {
		return err
	}

	cmd := exec.Command("docker", "compose", "-f", m.composeFile, "down", "-v")
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clean docker services: %w", err)
	}

	return nil
}

func (m *Manager) validateBaseDir() error {
	if _, err := os.Stat(filepath.Join(m.baseDir, m.composeFile)); os.IsNotExist(err) {
		return fmt.Errorf("docker-compose file not found in %s", m.baseDir)
	}
	return nil
}

func (m *Manager) postStartConfig() error {
	for _, service := range m.services {
		switch service {
		case OpenLDAP:
			if err := m.configureLDAP(); err != nil {
				return fmt.Errorf("LDAP configuration failed: %w", err)
			}
		case MySQLReplica:
			if err := m.configureMySQL(); err != nil {
				return fmt.Errorf("MySQL configuration failed: %w", err)
			}
		}
	}
	return nil
}

func (m *Manager) configureLDAP() error {
	ldifFile := "tests/ldap-data.ldif"
	if _, err := os.Stat(filepath.Join(m.baseDir, ldifFile)); os.IsNotExist(err) {
		return fmt.Errorf("LDAP data file not found: %s", ldifFile)
	}

	cmd := exec.Command("docker", "compose", "exec", "-T", "openldap",
		"ldapadd", "-x", "-D", "cn=admin,dc=mm,dc=test,dc=com", "-w", "mostest")
	cmd.Dir = m.baseDir
	cmd.Stdin, _ = os.Open(filepath.Join(m.baseDir, ldifFile))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Don't return error as this might fail if entries already exist
		fmt.Fprintf(os.Stderr, "Warning: LDAP configuration returned: %v\n", err)
	}

	return nil
}

func (m *Manager) configureMySQL() error {
	script := filepath.Join(m.baseDir, "scripts/replica-mysql-config.sh")
	if _, err := os.Stat(script); os.IsNotExist(err) {
		return fmt.Errorf("MySQL config script not found: %s", script)
	}

	cmd := exec.Command("/bin/sh", script)
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("MySQL configuration failed: %w", err)
	}

	return nil
}
