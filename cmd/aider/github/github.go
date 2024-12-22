package github

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/jespino/mmdev/cmd/aider/indexcommits"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "github owner/repo#number",
		Short: "Process GitHub issues with aider",
		Long:  `Downloads a GitHub issue and its comments, then processes them with aider.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runGitHub,
	}
	return cmd
}

func runGitHub(cmd *cobra.Command, args []string) error {
	issueURL := args[0]

	// Parse the GitHub issue URL
	parts := strings.Split(issueURL, "#")
	if len(parts) != 2 {
		return fmt.Errorf("invalid issue URL format. Expected: owner/repo#number")
	}

	repoPath := strings.Split(parts[0], "/")
	if len(repoPath) != 2 {
		return fmt.Errorf("invalid repository format. Expected: owner/repo")
	}

	owner := repoPath[0]
	repo := repoPath[1]
	issueNumber := parts[1]

	// Create GitHub client
	client := github.NewClient(nil)

	// Convert issue number to integer
	var issueNum int
	fmt.Sscanf(issueNumber, "%d", &issueNum)

	// Get issue content
	issue, _, err := client.Issues.Get(context.Background(), owner, repo, issueNum)
	if err != nil {
		return fmt.Errorf("error fetching issue: %v", err)
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "github-issue-*.txt")
	if err != nil {
		return fmt.Errorf("error creating temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Get issue comments
	comments, _, err := client.Issues.ListComments(context.Background(), owner, repo, issueNum, nil)
	if err != nil {
		return fmt.Errorf("error fetching comments: %v", err)
	}

	// Write issue content and comments to file
	var content strings.Builder
	content.WriteString(fmt.Sprintf("Issue #%d: %s\n\n%s\n\n", issueNum, *issue.Title, *issue.Body))
	
	if len(comments) > 0 {
		content.WriteString("Comments:\n")
		for i, comment := range comments {
			content.WriteString(fmt.Sprintf("\n--- Comment %d by @%s ---\n%s\n", 
				i+1, 
				*comment.User.Login,
				*comment.Body))
		}
	}

	if err := os.WriteFile(tmpFile.Name(), []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	// Run aider with explicit --read flag
	cmd2 := exec.Command("aider", "--read", tmpFile.Name())
	cmd2.Stdout = os.Stdout
	cmd2.Stderr = os.Stderr
	cmd2.Stdin = os.Stdin

	if err := cmd2.Run(); err != nil {
		return fmt.Errorf("error running aider: %v", err)
	}

	return nil
}
