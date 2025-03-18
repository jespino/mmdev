package pluginctl

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// CreateNewPlugin creates a new plugin based on the starter template
func CreateNewPlugin(ctx context.Context, pluginName string) error {
	// Validate plugin name - should be valid directory name and valid go package name
	if !isValidPluginName(pluginName) {
		return fmt.Errorf("invalid plugin name: %s - use only lowercase letters, numbers, and hyphens", pluginName)
	}

	// Get plugin description from user
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter plugin description: ")
	description, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read description: %w", err)
	}
	description = strings.TrimSpace(description)

	// Create directory name
	dirName := fmt.Sprintf("mattermost-plugin-%s", pluginName)

	// Check if directory already exists
	if _, err := os.Stat(dirName); err == nil {
		return fmt.Errorf("directory %s already exists", dirName)
	}

	// Clone the starter template
	fmt.Printf("Cloning starter template to %s...\n", dirName)
	cmd := exec.CommandContext(ctx, "git", "clone", "https://github.com/mattermost/mattermost-plugin-starter-template", dirName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone template repository: %w", err)
	}

	// Remove .git directory
	if err := os.RemoveAll(filepath.Join(dirName, ".git")); err != nil {
		return fmt.Errorf("failed to remove .git directory: %w", err)
	}

	// Walk through all files and replace "starter-template" with the new plugin name
	fmt.Println("Customizing plugin files...")
	if err := filepath.Walk(dirName, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip binary files or files that are typically not text
		if isBinaryFile(path) {
			return nil
		}

		// Read file
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		// Replace occurrences of "starter-template" with the new plugin name
		newContent := strings.ReplaceAll(string(content), "starter-template", pluginName)

		// If this is plugin.json, also update the description
		if filepath.Base(path) == "plugin.json" {
			re := regexp.MustCompile(`"description": "(.*?)"`)
			newContent = re.ReplaceAllString(newContent, fmt.Sprintf(`"description": "%s"`, description))
		}

		// Write back to file if content changed
		if newContent != string(content) {
			if err := os.WriteFile(path, []byte(newContent), info.Mode()); err != nil {
				return fmt.Errorf("failed to write file %s: %w", path, err)
			}
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to process plugin files: %w", err)
	}

	fmt.Printf("Plugin created successfully in %s\n", dirName)
	fmt.Println("To start developing your plugin:")
	fmt.Printf("  cd %s\n", dirName)
	fmt.Println("  make")

	return nil
}

// isValidPluginName checks if the plugin name is valid
// Plugin names should be valid directory names and valid go package names
func isValidPluginName(name string) bool {
	matched, _ := regexp.MatchString(`^[a-z][a-z0-9-]*$`, name)
	return matched
}

// isBinaryFile returns true if the file is likely a binary file
func isBinaryFile(path string) bool {
	// List of extensions that are typically binary files
	binaryExtensions := []string{
		".png", ".jpg", ".jpeg", ".gif", ".ico", ".zip", ".tar", ".gz", ".rar",
		".exe", ".dll", ".so", ".dylib", ".bin", ".dat", ".o", ".class",
	}

	ext := strings.ToLower(filepath.Ext(path))
	for _, binaryExt := range binaryExtensions {
		if ext == binaryExt {
			return true
		}
	}
	return false
}
