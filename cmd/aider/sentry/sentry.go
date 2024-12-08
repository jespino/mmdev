package sentry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

	// Print raw response for debugging
	issueBody, err := io.ReadAll(issueResp.Body)
	if err != nil {
		return fmt.Errorf("error reading issue response body: %v", err)
	}
	fmt.Printf("Raw Issue Response:\n%s\n\n", string(issueBody))
	
	// Create new reader from the response body for JSON decoding
	issueResp.Body = io.NopCloser(bytes.NewBuffer(issueBody))

	if issueResp.StatusCode != http.StatusOK {
		return fmt.Errorf("Sentry API returned status %d for issue request with body: %s", issueResp.StatusCode, string(issueBody))
	}

	type SentryIssue struct {
		Title           string `json:"title"`
		Culprit         string `json:"culprit"`
		FirstSeen       string `json:"firstSeen"`
		LastSeen        string `json:"lastSeen"`
		Count           string `json:"count"`
		UserCount       int    `json:"userCount"`
		Level           string `json:"level"`
		Status          string `json:"status"`
		StatusDetails   map[string]interface{} `json:"statusDetails"`
		IsPublic        bool   `json:"isPublic"`
		Platform        string `json:"platform"`
		Project         struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Slug string `json:"slug"`
		} `json:"project"`
		Type        string `json:"type"`
		Metadata    struct {
			Title string `json:"title"`
		} `json:"metadata"`
		NumComments     int    `json:"numComments"`
		AssignedTo      interface{} `json:"assignedTo"`
		IsSubscribed    bool   `json:"isSubscribed"`
		HasSeen         bool   `json:"hasSeen"`
		IsBookmarked    bool   `json:"isBookmarked"`
		ShareID         interface{} `json:"shareId"`
		ShortID         string `json:"shortId"`
		Permalink       string `json:"permalink"`
		UserReportCount int    `json:"userReportCount"`
	}

	var issue SentryIssue
	if err := json.NewDecoder(issueResp.Body).Decode(&issue); err != nil {
		return fmt.Errorf("error decoding issue: %v", err)
	}

	// Write issue information
	content.WriteString(fmt.Sprintf("Title: %s\n", issue.Metadata.Title))
	content.WriteString(fmt.Sprintf("Project: %s (%s)\n", issue.Project.Name, issue.Project.Slug))
	content.WriteString(fmt.Sprintf("Type: %s\n", issue.Type))
	content.WriteString(fmt.Sprintf("Level: %s\n", issue.Level))
	content.WriteString(fmt.Sprintf("Culprit: %s\n", issue.Culprit))
	content.WriteString(fmt.Sprintf("First Seen: %s\n", issue.FirstSeen))
	content.WriteString(fmt.Sprintf("Last Seen: %s\n", issue.LastSeen))
	content.WriteString(fmt.Sprintf("Event Count: %s\n", issue.Count))
	content.WriteString(fmt.Sprintf("User Count: %d\n", issue.UserCount))
	content.WriteString(fmt.Sprintf("User Reports: %d\n", issue.UserReportCount))
	content.WriteString(fmt.Sprintf("Comments: %d\n", issue.NumComments))
	content.WriteString(fmt.Sprintf("Status: %s\n", issue.Status))
	content.WriteString(fmt.Sprintf("Platform: %s\n", issue.Platform))
	content.WriteString(fmt.Sprintf("Permalink: %s\n", issue.Permalink))
	content.WriteString(fmt.Sprintf("Short ID: %s\n\n", issue.ShortID))

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

	// Print raw response for debugging
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}
	fmt.Printf("Raw Events Response:\n%s\n\n", string(respBody))
	
	// Create new reader from the response body for JSON decoding
	resp.Body = io.NopCloser(bytes.NewBuffer(respBody))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Sentry API returned status %d with body: %s", resp.StatusCode, string(respBody))
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
		EventID        string                 `json:"eventId"`
		Message        string                 `json:"message"`
		Title          string                 `json:"title"`
		Type           string                 `json:"type"`
		Platform       string                 `json:"platform"`
		DateCreated    string                 `json:"dateCreated"`
		DateReceived   string                 `json:"dateReceived"`
		Tags           []Tag                  `json:"tags"`
		Exception      []Exception            `json:"exception"`
		Entries        []map[string]interface{} `json:"entries"`
		Packages       map[string]string      `json:"packages"`
		Sdk           map[string]string      `json:"sdk"`
		Contexts      map[string]interface{} `json:"contexts"`
		Fingerprints  []string              `json:"fingerprints"`
		Context       map[string]interface{} `json:"context"`
		Release       map[string]interface{} `json:"release"`
		User          map[string]interface{} `json:"user"`
		Location      string                 `json:"location"`
		Culprit       string                 `json:"culprit"`
	}

	// Parse events from response (limit to 3)
	var events []SentryEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return fmt.Errorf("error decoding events: %v", err)
	}

	// Limit to 3 most recent events
	if len(events) > 3 {
		events = events[:3]
	}

	content.WriteString(fmt.Sprintf("Latest %d Events:\n", len(events)))
	for i, event := range events {
		content.WriteString(fmt.Sprintf("\n--- Event %d ---\n", i+1))
		content.WriteString(fmt.Sprintf("Event ID: %s\n", event.EventID))
		content.WriteString(fmt.Sprintf("Title: %s\n", event.Title))
		content.WriteString(fmt.Sprintf("Type: %s\n", event.Type))
		content.WriteString(fmt.Sprintf("Platform: %s\n", event.Platform))
		content.WriteString(fmt.Sprintf("Created: %s\n", event.DateCreated))
		content.WriteString(fmt.Sprintf("Received: %s\n", event.DateReceived))
		content.WriteString(fmt.Sprintf("Location: %s\n", event.Location))
		content.WriteString(fmt.Sprintf("Culprit: %s\n", event.Culprit))

		if event.User != nil {
			content.WriteString("\nUser:\n")
			userBytes, _ := json.MarshalIndent(event.User, "  ", "  ")
			content.WriteString(fmt.Sprintf("  %s\n", string(userBytes)))
		}

		if len(event.Tags) > 0 {
			content.WriteString("\nTags:\n")
			for _, tag := range event.Tags {
				content.WriteString(fmt.Sprintf("  %s: %s\n", tag.Key, tag.Value))
			}
		}

		if event.Sdk != nil {
			content.WriteString("\nSDK:\n")
			for k, v := range event.Sdk {
				content.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
			}
		}

		if len(event.Exception) > 0 {
			content.WriteString("\nExceptions:\n")
			for _, exc := range event.Exception {
				content.WriteString(fmt.Sprintf("\nException: %s\n", exc.Value))
				content.WriteString(fmt.Sprintf("Type: %s\n", exc.Type))
				if len(exc.Stacktrace.Frames) > 0 {
					content.WriteString("Stacktrace:\n")
					// Print frames in reverse order for better readability
					for i := len(exc.Stacktrace.Frames) - 1; i >= 0; i-- {
						frame := exc.Stacktrace.Frames[i]
						content.WriteString(fmt.Sprintf("  File \"%s\", line %d, in %s\n", 
							frame.Filename,
							frame.Lineno,
							frame.Function))
					}
				}
			}
		}

		if event.Release != nil {
			content.WriteString("\nRelease Info:\n")
			releaseBytes, _ := json.MarshalIndent(event.Release, "  ", "  ")
			content.WriteString(fmt.Sprintf("  %s\n", string(releaseBytes)))
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
