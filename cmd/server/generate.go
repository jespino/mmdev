package server

import (
	"fmt"
	"os"

	"github.com/jespino/mmdev/pkg/generator"
	"github.com/spf13/cobra"
)

func GenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Code generation commands",
	}

	cmd.AddCommand(
		LayersCmd(),
		MocksCmd(),
		AllCmd(),
	)

	return cmd
}

func LayersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "layers",
		Short: "Generate all layer code",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverDir := "./server"
			if _, err := os.Stat(serverDir); os.IsNotExist(err) {
				return fmt.Errorf("server directory not found at %s", serverDir)
			}

			manager := generator.NewManager(serverDir)

			if err := manager.GenerateAppLayers(); err != nil {
				return fmt.Errorf("failed to generate app layers: %w", err)
			}

			if err := manager.GenerateStoreLayers(); err != nil {
				return fmt.Errorf("failed to generate store layers: %w", err)
			}

			if err := manager.GeneratePluginAPI(); err != nil {
				return fmt.Errorf("failed to generate plugin API: %w", err)
			}

			return nil
		},
	}
	return cmd
}

func MocksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mocks",
		Short: "Generate all mock files",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverDir := "./server"
			if _, err := os.Stat(serverDir); os.IsNotExist(err) {
				return fmt.Errorf("server directory not found at %s", serverDir)
			}

			manager := generator.NewManager(serverDir)
			if err := manager.GenerateMocks(); err != nil {
				return fmt.Errorf("failed to generate mocks: %w", err)
			}

			return nil
		},
	}
	return cmd
}

func AllCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "all",
		Short: "Generate all code (layers and mocks)",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverDir := "./server"
			if _, err := os.Stat(serverDir); os.IsNotExist(err) {
				return fmt.Errorf("server directory not found at %s", serverDir)
			}

			manager := generator.NewManager(serverDir)

			if err := manager.GenerateAppLayers(); err != nil {
				return fmt.Errorf("failed to generate app layers: %w", err)
			}

			if err := manager.GenerateStoreLayers(); err != nil {
				return fmt.Errorf("failed to generate store layers: %w", err)
			}

			if err := manager.GeneratePluginAPI(); err != nil {
				return fmt.Errorf("failed to generate plugin API: %w", err)
			}

			if err := manager.GenerateMocks(); err != nil {
				return fmt.Errorf("failed to generate mocks: %w", err)
			}

			return nil
		},
	}
	return cmd
}
