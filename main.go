package main

import (
	"fmt"
	"os"

	"github.com/jespino/mmdev/cmd/client"
	"github.com/jespino/mmdev/cmd/docker"
	"github.com/jespino/mmdev/cmd/generate"
	"github.com/jespino/mmdev/cmd/server"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "mmdev",
		Short: "MMDev - Development tool",
	}

	rootCmd.AddCommand(
		server.ServerCmd(),
		client.ClientCmd(),
		docker.DockerCmd(),
		generate.GenerateCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
