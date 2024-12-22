package commits

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/coder/hnsw"
)

// SearchCommits searches for semantically similar commits using the HNSW index
func SearchCommits(query string, limit int, maxAge time.Duration) ([]string, error) {
	// Load the graph from disk
	graph := hnsw.NewGraph[string]()
	data, err := os.ReadFile(".commits.idx")
	if err != nil {
		return nil, fmt.Errorf("error loading index: %v", err)
	}
	if err := graph.Import(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("error importing index: %v", err)
	}

	// Create a simple vector from the query text
	vector := make([]float32, 128)
	for i, c := range query {
		if i < 128 {
			vector[i] = float32(c) / 255.0
		}
	}

	// Search the graph
	results := graph.Search(vector, limit)

	// Get commit dates to filter by age
	hashes := make([]string, 0, limit)
	for _, result := range results {
		// Get commit date
		gitCmd := exec.Command("git", "show", "-s", "--format=%aI", result.Key)
		output, err := gitCmd.Output()
		if err != nil {
			continue
		}

		date, err := time.Parse(time.RFC3339, strings.TrimSpace(string(output)))
		if err != nil {
			continue
		}

		// Check if commit is within maxAge
		if time.Since(date) <= maxAge {
			hashes = append(hashes, result.Key)
		}

		if len(hashes) >= limit {
			break
		}
	}

	return hashes, nil
}

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
