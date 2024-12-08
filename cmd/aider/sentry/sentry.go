package sentry

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sentry ISSUE-ID",
		Short: "Process Sentry issues with aider",
		Long:  `Downloads a Sentry issue and its events, then processes them with aider.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runSentry,
	}
	return cmd
}

func runSentry(cmd *cobra.Command, args []string) error {
	issueID := args[0]

	// Load configuration
	config, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %v", err)
	}

	// Initialize Sentry client
	options := sentry.ClientOptions{
		Dsn: os.Getenv("SENTRY_DSN"),
	}
	
	// Add auth token if configured
	if token := os.Getenv("SENTRY_TOKEN"); token != "" {
		options.AuthToken = token
	} else if config.Sentry.Token != "" {
		options.AuthToken = config.Sentry.Token
	}

	err = sentry.Init(options)
	if err != nil {
		return fmt.Errorf("sentry initialization failed: %v", err)
	}
	defer sentry.Flush(2)

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "sentry-issue-*.txt")
	if err != nil {
		return fmt.Errorf("error creating temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Get issue details using Sentry API
	client := sentry.CurrentHub().Client()
	if client == nil {
		return fmt.Errorf("failed to get Sentry client")
	}

	// Write issue content to file
	var content strings.Builder
	content.WriteString(fmt.Sprintf("Sentry Issue %s\n\n", issueID))

	// Get events for this issue
	events := client.GetEvents(context.Background(), issueID)
	for i, event := range events {
		content.WriteString(fmt.Sprintf("\n--- Event %d ---\n", i+1))
		content.WriteString(fmt.Sprintf("Message: %s\n", event.Message))
		if event.Exception != nil {
			for _, exc := range event.Exception {
				content.WriteString(fmt.Sprintf("Exception: %s\n", exc.Value))
				content.WriteString(fmt.Sprintf("Type: %s\n", exc.Type))
				if exc.Stacktrace != nil {
					content.WriteString("Stacktrace:\n")
					for _, frame := range exc.Stacktrace.Frames {
						content.WriteString(fmt.Sprintf("  %s:%d in %s\n", 
							frame.Filename,
							frame.Lineno,
							frame.Function))
					}
				}
			}
		}
	}

	if err := os.WriteFile(tmpFile.Name(), []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	// Run aider with the temporary file
	cmd2 := exec.Command("aider", "--read", tmpFile.Name())
	cmd2.Stdout = os.Stdout
	cmd2.Stderr = os.Stderr
	cmd2.Stdin = os.Stdin

	if err := cmd2.Run(); err != nil {
		return fmt.Errorf("error running aider: %v", err)
	}

	return nil
}
