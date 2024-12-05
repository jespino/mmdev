package docker

import (
	"fmt"
	"github.com/jespino/mmdev/pkg/docker"
	"github.com/spf13/cobra"
)

func DockerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker",
		Short: "Docker related commands",
	}

	cmd.AddCommand(
		StartCmd(),
		StopCmd(),
		CleanCmd(),
	)
	return cmd
}

func StartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start docker services",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := docker.NewManager()
			if err != nil {
				return fmt.Errorf("failed to create docker manager: %w", err)
			}
			
			// Enable default services
			manager.EnableService(docker.Minio)
			manager.EnableService(docker.OpenLDAP)
			manager.EnableService(docker.Elasticsearch)
			manager.EnableService(docker.Postgres)
			manager.EnableService(docker.Inbucket)
			manager.EnableService(docker.Redis)
			
			return manager.Start()
		},
	}
	return cmd
}

func StopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop docker services",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := docker.NewManager()
			if err != nil {
				return fmt.Errorf("failed to create docker manager: %w", err)
			}
			return manager.Stop()
		},
	}
	return cmd
}

func CleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove docker containers and volumes",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := docker.NewManager()
			if err != nil {
				return fmt.Errorf("failed to create docker manager: %w", err)
			}
			return manager.Clean()
		},
	}
	return cmd
}
