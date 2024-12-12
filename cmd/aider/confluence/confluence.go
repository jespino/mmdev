package confluence

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

	// Use Jira credentials for Confluence
	url := os.Getenv("JIRA_URL")
	if url == "" {
		url = config.Jira.URL
	}
	username := os.Getenv("JIRA_USER")
	if username == "" {
		username = config.Jira.Username
	}
	token := os.Getenv("JIRA_TOKEN")
	if token == "" {
		token = config.Jira.Token
	}

	if url == "" {
		return fmt.Errorf("Jira URL not configured. Set JIRA_URL env var or url in ~/.mmdev.toml")
	}
	if username == "" {
		return fmt.Errorf("Jira username not configured. Set JIRA_USER env var or username in ~/.mmdev.toml")
	}
	if token == "" {
		return fmt.Errorf("Jira token not configured. Set JIRA_TOKEN env var or token in ~/.mmdev.toml")
	}

	// Create HTTP client
	client := &http.Client{}

	// Create temporary directory for the page and its resources
	tmpDir, err := os.MkdirTemp("", "confluence-page-*")
	if err != nil {
		return fmt.Errorf("error creating temporary directory: %v", err)
	}
	// Ensure cleanup of temp directory and all contents
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to cleanup temporary directory %s: %v\n", tmpDir, err)
		}
	}()

	// Create temporary file inside the directory
	tmpFile, err := os.Create(filepath.Join(tmpDir, "content.txt"))
	if err != nil {
		return fmt.Errorf("error creating temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Get page content
	pageReq, err := http.NewRequest("GET",
		fmt.Sprintf("%s/wiki/rest/api/content/%s?expand=body.storage,version,space", url, pageID),
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
		Status  string `json:"status"`
		Title   string `json:"title"`
		Version struct {
			Number int `json:"number"`
		} `json:"version"`
		SpaceId string `json:"spaceId"`
		Body    struct {
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
	content.WriteString(fmt.Sprintf("Space ID: %s\n", page.SpaceId))
	content.WriteString(fmt.Sprintf("Version: %d\n", page.Version.Number))
	content.WriteString(fmt.Sprintf("ID: %s\n", page.ID))
	content.WriteString(fmt.Sprintf("Status: %s\n\n", page.Status))
	content.WriteString("Content:\n")
	// Process content to download images and update references
	processedContent, err := downloadAndReplaceImages(client, url, username, token, tmpDir, page.ID)
	if err != nil {
		return fmt.Errorf("failed to process attachments: %v", err)
	}
	content.WriteString(page.Body.Storage.Value)
	content.WriteString(processedContent)
	content.WriteString("\n\n")

	// Get comments
	commentsReq, err := http.NewRequest("GET",
		fmt.Sprintf("%s/wiki/rest/api/content/%s/child/comment?expand=body.storage,version", url, pageID),
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
			Status  string `json:"status"`
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

	// Build command with individual --read flags
	args = []string{}

	// Add content file
	args = append(args, "--read", tmpFile.Name())

	// Add each image file with its own --read flag
	var imageFiles []string
	imageFiles, err = filepath.Glob(filepath.Join(tmpDir, "images", "*"))
	if err == nil {
		for _, imgFile := range imageFiles {
			args = append(args, "--read", imgFile)
		}
	}

	cmd2 := exec.Command("aider", args...)
	cmd2.Stdout = os.Stdout
	cmd2.Stderr = os.Stderr
	cmd2.Stdin = os.Stdin

	if err := cmd2.Run(); err != nil {
		return fmt.Errorf("error running aider: %v", err)
	}

	return nil
}

func downloadAndReplaceImages(client *http.Client, baseURL, username, token, tmpDir, pageID string) (string, error) {
	// Create images directory
	imagesDir := filepath.Join(tmpDir, "images")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create images directory: %v", err)
	}

	// Get attachments for the page
	attachmentsReq, err := http.NewRequest("GET",
		fmt.Sprintf("%s/wiki/api/v2/pages/%s/attachments", baseURL, pageID),
		nil)
	if err != nil {
		return "", fmt.Errorf("failed to create attachments request: %v", err)
	}
	attachmentsReq.SetBasicAuth(username, token)
	attachmentsReq.Header.Set("Accept", "application/json")

	resp, err := client.Do(attachmentsReq)
	if err != nil {
		return "", fmt.Errorf("failed to fetch attachments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("attachments API returned status %d", resp.StatusCode)
	}

	type Attachment struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		MediaType    string `json:"mediaType"`
		DownloadLink string `json:"downloadLink"`
	}

	type AttachmentsResponse struct {
		Results []Attachment `json:"results"`
	}

	var attachments AttachmentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&attachments); err != nil {
		return "", fmt.Errorf("failed to decode attachments response: %v", err)
	}

	var content strings.Builder
	content.WriteString("\nAttachments:\n")

	for _, attachment := range attachments.Results {
		// Only process image attachments
		if !strings.HasPrefix(attachment.MediaType, "image/") {
			continue
		}

		// Download image using v1 API
		downloadURL := fmt.Sprintf("%s/wiki/rest/api/content/%s/child/attachment/%s/download", baseURL, pageID, attachment.ID)
		downloadReq, err := http.NewRequest("GET", downloadURL, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to create request for image %s: %v\n", attachment.Title, err)
			continue
		}
		downloadReq.SetBasicAuth(username, token)
		downloadReq.Header.Set("X-Atlassian-Token", "no-check")

		downloadResp, err := client.Do(downloadReq)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to download image %s: %v\n", attachment.Title, err)
			continue
		}
		defer downloadResp.Body.Close()

		if downloadResp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "Warning: Failed to download image %s: status %d\n", attachment.Title, downloadResp.StatusCode)
			continue
		}

		// Save image
		localPath := filepath.Join("images", attachment.Title)
		fullPath := filepath.Join(imagesDir, attachment.Title)

		out, err := os.Create(fullPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to create image file %s: %v\n", fullPath, err)
			continue
		}

		if _, err := io.Copy(out, downloadResp.Body); err != nil {
			out.Close()
			fmt.Fprintf(os.Stderr, "Warning: Failed to save image %s: %v\n", fullPath, err)
			continue
		}
		out.Close()

		content.WriteString(fmt.Sprintf("[Image: %s]\n", localPath))
	}

	return content.String(), nil
}
