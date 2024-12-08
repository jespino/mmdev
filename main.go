package main

import (
	"fmt"
	"os"

	"github.com/jespino/mmdev/cmd/aider"
	"github.com/jespino/mmdev/cmd/config"
	"github.com/jespino/mmdev/cmd/docker"
	"github.com/jespino/mmdev/cmd/e2e"
	"github.com/jespino/mmdev/cmd/generate"
	"github.com/jespino/mmdev/cmd/server"
	"github.com/jespino/mmdev/cmd/start"
	"github.com/jespino/mmdev/cmd/webapp"
	"github.com/jespino/mmdev/pkg/utils"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "mmdev",
		Short: "MMDev - Development tool",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip directory check for help command
			if cmd.Name() == "help" {
				return nil
			}

			// Find and validate Mattermost directory
			baseDir, err := utils.FindMattermostBaseDir()
			if err != nil {
				return err
			}

			// Change to Mattermost base directory
			if err := os.Chdir(baseDir); err != nil {
				return fmt.Errorf("failed to change to Mattermost directory: %w", err)
			}

			return nil
		},
	}

	rootCmd.AddCommand(
		server.ServerCmd(),
		webapp.WebappCmd(),
		docker.DockerCmd(),
		generate.GenerateCmd(),
		start.StartCmd(),
		e2e.E2ECmd(),
		aider.AiderCmd(),
		config.ConfigCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
