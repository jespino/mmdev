package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	containerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// Service represents a Docker service that can be enabled
type Service string

const (
	Minio         Service = "minio"
	OpenLDAP      Service = "openldap"
	Elasticsearch Service = "elasticsearch"
	MySQLReplica  Service = "mysql-read-replica"
	Postgres      Service = "postgres"
	Inbucket      Service = "inbucket"
	Redis         Service = "redis"
	Playwright    Service = "playwright"
)

// ServiceConfig holds configuration for a Docker service
type ServiceConfig struct {
	Image        string
	ExposedPorts map[string]string // host:container
	Env          []string
	Volumes      map[string]string // host:container
	Command      []string
}

func (m *Manager) waitForPort(containerName, port string) error {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%s", port), time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("timeout waiting for port %s", port)
}

func (m *Manager) waitForElasticsearch() error {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://localhost:9200/_cluster/health")
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("elasticsearch is not ready")
}

func (m *Manager) waitForMinio() error {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://localhost:9000/minio/health/live")
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("minio is not ready")
}

var serviceConfigs = map[Service]ServiceConfig{
	Playwright: {
		Image: "mcr.microsoft.com/playwright:v1.49.0-noble",
	},
	Postgres: {
		Image: "postgres:13",
		ExposedPorts: map[string]string{
			"5432": "5432",
		},
		Env: []string{
			"POSTGRES_USER=mmuser",
			"POSTGRES_PASSWORD=mostest",
			"POSTGRES_DB=mattermost_test",
		},
	},
	Inbucket: {
		Image: "inbucket/inbucket:3.0.3",
		ExposedPorts: map[string]string{
			"10000": "9000", // Web UI
			"10025": "2500", // SMTP
			"1100":  "1100", // POP3
		},
	},
	Redis: {
		Image: "redis:7",
		ExposedPorts: map[string]string{
			"6379": "6379",
		},
	},
	Minio: {
		Image: "minio/minio:RELEASE.2024-03-03T17-50-39Z",
		ExposedPorts: map[string]string{
			"9000": "9000",
			"9001": "9001",
		},
		Env: []string{
			"MINIO_ROOT_USER=minioadmin",
			"MINIO_ROOT_PASSWORD=minioadmin",
		},
		Volumes: map[string]string{
			"/tmp/minio/data": "/data",
		},
		Command: []string{"server", "/data", "--console-address", ":9001"},
	},
	OpenLDAP: {
		Image: "osixia/openldap:1.5.0",
		ExposedPorts: map[string]string{
			"389": "389",
			"636": "636",
		},
		Env: []string{
			"LDAP_ORGANISATION=Mattermost Test",
			"LDAP_DOMAIN=mm.test.com",
			"LDAP_ADMIN_PASSWORD=mostest",
		},
	},
	Elasticsearch: {
		Image: "elasticsearch:7.17.10",
		ExposedPorts: map[string]string{
			"9200": "9200",
		},
		Env: []string{
			"discovery.type=single-node",
			"xpack.security.enabled=false",
		},
	},
}

// Manager handles Docker operations
type Manager struct {
	client    *client.Client
	ctx       context.Context
	services  []Service
	networkID string
}

// EnsureImage ensures a Docker image is available locally, pulling it if needed
func (m *Manager) EnsureImage(image string) error {
	// Check if image exists locally
	_, _, err := m.client.ImageInspectWithRaw(m.ctx, image)
	if err == nil {
		return nil // Image exists
	}

	// Image doesn't exist, pull it
	reader, err := m.client.ImagePull(m.ctx, image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", image, err)
	}
	defer reader.Close()

	decoder := json.NewDecoder(reader)
	fmt.Printf("Pulling image %s\n", image)

	layerStatus := make(map[string]string)
	for decoder.More() {
		var pullStatus struct {
			Status         string `json:"status"`
			ID             string `json:"id"`
			Progress       string `json:"progress"`
			ProgressDetail struct {
				Current int64 `json:"current"`
				Total   int64 `json:"total"`
			} `json:"progressDetail"`
		}

		if err := decoder.Decode(&pullStatus); err != nil {
			return fmt.Errorf("failed to decode pull status: %w", err)
		}

		if pullStatus.ID != "" {
			layerStatus[pullStatus.ID] = pullStatus.Status

			// Move cursor to bottom of terminal
			fmt.Print("\033[9999B")

			// Clear lines and move up
			for range layerStatus {
				fmt.Print("\033[2K\033[A")
			}

			// Print current status in a stable order
			var layerIDs []string
			for id := range layerStatus {
				layerIDs = append(layerIDs, id)
			}
			sort.Strings(layerIDs)

			for _, id := range layerIDs {
				progress := ""
				if strings.Contains(layerStatus[id], "Download") {
					if pullStatus.ID == id {
						progress = pullStatus.Progress
					} else if strings.Contains(layerStatus[id], "complete") {
						progress = "[=========================>] 100%"
					}
				}
				fmt.Printf("%s: %s %s\n", id, layerStatus[id], progress)
			}
		} else {
			// Move cursor to bottom and print status
			fmt.Print("\033[9999B")
			fmt.Printf("%s\n", pullStatus.Status)
			fmt.Print("\033[A")
		}
	}
	fmt.Println()
	return nil
}

// NewManager creates a new Docker manager
func NewManager() (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Manager{
		client:   cli,
		ctx:      context.Background(),
		services: make([]Service, 0),
	}, nil
}

// EnableService adds a service to be started
func (m *Manager) EnableService(service Service) {
	m.services = append(m.services, service)
}

// SetupDefaultServices configures the manager with the default set of services
func (m *Manager) SetupDefaultServices() {
	m.EnableService(Minio)
	m.EnableService(OpenLDAP)
	m.EnableService(Elasticsearch)
	m.EnableService(Postgres)
	m.EnableService(Inbucket)
	m.EnableService(Redis)
}

// Start starts the Docker services
func (m *Manager) Start() error {
	// Create network if it doesn't exist
	networkName := "mmdev-network"
	networks, err := m.client.NetworkList(m.ctx, types.NetworkListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	networkExists := false
	for _, network := range networks {
		if network.Name == networkName {
			m.networkID = network.ID
			networkExists = true
			break
		}
	}

	if !networkExists {
		networkResponse, err := m.client.NetworkCreate(m.ctx, networkName, types.NetworkCreate{
			Driver: "bridge",
		})
		if err != nil {
			return fmt.Errorf("failed to create network: %w", err)
		}
		m.networkID = networkResponse.ID
	}

	for _, service := range m.services {
		config, ok := serviceConfigs[service]
		if !ok {
			return fmt.Errorf("no configuration found for service %s", service)
		}

		if err := m.EnsureImage(config.Image); err != nil {
			return fmt.Errorf("failed to ensure image %s: %w", config.Image, err)
		}

		// Create port bindings
		portBindings := nat.PortMap{}
		exposedPorts := nat.PortSet{}
		for hostPort, containerPort := range config.ExposedPorts {
			port := nat.Port(fmt.Sprintf("%s/tcp", containerPort))
			portBindings[port] = []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: hostPort,
				},
			}
			exposedPorts[port] = struct{}{}
		}

		// Create volume bindings
		var binds []string
		for hostPath, containerPath := range config.Volumes {
			binds = append(binds, fmt.Sprintf("%s:%s", hostPath, containerPath))
		}

		containerName := fmt.Sprintf("mmdev-%s", service)

		// Check if container already exists
		containers, err := m.client.ContainerList(m.ctx, types.ContainerListOptions{All: true})
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}

		var existingContainer *types.Container
		for _, container := range containers {
			if len(container.Names) == 1 && container.Names[0] == fmt.Sprintf("/mmdev-%s", service) {
				existingContainer = &container
				break
			}
		}

		var containerID string
		if existingContainer != nil {
			containerID = existingContainer.ID

			// Get detailed container info to check actual state
			inspect, err := m.client.ContainerInspect(m.ctx, containerID)
			if err != nil {
				return fmt.Errorf("failed to inspect container %s: %w", containerName, err)
			}

			if inspect.State.Running {
				fmt.Printf("Container %s is already running\n", service)
			} else {
				fmt.Printf("Starting existing %s container\n", service)
				if err := m.client.ContainerStart(m.ctx, containerID, types.ContainerStartOptions{}); err != nil {
					// If we can't start it, remove and recreate
					if err := m.client.ContainerRemove(m.ctx, containerID, types.ContainerRemoveOptions{Force: true}); err != nil {
						return fmt.Errorf("failed to remove container %s: %w", containerName, err)
					}
					existingContainer = nil
				}
			}
		}

		if existingContainer == nil {
			// Create new container
			containerConfig := &containerTypes.Config{
				Image:        config.Image,
				Env:          config.Env,
				ExposedPorts: exposedPorts,
				Cmd:          config.Command,
			}

			hostConfig := &containerTypes.HostConfig{
				PortBindings: portBindings,
				Binds:        binds,
			}

			networkingConfig := &network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{
					networkName: {
						NetworkID: m.networkID,
					},
				},
			}

			resp, err := m.client.ContainerCreate(m.ctx, containerConfig, hostConfig, networkingConfig, nil, containerName)
			if err != nil {
				return fmt.Errorf("failed to create container %s: %w", containerName, err)
			}
			containerID = resp.ID

			if err := m.client.ContainerStart(m.ctx, containerID, types.ContainerStartOptions{}); err != nil {
				return fmt.Errorf("failed to start container %s: %w", containerName, err)
			}
			fmt.Printf("Created and started new %s container\n", service)
		}
	}

	// Wait for containers to be ready
	fmt.Println("Waiting for services to be ready...")
	for _, service := range m.services {
		config, ok := serviceConfigs[service]
		if !ok {
			continue
		}

		containerName := fmt.Sprintf("mmdev-%s", service)

		// Skip waiting for Inbucket
		if service != Inbucket {
			for hostPort := range config.ExposedPorts {
				if err := m.waitForPort(containerName, hostPort); err != nil {
					return fmt.Errorf("service %s failed to become ready: %w", service, err)
				}
			}
			fmt.Printf("Service %s ports are ready\n", service)
		}

		// Additional service-specific health checks
		switch service {
		case Elasticsearch:
			if err := m.waitForElasticsearch(); err != nil {
				return fmt.Errorf("elasticsearch failed health check: %w", err)
			}
			fmt.Println("Elasticsearch is fully ready")
		case Minio:
			if err := m.waitForMinio(); err != nil {
				return fmt.Errorf("minio failed health check: %w", err)
			}
			fmt.Println("MinIO is fully ready")
		}
	}

	fmt.Println("All services are ready!")
	return nil
}

// Stop stops all Docker services without removing containers
func (m *Manager) Stop() error {
	containers, err := m.client.ContainerList(m.ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for _, container := range containers {
		// Check for our container name prefix
		for _, name := range container.Names {
			if strings.HasPrefix(name, "/mmdev-") {
				fmt.Printf("Stopping container %s\n", name)
				if err := m.client.ContainerStop(m.ctx, container.ID, containerTypes.StopOptions{Timeout: new(int)}); err != nil {
					// Only log stop errors since container might already be stopped
					fmt.Printf("Warning: failed to stop container %s: %v\n", name, err)
				}
				break
			}
		}
	}

	return nil
}

// Clean removes all Docker containers and volumes
// EnsurePlaywrightImage ensures the Playwright Docker image is available
func (m *Manager) EnsurePlaywrightImage() error {
	config, ok := serviceConfigs[Playwright]
	if !ok {
		return fmt.Errorf("no configuration found for Playwright service")
	}
	return m.EnsureImage(config.Image)
}

func (m *Manager) Clean() error {
	containers, err := m.client.ContainerList(m.ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for _, container := range containers {
		// Check for our container name prefix
		for _, name := range container.Names {
			if strings.HasPrefix(name, "/mmdev-") {
				fmt.Printf("Stopping container %s\n", name)
				if err := m.client.ContainerStop(m.ctx, container.ID, containerTypes.StopOptions{Timeout: new(int)}); err != nil {
					// Only log stop errors since container might already be stopped
					fmt.Printf("Warning: failed to stop container %s: %v\n", name, err)
				}
				if err := m.client.ContainerRemove(m.ctx, container.ID, types.ContainerRemoveOptions{}); err != nil {
					return fmt.Errorf("failed to remove container %s: %w", name, err)
				}
				break
			}
		}
	}

	if m.networkID != "" {
		// Try to remove network, but don't fail if it's in use
		if err := m.client.NetworkRemove(m.ctx, m.networkID); err != nil {
			fmt.Printf("Warning: failed to remove network: %v\n", err)
		}
	}

	return nil
}
