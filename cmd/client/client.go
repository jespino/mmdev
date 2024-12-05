package client

import (
	"github.com/spf13/cobra"
)

func ClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client",
		Short: "Client related commands",
	}

	cmd.AddCommand(StartCmd())
	return cmd
}

func StartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the client",
		Run: func(cmd *cobra.Command, args []string) {
			// TODO: Implement client start
		},
	}
}
