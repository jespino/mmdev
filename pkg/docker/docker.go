package docker

import (
	"context"
	"encoding/json"
	"fmt"
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
)

// ServiceConfig holds configuration for a Docker service
type ServiceConfig struct {
	Image        string
	ExposedPorts map[string]string // host:container
	Env          []string
	Volumes      map[string]string // host:container
}

var serviceConfigs = map[Service]ServiceConfig{
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

		// Check if image exists locally
		_, _, err = m.client.ImageInspectWithRaw(m.ctx, config.Image)
		if err != nil {
			// Image doesn't exist, pull it
			reader, err := m.client.ImagePull(m.ctx, config.Image, types.ImagePullOptions{})
			if err != nil {
				return fmt.Errorf("failed to pull image %s: %w", config.Image, err)
			}
			defer reader.Close()

			decoder := json.NewDecoder(reader)
			fmt.Printf("Pulling image %s\n", config.Image)

			layerStatus := make(map[string]string)
		for decoder.More() {
			var pullStatus struct {
				Status         string `json:"status"`
				ID            string `json:"id"`
				Progress      string `json:"progress"`
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
			for _, name := range container.Names {
				if name == "/"+containerName {
					existingContainer = &container
					break
				}
			}
		}

		var containerID string
		if existingContainer != nil {
			containerID = existingContainer.ID
			// If container exists but is not running, start it
			if existingContainer.State != "running" {
				if err := m.client.ContainerStart(m.ctx, containerID, types.ContainerStartOptions{}); err != nil {
					return fmt.Errorf("failed to start existing container %s: %w", containerName, err)
				}
				fmt.Printf("Started existing %s container\n", service)
			} else {
				fmt.Printf("Container %s is already running\n", service)
			}
		} else {
			// Create new container
			containerConfig := &containerTypes.Config{
				Image:        config.Image,
				Env:          config.Env,
				ExposedPorts: exposedPorts,
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
	time.Sleep(5 * time.Second)

	return nil
}

// Stop stops all Docker services
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

// Clean removes all Docker containers and volumes
func (m *Manager) Clean() error {
	return m.Stop()
}
