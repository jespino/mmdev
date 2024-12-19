package start

import (
	"github.com/spf13/cobra"
)

func StartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the development environment",
		Annotations: map[string]string{
			"requiresMMRepo": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return StartTUI()
		},
	}
	return cmd
}
