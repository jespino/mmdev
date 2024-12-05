package main

import (
	"fmt"
	"os"

	"github.com/jespino/mmdev/cmd/server"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "mmdev",
		Short: "MMDev - Development tool",
	}

	rootCmd.AddCommand(server.StartServerCmd())

	var startClientCmd = &cobra.Command{
		Use:   "start-client",
		Short: "Start the client",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Starting client...")
		},
	}

	rootCmd.AddCommand(startClientCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
