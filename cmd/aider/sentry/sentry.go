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

	// Get issue details
	issueReq, err := http.NewRequest("GET", fmt.Sprintf("https://sentry.io/api/0/issues/%s/", issueID), nil)
	if err != nil {
		return fmt.Errorf("error creating issue request: %v", err)
	}
	issueReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	issueResp, err := httpClient.Do(issueReq)
	if err != nil {
		return fmt.Errorf("error fetching issue: %v", err)
	}
	defer issueResp.Body.Close()

	if issueResp.StatusCode != http.StatusOK {
		return fmt.Errorf("Sentry API returned status %d for issue request", issueResp.StatusCode)
	}

	type SentryIssue struct {
		Title       string `json:"title"`
		Culprit     string `json:"culprit"`
		FirstSeen   string `json:"firstSeen"`
		LastSeen    string `json:"lastSeen"`
		Count       int    `json:"count"`
		UserCount   int    `json:"userCount"`
		Platform    string `json:"platform"`
		Status      string `json:"status"`
	}

	var issue SentryIssue
	if err := json.NewDecoder(issueResp.Body).Decode(&issue); err != nil {
		return fmt.Errorf("error decoding issue: %v", err)
	}

	// Write issue information
	content.WriteString(fmt.Sprintf("Title: %s\n", issue.Title))
	content.WriteString(fmt.Sprintf("Culprit: %s\n", issue.Culprit))
	content.WriteString(fmt.Sprintf("First Seen: %s\n", issue.FirstSeen))
	content.WriteString(fmt.Sprintf("Last Seen: %s\n", issue.LastSeen))
	content.WriteString(fmt.Sprintf("Event Count: %d\n", issue.Count))
	content.WriteString(fmt.Sprintf("User Count: %d\n", issue.UserCount))
	content.WriteString(fmt.Sprintf("Platform: %s\n", issue.Platform))
	content.WriteString(fmt.Sprintf("Status: %s\n\n", issue.Status))

	// Get events
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

	type Tag struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	type SentryEvent struct {
		EventID   string      `json:"eventId"`
		Message   string      `json:"message"`
		Tags      []Tag       `json:"tags"`
		Exception []Exception `json:"exception"`
	}

	// Parse events from response (limit to 10)
	var events []SentryEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return fmt.Errorf("error decoding events: %v", err)
	}

	// Limit to 10 most recent events
	if len(events) > 10 {
		events = events[:10]
	}

	content.WriteString(fmt.Sprintf("Latest %d Events:\n", len(events)))
	for i, event := range events {
		content.WriteString(fmt.Sprintf("\n--- Event %d ---\n", i+1))
		content.WriteString(fmt.Sprintf("Event ID: %s\n", event.EventID))
		content.WriteString(fmt.Sprintf("Message: %s\n", event.Message))

		if len(event.Tags) > 0 {
			content.WriteString("Tags:\n")
			for _, tag := range event.Tags {
				content.WriteString(fmt.Sprintf("  %s: %s\n", tag.Key, tag.Value))
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
