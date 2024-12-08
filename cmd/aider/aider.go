package aider

import (
	"github.com/jespino/mmdev/cmd/github"
	"github.com/jespino/mmdev/cmd/jira"
	"github.com/spf13/cobra"
)

func AiderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aider",
		Short: "Process issues with aider",
		Long:  `Process GitHub or Jira issues with aider for automated fixes.`,
	}

	cmd.AddCommand(
		github.NewCommand(),
		jira.NewCommand(),
	)

	return cmd
}
