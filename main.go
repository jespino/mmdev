package main

import (
	"fmt"
	"os"

	"github.com/jespino/mmdev/cmd/client"
	"github.com/jespino/mmdev/cmd/docker"
	"github.com/jespino/mmdev/cmd/generate"
	"github.com/jespino/mmdev/cmd/server" 
	"github.com/jespino/mmdev/cmd/start"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "mmdev",
		Short: "MMDev - Development tool",
	}

	rootCmd.AddCommand(
		server.ServerCmd(),
		client.ClientCmd(),
		start.StartCmd(),
		docker.DockerCmd(),
		generate.GenerateCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
