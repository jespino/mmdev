package confluence

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"encoding/json"
	"io"
	"path/filepath"
	"regexp"

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
	processedContent := downloadAndReplaceImages(client, url, username, token, tmpDir, page.Body.Storage.Value)
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
			processedComment := downloadAndReplaceImages(client, url, username, token, tmpDir, comment.Body.Storage.Value)
			content.WriteString(fmt.Sprintf("Content:\n%s\n", processedComment))
		}
	}

	if err := os.WriteFile(tmpFile.Name(), []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	// Build command with individual --read flags
	args := []string{}
	
	// Add content file
	args = append(args, "--read", tmpFile.Name())
	
	// Add each image file with its own --read flag
	imageFiles, _ := filepath.Glob(filepath.Join(tmpDir, "images", "*"))
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

func downloadAndReplaceImages(client *http.Client, baseURL, username, token, tmpDir, content string) string {
	// Regular expression to find image tags with ac:image-uri
	re := regexp.MustCompile(`<ac:image[^>]*><ri:url[^>]*>([^<]+)</ri:url></ac:image>`)
	
	// Create images directory
	imagesDir := filepath.Join(tmpDir, "images")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to create images directory: %v\n", err)
		return content
	}

	processedContent := re.ReplaceAllStringFunc(content, func(match string) string {
		// Extract image URL
		urlMatch := re.FindStringSubmatch(match)
		if len(urlMatch) < 2 {
			return match
		}
		imageURL := urlMatch[1]

		// Download image
		req, err := http.NewRequest("GET", imageURL, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to create request for image %s: %v\n", imageURL, err)
			return match
		}
		req.SetBasicAuth(username, token)

		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to download image %s: %v\n", imageURL, err)
			return match
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "Warning: Failed to download image %s: status %d\n", imageURL, resp.StatusCode)
			return match
		}

		// Generate a filename based on the last part of the URL
		filename := filepath.Base(imageURL)
		localPath := filepath.Join("images", filename)
		fullPath := filepath.Join(imagesDir, filename)

		// Save the image
		out, err := os.Create(fullPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to create image file %s: %v\n", fullPath, err)
			return match
		}
		defer out.Close()

		if _, err := io.Copy(out, resp.Body); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to save image %s: %v\n", fullPath, err)
			return match
		}

		// Replace the Confluence image tag with a reference to the local file
		return fmt.Sprintf("\n[Image: %s]\n", localPath)
	})

	return processedContent
}
