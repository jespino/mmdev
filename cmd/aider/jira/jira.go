package jira

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jespino/mmdev/internal/config"

	jira "github.com/andygrunwald/go-jira"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jira PROJECT-123",
		Short: "Process Jira issues with aider",
		Long:  `Downloads a Jira issue and its comments, then processes them with aider.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runJira,
	}
	return cmd
}

func runJira(cmd *cobra.Command, args []string) error {
	issueKey := args[0]

	// Load configuration
	config, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %v", err)
	}

	if config.Jira.URL == "" {
		return fmt.Errorf("Jira URL not configured. Set it in ~/.mmdev.toml or JIRA_URL environment variable")
	}
	if config.Jira.Username == "" {
		return fmt.Errorf("Jira username not configured. Set it in ~/.mmdev.toml or JIRA_USER environment variable")
	}
	if config.Jira.Token == "" {
		return fmt.Errorf("Jira token not configured. Set it in ~/.mmdev.toml or JIRA_TOKEN environment variable")
	}

	// Create Jira client
	tp := jira.BasicAuthTransport{
		Username: config.Jira.Username,
		Password: config.Jira.Token,
	}
	client, err := jira.NewClient(tp.Client(), config.Jira.URL)
	if err != nil {
		return fmt.Errorf("error creating Jira client: %v", err)
	}

	// Get issue content
	issue, _, err := client.Issue.Get(issueKey, nil)
	if err != nil {
		return fmt.Errorf("error fetching issue: %v", err)
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "jira-issue-*.txt")
	if err != nil {
		return fmt.Errorf("error creating temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write issue content to file
	var content strings.Builder
	content.WriteString(fmt.Sprintf("Issue %s: %s\n\n%s\n\n",
		issue.Key,
		issue.Fields.Summary,
		issue.Fields.Description))

	// Get issue comments
	if issue.Fields.Comments != nil {
		content.WriteString("Comments:\n")
		for i, comment := range issue.Fields.Comments.Comments {
			content.WriteString(fmt.Sprintf("\n--- Comment %d by @%s ---\n%s\n",
				i+1,
				comment.Author.DisplayName,
				comment.Body))
		}
	}

	if err := os.WriteFile(tmpFile.Name(), []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	// Run aider with explicit --read flag
	cmd2 = exec.Command("aider", "--read", tmpFile.Name())
	cmd2.Stdout = os.Stdout
	cmd2.Stderr = os.Stderr
	cmd2.Stdin = os.Stdin

	if err := cmd2.Run(); err != nil {
		return fmt.Errorf("error running aider: %v", err)
	}

	return nil
}
