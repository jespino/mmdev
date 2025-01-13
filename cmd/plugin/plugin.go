package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourusername/yourproject/pkg/plugins/pluginctl"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Plugin management tool",
}

var manifestCmd = &cobra.Command{
	Use:   "manifest",
	Short: "Plugin manifest operations",
}

var manifestApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply manifest to generate server/webapp files",
	RunE:  runManifestApply,
}

var manifestDistCmd = &cobra.Command{
	Use:   "dist",
	Short: "Write manifest to dist directory",
	RunE:  runManifestDist,
}

var manifestCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate manifest",
	RunE:  runManifestCheck,
}

var deployCmd = &cobra.Command{
	Use:   "deploy <plugin-id> <bundle-path>",
	Short: "Deploy a plugin",
	Args:  cobra.ExactArgs(2),
	RunE:  runDeploy,
}

var disableCmd = &cobra.Command{
	Use:   "disable <plugin-id>",
	Short: "Disable a plugin",
	Args:  cobra.ExactArgs(1),
	RunE:  runDisable,
}

var enableCmd = &cobra.Command{
	Use:   "enable <plugin-id>",
	Short: "Enable a plugin",
	Args:  cobra.ExactArgs(1),
	RunE:  runEnable,
}

var resetCmd = &cobra.Command{
	Use:   "reset <plugin-id>",
	Short: "Reset a plugin",
	Args:  cobra.ExactArgs(1),
	RunE:  runReset,
}

var logsCmd = &cobra.Command{
	Use:   "logs <plugin-id>",
	Short: "Show plugin logs",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogs,
}

var watchCmd = &cobra.Command{
	Use:   "watch <plugin-id>",
	Short: "Watch plugin logs",
	Args:  cobra.ExactArgs(1),
	RunE:  runWatch,
}

func init() {
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(disableCmd)
	rootCmd.AddCommand(enableCmd)
	rootCmd.AddCommand(resetCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(watchCmd)
	
	manifestCmd.AddCommand(manifestApplyCmd)
	manifestCmd.AddCommand(manifestDistCmd)
	manifestCmd.AddCommand(manifestCheckCmd)
	rootCmd.AddCommand(manifestCmd)
}

func getClient() (*pluginctl.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return pluginctl.NewClient(ctx)
}

func runDeploy(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}
	return client.Deploy(cmd.Context(), args[0], args[1])
}

func runDisable(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}
	return client.Disable(cmd.Context(), args[0])
}

func runEnable(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}
	return client.Enable(cmd.Context(), args[0])
}

func runReset(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}
	return client.Reset(cmd.Context(), args[0])
}

func runLogs(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}
	return client.GetLogs(cmd.Context(), args[0])
}

func runWatch(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}
	return client.WatchLogs(context.Background(), args[0])
}

func runManifestApply(cmd *cobra.Command, args []string) error {
	manifest, err := manifest.FindManifest()
	if err != nil {
		return fmt.Errorf("failed to find manifest: %w", err)
	}
	return manifest.Apply(manifest)
}

func runManifestDist(cmd *cobra.Command, args []string) error {
	manifest, err := manifest.FindManifest()
	if err != nil {
		return fmt.Errorf("failed to find manifest: %w", err)
	}
	return manifest.Dist(manifest)
}

func runManifestCheck(cmd *cobra.Command, args []string) error {
	manifest, err := manifest.FindManifest()
	if err != nil {
		return fmt.Errorf("failed to find manifest: %w", err)
	}
	return manifest.IsValid()
}
