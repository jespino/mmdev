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
		Title         string                 `json:"title"`
		Culprit       string                 `json:"culprit"`
		FirstSeen     string                 `json:"firstSeen"`
		LastSeen      string                 `json:"lastSeen"`
		Count         string                 `json:"count"`
		UserCount     int                    `json:"userCount"`
		Level         string                 `json:"level"`
		Status        string                 `json:"status"`
		StatusDetails map[string]interface{} `json:"statusDetails"`
		IsPublic      bool                   `json:"isPublic"`
		Platform      string                 `json:"platform"`
		Project       struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Slug string `json:"slug"`
		} `json:"project"`
		Type     string `json:"type"`
		Metadata struct {
			Title string `json:"title"`
		} `json:"metadata"`
		NumComments     int         `json:"numComments"`
		AssignedTo      interface{} `json:"assignedTo"`
		IsSubscribed    bool        `json:"isSubscribed"`
		HasSeen         bool        `json:"hasSeen"`
		IsBookmarked    bool        `json:"isBookmarked"`
		ShareID         interface{} `json:"shareId"`
		ShortID         string      `json:"shortId"`
		Permalink       string      `json:"permalink"`
		UserReportCount int         `json:"userReportCount"`
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Sentry API returned status %d", resp.StatusCode)
	}

	type Frame struct {
		Filename string  `json:"filename"`
		Lineno   int     `json:"lineno"`
		Function string  `json:"function"`
		Context  [][]any `json:"context"`
	}

	// Define custom event struct to match Sentry API response
	type Exception struct {
		Type       string `json:"type"`
		Value      string `json:"value"`
		Stacktrace struct {
			Frames []Frame `json:"frames"`
		} `json:"stacktrace"`
	}

	type Tag struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	type SentryEvent struct {
		EventID      string                   `json:"eventId"`
		Message      string                   `json:"message"`
		Title        string                   `json:"title"`
		Type         string                   `json:"type"`
		Platform     string                   `json:"platform"`
		DateCreated  string                   `json:"dateCreated"`
		DateReceived string                   `json:"dateReceived"`
		Tags         []Tag                    `json:"tags"`
		Exception    []Exception              `json:"exception"`
		Entries      []map[string]interface{} `json:"entries"`
		Packages     map[string]string        `json:"packages"`
		Sdk          map[string]string        `json:"sdk"`
		Contexts     map[string]interface{}   `json:"contexts"`
		Fingerprints []string                 `json:"fingerprints"`
		Context      map[string]interface{}   `json:"context"`
		Release      map[string]interface{}   `json:"release"`
		User         map[string]interface{}   `json:"user"`
		Location     string                   `json:"location"`
		Culprit      string                   `json:"culprit"`
	}

	// Parse event list from response
	var eventList []struct {
		EventID string `json:"eventId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&eventList); err != nil {
		return fmt.Errorf("error decoding event list: %v", err)
	}

	// Limit to 3 most recent events
	if len(eventList) > 3 {
		eventList = eventList[:3]
	}

	content.WriteString(fmt.Sprintf("Latest %d Events:\n", len(eventList)))

	// Fetch full details for each event
	for i, eventSummary := range eventList {
		// Get detailed event data
		eventReq, err := http.NewRequest("GET", fmt.Sprintf("https://sentry.io/api/0/issues/%s/events/%s/", issueID, eventSummary.EventID), nil)
		if err != nil {
			return fmt.Errorf("error creating event request: %v", err)
		}
		eventReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

		eventResp, err := httpClient.Do(eventReq)
		if err != nil {
			return fmt.Errorf("error fetching event details: %v", err)
		}
		defer eventResp.Body.Close()

		if eventResp.StatusCode != http.StatusOK {
			return fmt.Errorf("Sentry API returned status %d for event request", eventResp.StatusCode)
		}

		var event SentryEvent
		if err := json.NewDecoder(eventResp.Body).Decode(&event); err != nil {
			return fmt.Errorf("error decoding event details: %v", err)
		}

		content.WriteString(fmt.Sprintf("\n--- Event %d ---\n", i+1))
		content.WriteString(fmt.Sprintf("Event ID: %s\n", eventSummary.EventID))
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

		// Process exceptions from both direct exception field and entries
		processExceptions := func(exceptions []Exception) {
			for _, exc := range exceptions {
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
						if frame.Context != nil && len(frame.Context) > 0 {
							content.WriteString("    Context:\n")
							for _, ctx := range frame.Context {
								if len(ctx) == 2 {
									content.WriteString(fmt.Sprintf("      %d: %s\n", ctx[0], ctx[1]))
								}
							}
						}
					}
				}
			}
		}

		if len(event.Exception) > 0 {
			content.WriteString("\nDirect Exceptions:\n")
			processExceptions(event.Exception)
		}

		// Process entries that contain exceptions
		for _, entry := range event.Entries {
			if entry["type"] == "exception" {
				if data, ok := entry["data"].(map[string]interface{}); ok {
					if values, ok := data["values"].([]interface{}); ok {
						content.WriteString("\nException Entries:\n")
						for _, v := range values {
							if excMap, ok := v.(map[string]interface{}); ok {
								exc := Exception{
									Type:  excMap["type"].(string),
									Value: excMap["value"].(string),
								}

								if stacktrace, ok := excMap["stacktrace"].(map[string]interface{}); ok {
									if frames, ok := stacktrace["frames"].([]interface{}); ok {
										for _, f := range frames {
											if frame, ok := f.(map[string]interface{}); ok {
												exc.Stacktrace.Frames = append(exc.Stacktrace.Frames, Frame{
													Filename: toString(frame["filename"]),
													Lineno:   toInt(frame["lineNo"]),
													Function: toString(frame["function"]),
													Context:  toContext(frame["context"]),
												})
											}
										}
									}
								}
								processExceptions([]Exception{exc})
							}
						}
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

// Helper functions to safely handle nil values
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func toInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case float32:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case int32:
		return int(n)
	}
	return 0
}

func toContext(v interface{}) [][]any {
	if v == nil {
		return nil
	}
	if ctx, ok := v.([][]any); ok {
		return ctx
	}
	return nil
}
