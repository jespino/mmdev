package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	baseDir      string
	composeFile  string
	services     []Service
	composeCmd   []string
}

// NewManager creates a new Docker manager
func NewManager(baseDir string) *Manager {
	// Check if docker-compose command exists
	_, err := exec.LookPath("docker-compose")
	composeCmd := []string{"docker", "compose"}
	if err == nil {
		composeCmd = []string{"docker-compose"}
	}

	return &Manager{
		baseDir:     baseDir,
		composeFile: "docker-compose.makefile.yml",
		services:    make([]Service, 0),
		composeCmd:  composeCmd,
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

	// Build compose command
	args := append(m.composeCmd[1:], "--file", m.composeFile)

	// Add override file if it exists
	overridePath := filepath.Join(m.baseDir, "docker-compose.override.yaml")
	if _, err := os.Stat(overridePath); err == nil {
		args = append(args, "--file", "docker-compose.override.yaml")
	}

	// Clean up any existing networks first
	cleanCmd := exec.Command(m.composeCmd[0], append(m.composeCmd[1:], "--file", m.composeFile, "down", "--remove-orphans")...)
	cleanCmd.Dir = m.baseDir
	cleanCmd.Stdout = os.Stdout
	cleanCmd.Stderr = os.Stderr
	if err := cleanCmd.Run(); err != nil {
		fmt.Printf("Warning: Network cleanup returned: %v\n", err)
	}

	// Add services and up command with unique network name
	args = append(args, "up", "-d", "--force-recreate")
	args = append(args, servicesList...)

	cmd := exec.Command(m.composeCmd[0], args...)
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Starting Docker services: %v\n", servicesList)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start docker services: %w", err)
	}
	fmt.Printf("Docker services started successfully\n")

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

	args := append(m.composeCmd[1:], "--file", m.composeFile, "stop")
	cmd := exec.Command(m.composeCmd[0], args...)
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Stopping Docker services...\n")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop docker services: %w", err)
	}
	fmt.Printf("Docker services stopped successfully\n")

	return nil
}

// Clean removes all Docker containers and volumes
func (m *Manager) Clean() error {
	if err := m.validateBaseDir(); err != nil {
		return err
	}

	args := append(m.composeCmd[1:], "--file", m.composeFile, "down", "--volumes")
	cmd := exec.Command(m.composeCmd[0], args...)
	cmd.Dir = m.baseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Cleaning up Docker services and volumes...\n")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clean docker services: %w", err)
	}
	fmt.Printf("Docker services and volumes cleaned successfully\n")

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

	args := append(m.composeCmd[1:], "exec", "-T", "openldap")
	cmd := exec.Command(m.composeCmd[0], append(args,
		"ldapadd", "-x", "-D", "cn=admin,dc=mm,dc=test,dc=com", "-w", "mostest")...)
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
