package confluence

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"encoding/json"

	"github.com/jespino/mmdev/internal/config"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "confluence PAGE-ID",
		Short: "Process Confluence pages with aider",
		Long:  `Downloads a Confluence page and its comments, then processes them with aider.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runConfluence,
	}
	return cmd
}

func runConfluence(cmd *cobra.Command, args []string) error {
	pageID := args[0]

	// Load configuration
	config, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %v", err)
	}

	// Get auth credentials
	url := os.Getenv("CONFLUENCE_URL")
	if url == "" {
		url = config.Confluence.URL
	}
	username := os.Getenv("CONFLUENCE_USER")
	if username == "" {
		username = config.Confluence.Username
	}
	token := os.Getenv("CONFLUENCE_TOKEN")
	if token == "" {
		token = config.Confluence.Token
	}

	if url == "" {
		return fmt.Errorf("Confluence URL not configured. Set CONFLUENCE_URL env var or url in ~/.mmdev.toml")
	}
	if username == "" {
		return fmt.Errorf("Confluence username not configured. Set CONFLUENCE_USER env var or username in ~/.mmdev.toml")
	}
	if token == "" {
		return fmt.Errorf("Confluence token not configured. Set CONFLUENCE_TOKEN env var or token in ~/.mmdev.toml")
	}

	// Create HTTP client
	client := &http.Client{}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "confluence-page-*.txt")
	if err != nil {
		return fmt.Errorf("error creating temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Get page content
	pageReq, err := http.NewRequest("GET", 
		fmt.Sprintf("%s/rest/api/content/%s?expand=body.storage,version,space,metadata.labels,metadata.properties", url, pageID),
		nil)
	if err != nil {
		return fmt.Errorf("error creating page request: %v", err)
	}
	pageReq.SetBasicAuth(username, token)

	pageResp, err := client.Do(pageReq)
	if err != nil {
		return fmt.Errorf("error fetching page: %v", err)
	}
	defer pageResp.Body.Close()

	if pageResp.StatusCode != http.StatusOK {
		return fmt.Errorf("Confluence API returned status %d", pageResp.StatusCode)
	}

	type Page struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Status  string `json:"status"`
		Title   string `json:"title"`
		Version struct {
			Number int `json:"number"`
		} `json:"version"`
		Space struct {
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"space"`
		Body struct {
			Storage struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
	}

	var page Page
	if err := json.NewDecoder(pageResp.Body).Decode(&page); err != nil {
		return fmt.Errorf("error decoding page: %v", err)
	}

	// Write page content to file
	var content strings.Builder
	content.WriteString(fmt.Sprintf("Confluence Page: %s\n", page.Title))
	content.WriteString(fmt.Sprintf("Space: %s (%s)\n", page.Space.Name, page.Space.Key))
	content.WriteString(fmt.Sprintf("Version: %d\n", page.Version.Number))
	content.WriteString(fmt.Sprintf("ID: %s\n", page.ID))
	content.WriteString(fmt.Sprintf("Type: %s\n", page.Type))
	content.WriteString(fmt.Sprintf("Status: %s\n\n", page.Status))
	content.WriteString("Content:\n")
	content.WriteString(page.Body.Storage.Value)
	content.WriteString("\n\n")

	// Get comments
	commentsReq, err := http.NewRequest("GET",
		fmt.Sprintf("%s/rest/api/content/%s/child/comment?expand=body.storage,version", url, pageID),
		nil)
	if err != nil {
		return fmt.Errorf("error creating comments request: %v", err)
	}
	commentsReq.SetBasicAuth(username, token)

	commentsResp, err := client.Do(commentsReq)
	if err != nil {
		return fmt.Errorf("error fetching comments: %v", err)
	}
	defer commentsResp.Body.Close()

	if commentsResp.StatusCode != http.StatusOK {
		return fmt.Errorf("Confluence API returned status %d for comments request", commentsResp.StatusCode)
	}

	type CommentsResponse struct {
		Results []struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Version struct {
				Number int `json:"number"`
			} `json:"version"`
			Body struct {
				Storage struct {
					Value string `json:"value"`
				} `json:"storage"`
			} `json:"body"`
		} `json:"results"`
	}

	var comments CommentsResponse
	if err := json.NewDecoder(commentsResp.Body).Decode(&comments); err != nil {
		return fmt.Errorf("error decoding comments: %v", err)
	}

	if len(comments.Results) > 0 {
		content.WriteString("Comments:\n")
		for i, comment := range comments.Results {
			content.WriteString(fmt.Sprintf("\n--- Comment %d ---\n", i+1))
			content.WriteString(fmt.Sprintf("ID: %s\n", comment.ID))
			content.WriteString(fmt.Sprintf("Version: %d\n", comment.Version.Number))
			content.WriteString(fmt.Sprintf("Content:\n%s\n", comment.Body.Storage.Value))
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
