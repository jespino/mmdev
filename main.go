package main

import (
	"fmt"
	"os"

	"github.com/jespino/mmdev/cmd/client"
	"github.com/jespino/mmdev/cmd/server"
	"github.com/jespino/mmdev/cmd/start"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "mmdev",
		Short: "MMDev - Development tool",
	}

	rootCmd.AddCommand(server.ServerCmd())
	rootCmd.AddCommand(client.ClientCmd())
	rootCmd.AddCommand(start.StartCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
