package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindMattermostBaseDir finds the Mattermost base directory by looking for .git
// and verifying it's a Mattermost repository
func FindMattermostBaseDir() (string, error) {
	// Start from current directory
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	// Keep going up until we find .git
	for {
		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			// Found .git, verify it's a Mattermost repository
			if isMattermostRepo(dir) {
				return dir, nil
			}
			return "", fmt.Errorf("directory %s is not a Mattermost repository", dir)
		}

		// Go up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding .git
			return "", fmt.Errorf("not inside a Mattermost repository")
		}
		dir = parent
	}
}

// isMattermostRepo checks if the given directory is a Mattermost repository
func isMattermostRepo(dir string) bool {
	// Check for some Mattermost-specific files/directories
	indicators := []string{
		"server/Makefile",
		"webapp/package.json",
		"LICENSE.txt", // Contains Mattermost copyright
	}

	for _, indicator := range indicators {
		path := filepath.Join(dir, indicator)
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}
