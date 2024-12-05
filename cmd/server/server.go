package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/jespino/mmdev/pkg/server"
)

var watch bool

func ServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Server related commands",
	}

	cmd.AddCommand(
		StartCmd(),
		LintCmd(),
	)
	return cmd
}

func LintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Run linting on the server code",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverDir := "./server"
			if _, err := os.Stat(serverDir); os.IsNotExist(err) {
				return fmt.Errorf("server directory not found at %s", serverDir)
			}

			// Change to server directory
			if err := os.Chdir(serverDir); err != nil {
				return fmt.Errorf("failed to change to server directory: %w", err)
			}

			// Run make check-style
			makeCmd := exec.Command("make", "check-style")
			makeCmd.Stdout = os.Stdout
			makeCmd.Stderr = os.Stderr
			makeCmd.Env = os.Environ()

			if err := makeCmd.Run(); err != nil {
				return fmt.Errorf("failed to run linting: %w", err)
			}

			return nil
		},
	}
	return cmd
}

func cleanup() error {
	manager := server.NewManager(".")
	if err := manager.Stop(); err != nil {
		return fmt.Errorf("failed to cleanup server: %w", err)
	}
	return nil
}

func runServer() error {
	manager := server.NewManager(".")
	
	if err := manager.Start(); err != nil {
		return fmt.Errorf("failed to run server: %w", err)
	}

	return nil
}

func runWithWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	// Add all directories with .go files
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && !strings.Contains(path, "vendor") && !strings.Contains(path, "node_modules") {
			return watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to add directories to watcher: %w", err)
	}

	var cmd *exec.Cmd
	var mu sync.Mutex
	restart := make(chan struct{}, 1)
	
	// Start the server initially
	cmd = startServer()

	// Debounce function to prevent multiple restarts
	lastRestart := time.Now()
	shouldRestart := func() bool {
		mu.Lock()
		defer mu.Unlock()
		if time.Since(lastRestart) < time.Second {
			return false
		}
		lastRestart = time.Now()
		return true
	}

	// Watch for changes
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// Only watch .go files that aren't test files
				if !strings.HasSuffix(event.Name, ".go") || strings.HasSuffix(event.Name, "_test.go") {
					continue
				}

				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 && shouldRestart() {
					select {
					case restart <- struct{}{}:
					default:
					}
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Fprintf(os.Stderr, "Watcher error: %v\n", err)
			}
		}
	}()

	// Handle restarts
	for range restart {
		fmt.Println("\nRestarting server...")
		if cmd != nil && cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				fmt.Fprintf(os.Stderr, "Error killing process: %v\n", err)
			}
			cmd.Wait()
			if err := cleanup(); err != nil {
				fmt.Fprintf(os.Stderr, "Error during cleanup: %v\n", err)
			}
		}
		cmd = startServer()
	}

	return nil
}

func startServer() *exec.Cmd {
	env := os.Environ()
	env = append(env, "RUN_SERVER_IN_BACKGROUND=false")
	
	cmd := exec.Command("make", "run-server")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		return nil
	}

	return cmd
}

func StartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the server",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverDir := "./server"
			if _, err := os.Stat(serverDir); os.IsNotExist(err) {
				return fmt.Errorf("server directory not found at %s", serverDir)
			}

			// Change to server directory
			if err := os.Chdir(serverDir); err != nil {
				return fmt.Errorf("failed to change to server directory: %w", err)
			}

			if watch {
				return runWithWatcher()
			}

			return runServer()
		},
	}

	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes and restart server")
	return cmd
}
