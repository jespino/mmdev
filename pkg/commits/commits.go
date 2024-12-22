package commits

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

// SearchAndCreatePatchFiles searches for related commits and creates temporary patch files
func SearchAndCreatePatchFiles(searchQuery string, limit int, maxAge time.Duration) ([]string, []string, error) {
	// Search for related commits
	relatedCommits, err := SearchCommits(searchQuery, limit, maxAge)
	if err != nil {
		return nil, nil, fmt.Errorf("error searching commits: %v", err)
	}

	// Create temporary patch files for each related commit
	var patchFiles []string
	var createdFiles []string
	for i, hash := range relatedCommits {
		patchFile, err := os.CreateTemp("", fmt.Sprintf("commit-%d-*.patch", i))
		if err != nil {
			return nil, createdFiles, fmt.Errorf("error creating patch file: %v", err)
		}
		createdFiles = append(createdFiles, patchFile.Name())
		patchFiles = append(patchFiles, "--read", patchFile.Name())

		// Generate patch using git show
		gitCmd := exec.Command("git", "show", hash)
		patch, err := gitCmd.Output()
		if err != nil {
			return nil, createdFiles, fmt.Errorf("error generating patch for commit %s: %v", hash, err)
		}

		if err := os.WriteFile(patchFile.Name(), patch, 0644); err != nil {
			return nil, createdFiles, fmt.Errorf("error writing patch file: %v", err)
		}
	}

	return patchFiles, createdFiles, nil
}
