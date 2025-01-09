package githubpr

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "github-pr owner/repo#number",
		Short: "Process GitHub Pull Requests with aider",
		Long:  `Downloads a GitHub Pull Request, its comments and patch, then processes them with aider.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runGitHubPR,
	}
	return cmd
}

func runGitHubPR(cmd *cobra.Command, args []string) error {
	prURL := args[0]

	// Parse the GitHub PR URL
	parts := strings.Split(prURL, "#")
	if len(parts) != 2 {
		return fmt.Errorf("invalid PR URL format. Expected: owner/repo#number")
	}

	repoPath := strings.Split(parts[0], "/")
	if len(repoPath) != 2 {
		return fmt.Errorf("invalid repository format. Expected: owner/repo")
	}

	owner := repoPath[0]
	repo := repoPath[1]
	prNumber := parts[1]

	// Create GitHub client
	client := github.NewClient(nil)

	// Convert PR number to integer
	var prNum int
	fmt.Sscanf(prNumber, "%d", &prNum)

	// Get PR content
	pr, _, err := client.PullRequests.Get(context.Background(), owner, repo, prNum)
	if err != nil {
		return fmt.Errorf("error fetching PR: %v", err)
	}

	// Create temporary file for PR content
	tmpFile, err := os.CreateTemp("", "github-pr-*.txt")
	if err != nil {
		return fmt.Errorf("error creating temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Get PR comments
	comments, _, err := client.Issues.ListComments(context.Background(), owner, repo, prNum, nil)
	if err != nil {
		return fmt.Errorf("error fetching comments: %v", err)
	}

	// Write PR content and comments to file
	var content strings.Builder
	content.WriteString(fmt.Sprintf("Pull Request #%d: %s\n\n%s\n\n", prNum, *pr.Title, *pr.Body))

	if len(comments) > 0 {
		content.WriteString("Comments:\n")
		for i, comment := range comments {
			content.WriteString(fmt.Sprintf("\n--- Comment %d by @%s ---\n%s\n",
				i+1,
				*comment.User.Login,
				*comment.Body))
		}
	}

	// Get PR patch
	patch, _, err := client.PullRequests.GetRaw(
		context.Background(),
		owner,
		repo,
		prNum,
		github.RawOptions{Type: github.Patch},
	)
	if err != nil {
		return fmt.Errorf("error fetching PR patch: %v", err)
	}

	// Create temporary file for patch
	patchFile, err := os.CreateTemp("", "github-pr-*.patch")
	if err != nil {
		return fmt.Errorf("error creating patch file: %v", err)
	}
	defer os.Remove(patchFile.Name())

	if err := os.WriteFile(patchFile.Name(), []byte(patch), 0644); err != nil {
		return fmt.Errorf("error writing patch file: %v", err)
	}

	if err := os.WriteFile(tmpFile.Name(), []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	// Get current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current directory: %v", err)
	}

	// Run aider with the content and patch files
	aiderCmd := exec.Command("aider", "--read", tmpFile.Name(), patchFile.Name())
	aiderCmd.Dir = currentDir
	aiderCmd.Stdout = os.Stdout
	aiderCmd.Stderr = os.Stderr
	aiderCmd.Stdin = os.Stdin

	if err := aiderCmd.Run(); err != nil {
		return fmt.Errorf("error running aider: %v", err)
	}

	return nil
}
