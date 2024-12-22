package indexcommits

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/coder/hnsw"
	"github.com/spf13/cobra"
)

type Commit struct {
	Hash    string    `json:"hash"`
	Message string    `json:"message"`
	Date    time.Time `json:"date"`
	Vector  []float32 `json:"vector"`
}

type CommitIndex struct {
	Graph *hnsw.Graph[string]
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
		date, err := time.Parse(time.RFC3339, parts[2])
		if err != nil {
			continue
		}

		// Create a simple vector from the commit message
		// This is a very basic approach - in a real implementation,
		// you might want to use a proper embedding model
		vector := make([]float32, 128)
		for i, c := range message {
			if i < 128 {
				vector[i] = float32(c) / 255.0
			}
		}

		// Add the commit to the graph
		node := hnsw.MakeNode(hash, vector)
		graph.Add(node)
	}

	// Save the graph
	savedGraph := &hnsw.SavedGraph[string]{
		Graph: graph,
		Path:  ".commits.idx",
	}

	if err := savedGraph.Save(); err != nil {
		return fmt.Errorf("error saving index: %v", err)
	}

	fmt.Printf("Successfully indexed %d commits\n", len(commits))
	return nil
}
