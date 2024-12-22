package indexcommits

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/coder/hnsw"
	"github.com/jespino/mmdev/pkg/embedding"
	"github.com/spf13/cobra"
)


type CommitIndex struct {
	Graph *hnsw.Graph[string]
}

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

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index-commits",
		Short: "Index git commits for semantic search",
		Long:  `Creates a semantic index of git commits in the current repository for later searching.`,
		RunE:  runIndexCommits,
	}
	return cmd
}

func runIndexCommits(cmd *cobra.Command, args []string) error {
	// Check if we're in a git repository
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository (or any of the parent directories)")
	}

	// Get all commits
	gitCmd := exec.Command("git", "log", "--pretty=format:%H|||%s|||%aI")
	output, err := gitCmd.Output()
	if err != nil {
		return fmt.Errorf("error getting git commits: %v", err)
	}

	// Create a new graph
	graph := hnsw.NewGraph[string]()
	graph.M = 16        // Maximum number of connections per node
	graph.Ml = 0.25     // Level generation factor
	graph.EfSearch = 20 // Number of nodes to consider during search

	// Process each commit
	commits := strings.Split(string(output), "\n")
	for _, commit := range commits {
		parts := strings.Split(commit, "|||")
		if len(parts) != 3 {
			continue
		}

		hash := parts[0]
		message := parts[1]

		// Build vocabulary from all commit messages first
		vocab := embedding.NewVocabulary()
		for _, c := range commits {
			parts := strings.Split(c, "|||")
			if len(parts) == 3 {
				vocab.AddDocument(parts[1]) // Add commit message
			}
		}
		vocab.Finalize()

		// Create vector from commit message using TF-IDF
		vector := vocab.CreateVector(message)

		// Add the commit to the graph
		node := hnsw.MakeNode(hash, vector)
		graph.Add(node)
	}

	// Save the graph to disk
	var buf bytes.Buffer
	if err := graph.Export(&buf); err != nil {
		return fmt.Errorf("error exporting index: %v", err)
	}
	data := buf.Bytes()
	if err := os.WriteFile(".commits.idx", data, 0644); err != nil {
		return fmt.Errorf("error saving index: %v", err)
	}

	fmt.Printf("Successfully indexed %d commits\n", len(commits))
	return nil
}
