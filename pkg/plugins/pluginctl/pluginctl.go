package pluginctl

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/mattermost/mattermost/server/public/model"
)

// Client handles plugin control operations
type Client struct {
	client *model.Client4
}

// NewClient creates a new plugin control client
func NewClient(ctx context.Context) (*Client, error) {
	client, err := getClient(ctx)
	if err != nil {
		return nil, err
	}
	return &Client{client: client}, nil
}

// Deploy attempts to upload and enable a plugin
func (c *Client) Deploy(ctx context.Context, pluginID, bundlePath string) error {
	pluginBundle, err := os.Open(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", bundlePath, err)
	}
	defer pluginBundle.Close()

	_, _, err = c.client.UploadPluginForced(ctx, pluginBundle)
	if err != nil {
		return fmt.Errorf("failed to upload plugin bundle: %s", err.Error())
	}

	_, err = c.client.EnablePlugin(ctx, pluginID)
	if err != nil {
		return fmt.Errorf("failed to enable plugin: %s", err.Error())
	}

	return nil
}

// Disable attempts to disable the plugin
func (c *Client) Disable(ctx context.Context, pluginID string) error {
	_, err := c.client.DisablePlugin(ctx, pluginID)
	if err != nil {
		return fmt.Errorf("failed to disable plugin: %w", err)
	}
	return nil
}

// Enable attempts to enable the plugin
func (c *Client) Enable(ctx context.Context, pluginID string) error {
	_, err := c.client.EnablePlugin(ctx, pluginID)
	if err != nil {
		return fmt.Errorf("failed to enable plugin: %w", err)
	}
	return nil
}

// Reset attempts to reset the plugin by disabling and re-enabling it
func (c *Client) Reset(ctx context.Context, pluginID string) error {
	err := c.Disable(ctx, pluginID)
	if err != nil {
		return err
	}

	err = c.Enable(ctx, pluginID)
	if err != nil {
		return err
	}

	return nil
}

// GetLogs fetches and filters plugin logs
func (c *Client) GetLogs(ctx context.Context, pluginID string) error {
	return logs(ctx, c.client, pluginID)
}

// WatchLogs continuously fetches and displays plugin logs
func (c *Client) WatchLogs(ctx context.Context, pluginID string) error {
	return watchLogs(ctx, c.client, pluginID)
}

func getClient(ctx context.Context) (*model.Client4, error) {
	socketPath := os.Getenv("MM_LOCALSOCKETPATH")
	if socketPath == "" {
		socketPath = model.LocalModeSocketPath
	}

	client, connected := getUnixClient(socketPath)
	if connected {
		return client, nil
	}

	if os.Getenv("MM_LOCALSOCKETPATH") != "" {
		return nil, fmt.Errorf("no socket found at %s for local mode deployment", socketPath)
	}

	siteURL := os.Getenv("MM_SERVICESETTINGS_SITEURL")
	adminToken := os.Getenv("MM_ADMIN_TOKEN")
	adminUsername := os.Getenv("MM_ADMIN_USERNAME")
	adminPassword := os.Getenv("MM_ADMIN_PASSWORD")

	if siteURL == "" {
		return nil, errors.New("MM_SERVICESETTINGS_SITEURL is not set")
	}

	client = model.NewAPIv4Client(siteURL)

	if adminToken != "" {
		client.SetToken(adminToken)
		return client, nil
	}

	if adminUsername != "" && adminPassword != "" {
		client := model.NewAPIv4Client(siteURL)
		_, _, err := client.Login(ctx, adminUsername, adminPassword)
		if err != nil {
			return nil, fmt.Errorf("failed to login as %s: %w", adminUsername, err)
		}
		return client, nil
	}

	return nil, errors.New("one of MM_ADMIN_TOKEN or MM_ADMIN_USERNAME/MM_ADMIN_PASSWORD must be defined")
}

func getUnixClient(socketPath string) (*model.Client4, bool) {
	_, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, false
	}
	return model.NewAPIv4SocketClient(socketPath), true
}
