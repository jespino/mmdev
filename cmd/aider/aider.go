package aider

import (
	"github.com/jespino/mmdev/cmd/aider/confluence"
	"github.com/jespino/mmdev/cmd/aider/github"
	"github.com/jespino/mmdev/cmd/aider/jira"
	"github.com/jespino/mmdev/cmd/aider/sentry"
	"github.com/spf13/cobra"
)

func AiderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aider",
		Short: "Process issues with aider",
		Long:  `Process GitHub or Jira issues with aider for automated fixes.`,
	}

	cmd.AddCommand(
		confluence.NewCommand(),
		github.NewCommand(),
		jira.NewCommand(),
		sentry.NewCommand(),
	)

	return cmd
}
