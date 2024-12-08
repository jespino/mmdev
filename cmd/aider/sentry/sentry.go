package sentry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/jespino/mmdev/internal/config"
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

	// Create HTTP client for Sentry API
	httpClient := &http.Client{}

	// Get auth token
	token := os.Getenv("SENTRY_TOKEN")
	if token == "" {
		token = config.Sentry.Token
	}

	if token == "" {
		return fmt.Errorf("Sentry token not configured. Set SENTRY_TOKEN env var or token in ~/.mmdev.toml")
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "sentry-issue-*.txt")
	if err != nil {
		return fmt.Errorf("error creating temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write issue content to file
	var content strings.Builder
	content.WriteString(fmt.Sprintf("Sentry Issue %s\n\n", issueID))

	// Get issue details from Sentry API
	req, err := http.NewRequest("GET", fmt.Sprintf("https://sentry.io/api/0/issues/%s/events/", issueID), nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error fetching events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Sentry API returned status %d", resp.StatusCode)
	}

	// Define custom event struct to match Sentry API response
	type Exception struct {
		Type       string `json:"type"`
		Value      string `json:"value"`
		Stacktrace struct {
			Frames []struct {
				Filename string `json:"filename"`
				Lineno   int    `json:"lineno"`
				Function string `json:"function"`
			} `json:"frames"`
		} `json:"stacktrace"`
	}

	type SentryEvent struct {
		EventID   string      `json:"eventId"`
		Message   string      `json:"message"`
		Tags      [][]string  `json:"tags"`
		Exception []Exception `json:"exception"`
	}

	// Parse events from response
	var events []SentryEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return fmt.Errorf("error decoding events: %v", err)
	}

	for i, event := range events {
		content.WriteString(fmt.Sprintf("\n--- Event %d ---\n", i+1))
		content.WriteString(fmt.Sprintf("Event ID: %s\n", event.EventID))
		content.WriteString(fmt.Sprintf("Message: %s\n", event.Message))

		if len(event.Tags) > 0 {
			content.WriteString("Tags:\n")
			for _, tag := range event.Tags {
				if len(tag) == 2 {
					content.WriteString(fmt.Sprintf("  %s: %s\n", tag[0], tag[1]))
				}
			}
		}

		if len(event.Exception) > 0 {
			for _, exc := range event.Exception {
				content.WriteString(fmt.Sprintf("Exception: %s\n", exc.Value))
				content.WriteString(fmt.Sprintf("Type: %s\n", exc.Type))
				if len(exc.Stacktrace.Frames) > 0 {
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
