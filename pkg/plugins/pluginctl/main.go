package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/yourusername/yourproject/pkg/plugins/pluginctl"
)

const commandTimeout = 120 * time.Second

const helpText = `
Usage:
    pluginctl deploy <plugin id> <bundle path>
    pluginctl disable <plugin id>
    pluginctl enable <plugin id>
    pluginctl reset <plugin id>
    pluginctl logs <plugin id>
    pluginctl logs-watch <plugin id>
`

func main() {
	err := run()
	if err != nil {
		fmt.Printf("Failed: %s\n", err.Error())
		fmt.Print(helpText)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("invalid number of arguments")
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	client, err := pluginctl.NewClient(ctx)
	if err != nil {
		return err
	}

	switch os.Args[1] {
	case "deploy":
		if len(os.Args) < 4 {
			return fmt.Errorf("invalid number of arguments")
		}
		return client.Deploy(ctx, os.Args[2], os.Args[3])
	case "disable":
		return client.Disable(ctx, os.Args[2])
	case "enable":
		return client.Enable(ctx, os.Args[2])
	case "reset":
		return client.Reset(ctx, os.Args[2])
	case "logs":
		return client.GetLogs(ctx, os.Args[2])
	case "logs-watch":
		return client.WatchLogs(context.Background(), os.Args[2])
	default:
		return fmt.Errorf("invalid command: %s", os.Args[1])
	}
}
