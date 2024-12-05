package docker

import (
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
			manager := docker.NewManager("server")
			
			// Enable default services
			manager.EnableService(docker.Minio)
			manager.EnableService(docker.OpenLDAP)
			manager.EnableService(docker.Elasticsearch)
			
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
			manager := docker.NewManager("server")
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
			manager := docker.NewManager("server")
			return manager.Clean()
		},
	}
	return cmd
}