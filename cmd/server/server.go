package server

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/jespino/mmdev/cmd/docker"
	"github.com/jespino/mmdev/cmd/generate"
	"github.com/jespino/mmdev/pkg/server"
	"github.com/spf13/cobra"
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
		generate.GenerateCmd(),
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

			manager := server.NewManager(serverDir)
			if err := manager.Lint(); err != nil {
				fmt.Printf("Linting found issues: %v\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
	return cmd
}

func runServer() error {
	// Start docker services
	if err := docker.StartDockerServices(); err != nil {
		return fmt.Errorf("failed to start docker services: %w", err)
	}

	// Create channels to listen for signals
	sigChan := make(chan os.Signal, 1)
	restartChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	signal.Notify(restartChan, syscall.SIGUSR1)

	// Create a channel to signal server completion
	done := make(chan error, 1)

	// Start server in a goroutine
	manager := server.NewManager(".")
	cmd, err := manager.Start()
	if err != nil {
		done <- err
		return err
	}
	go func() {
		done <- cmd.Wait()
	}()

	// Wait for server completion, interrupt or restart signal
	for {
		select {
		case err := <-done:
			fmt.Println("Server process ended, cleaning up...")
			if err := docker.StopDockerServices(); err != nil {
				fmt.Printf("Warning: failed to stop docker services: %v\n", err)
			}
			return err
		case <-sigChan:
			fmt.Println("\nReceived interrupt signal. Shutting down...")
			if cmd != nil && cmd.Process != nil {
				if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
					fmt.Printf("Warning: failed to send SIGTERM to server: %v\n", err)
					cmd.Process.Kill()
				}
				// Wait for the process to finish
				<-done
			}
			fmt.Println("Stopping docker services...")
			if err := docker.StopDockerServices(); err != nil {
				fmt.Printf("Warning: failed to stop docker services: %v\n", err)
			}
			return nil
		case <-restartChan:
			fmt.Println("\nReceived restart signal. Restarting server...")
			if cmd != nil && cmd.Process != nil {
				if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
					fmt.Printf("Warning: failed to send SIGTERM to server: %v\n", err)
					cmd.Process.Kill()
				}
				// Wait for the process to finish
				<-done
			}
			// Start new server instance
			cmd, err = manager.Start()
			if err != nil {
				return fmt.Errorf("failed to restart server: %w", err)
			}
			go func() {
				done <- cmd.Wait()
			}()
		}
	}
}

func runWithWatcher() error {
	// Start docker services
	if err := docker.StartDockerServices(); err != nil {
		return fmt.Errorf("failed to start docker services: %w", err)
	}
	defer docker.StopDockerServices()

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

	// Create a channel to listen for interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start the server initially
	cmd = startServer()

	// Create a channel to signal server completion
	done := make(chan error, 1)
	if cmd != nil {
		go func() {
			done <- cmd.Wait()
		}()
	}

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

	// Handle restarts and signals
	for {
		select {
		case <-restart:
			fmt.Println("\nRestarting server...")
			if cmd != nil && cmd.Process != nil {
				if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
					fmt.Printf("Warning: failed to send SIGTERM to server: %v\n", err)
					cmd.Process.Kill()
				}
				<-done // Wait for process to finish
			}
			fmt.Println("Starting new server instance...")
			cmd = startServer()
			if cmd != nil {
				go func() {
					done <- cmd.Wait()
				}()
			}

		case <-sigChan:
			fmt.Println("\nReceived interrupt signal. Shutting down...")
			if cmd != nil && cmd.Process != nil {
				if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
					fmt.Printf("Warning: failed to send SIGTERM to server: %v\n", err)
					cmd.Process.Kill()
				}
				<-done // Wait for process to finish
			}
			fmt.Println("Stopping docker services...")
			if err := docker.StopDockerServices(); err != nil {
				fmt.Printf("Warning: failed to stop docker services: %v\n", err)
			}
			return nil

		case err := <-done:
			if err != nil {
				fmt.Printf("Server process ended with error: %v\n", err)
			}
			return err
		}
	}
}

func startServer() *exec.Cmd {
	manager := server.NewManager(".")
	cmd, err := manager.Start()
	if err != nil {
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
