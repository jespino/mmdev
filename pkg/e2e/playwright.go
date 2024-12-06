package e2e

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

type PlaywrightRunner struct {
	client  *client.Client
	baseDir string
}

func NewPlaywrightRunner(baseDir string) (*PlaywrightRunner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &PlaywrightRunner{
		client:  cli,
		baseDir: baseDir,
	}, nil
}

func (r *PlaywrightRunner) RunTests() error {
	ctx := context.Background()

	// Get absolute path for tests directory
	absBaseDir, err := filepath.Abs(r.baseDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	if _, err := os.Stat(absBaseDir); os.IsNotExist(err) {
		return fmt.Errorf("mattermost directory not found at %s", absBaseDir)
	}

	// Pull the Playwright Docker image
	fmt.Println("Pulling Playwright Docker image...")
	_, err = r.client.ImagePull(ctx, "mcr.microsoft.com/playwright:v1.49.0-noble", types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull Playwright image: %w", err)
	}

	// Create container config
	config := &container.Config{
		Image:        "mcr.microsoft.com/playwright:v1.49.0-noble",
		Cmd:          []string{"sh", "-c", "npm install && npm run test"},
		Tty:          true,
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   "/mattermost/e2e-tests/playwright",
	}

	// Create host config with volume mount
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: absBaseDir,
				Target: "/mattermost",
			},
		},
		NetworkMode: "host",
	}

	// Create container
	resp, err := r.client.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := r.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Attach to container output
	out, err := r.client.ContainerAttach(ctx, resp.ID, types.ContainerAttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return fmt.Errorf("failed to attach to container: %w", err)
	}
	defer out.Close()

	// Copy container output to stdout in real time
	go func() {
		_, err := io.Copy(os.Stdout, out.Reader)
		if err != nil {
			fmt.Printf("Error copying output: %v\n", err)
		}
	}()

	// Wait for container to finish and get exit code
	statusCh, errCh := r.client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		// Get container logs
		out, err := r.client.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
		if err != nil {
			return fmt.Errorf("error getting container logs: %w", err)
		}
		defer out.Close()

		// Copy logs to stdout
		_, err = os.Stdout.ReadFrom(out)
		if err != nil {
			return fmt.Errorf("error reading container logs: %w", err)
		}

		// Remove container
		err = r.client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{})
		if err != nil {
			fmt.Printf("Warning: failed to remove container: %v\n", err)
		}

		if status.StatusCode != 0 {
			return fmt.Errorf("tests failed with exit code %d", status.StatusCode)
		}
	}

	return nil
}
