package docker

import (
	"context"
	"fmt"
	"io"
	"os"
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

		// Pull image
		reader, err := m.client.ImagePull(m.ctx, config.Image, types.ImagePullOptions{})
		if err != nil {
			return fmt.Errorf("failed to pull image %s: %w", config.Image, err)
		}
		io.Copy(os.Stdout, reader)
		reader.Close()

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

		// Create container
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

		containerName := fmt.Sprintf("mmdev-%s", service)
		resp, err := m.client.ContainerCreate(m.ctx, containerConfig, hostConfig, networkingConfig, nil, containerName)
		if err != nil {
			return fmt.Errorf("failed to create container %s: %w", containerName, err)
		}

		if err := m.client.ContainerStart(m.ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
			return fmt.Errorf("failed to start container %s: %w", containerName, err)
		}

		fmt.Printf("Started %s container\n", service)
	}

	// Wait for containers to be ready
	time.Sleep(5 * time.Second)

	return nil
}

// Stop stops all Docker services
func (m *Manager) Stop() error {
	containers, err := m.client.ContainerList(m.ctx, types.ContainerListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for _, container := range containers {
		if container.Labels["com.docker.compose.project"] == "mmdev" {
			fmt.Printf("Stopping container %s\n", container.Names[0])
			if err := m.client.ContainerStop(m.ctx, container.ID, containerTypes.StopOptions{Timeout: new(int)}); err != nil {
				return fmt.Errorf("failed to stop container %s: %w", container.Names[0], err)
			}
			if err := m.client.ContainerRemove(m.ctx, container.ID, types.ContainerRemoveOptions{}); err != nil {
				return fmt.Errorf("failed to remove container %s: %w", container.Names[0], err)
			}
		}
	}

	if m.networkID != "" {
		if err := m.client.NetworkRemove(m.ctx, m.networkID); err != nil {
			return fmt.Errorf("failed to remove network: %w", err)
		}
	}

	return nil
}

// Clean removes all Docker containers and volumes
func (m *Manager) Clean() error {
	return m.Stop()
}