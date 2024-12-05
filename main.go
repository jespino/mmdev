package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "mmdev",
		Short: "MMDev - Development tool",
	}

	var startServerCmd = &cobra.Command{
		Use:   "start-server",
		Short: "Start the server",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Starting server...")
		},
	}

	var startClientCmd = &cobra.Command{
		Use:   "start-client",
		Short: "Start the client",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Starting client...")
		},
	}

	rootCmd.AddCommand(startServerCmd)
	rootCmd.AddCommand(startClientCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
