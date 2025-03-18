package plugin

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/jespino/mmdev/pkg/plugins/manifest"
	"github.com/jespino/mmdev/pkg/plugins/pluginctl"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
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

var newCmd = &cobra.Command{
	Use:   "new <plugin-name>",
	Short: "Create a new plugin from the starter template",
	Args:  cobra.ExactArgs(1),
	RunE:  runNew,
}

	cmd.AddCommand(deployCmd)
	cmd.AddCommand(disableCmd)
	cmd.AddCommand(enableCmd)
	cmd.AddCommand(resetCmd)
	cmd.AddCommand(logsCmd)
	cmd.AddCommand(watchCmd)
	cmd.AddCommand(newCmd)
	
	manifestCmd.AddCommand(manifestApplyCmd)
	manifestCmd.AddCommand(manifestDistCmd)
	manifestCmd.AddCommand(manifestCheckCmd)
	cmd.AddCommand(manifestCmd)

	return cmd
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

func runNew(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}
	return client.NewPlugin(cmd.Context(), args[0])
}

func runManifestApply(cmd *cobra.Command, args []string) error {
	m, err := manifest.FindManifest()
	if err != nil {
		return fmt.Errorf("failed to find manifest: %w", err)
	}
	return manifest.Apply(m)
}

func runManifestDist(cmd *cobra.Command, args []string) error {
	m, err := manifest.FindManifest()
	if err != nil {
		return fmt.Errorf("failed to find manifest: %w", err)
	}
	return manifest.Dist(m)
}

func runManifestCheck(cmd *cobra.Command, args []string) error {
	m, err := manifest.FindManifest()
	if err != nil {
		return fmt.Errorf("failed to find manifest: %w", err)
	}
	if err := m.IsValid(); err != nil {
		return fmt.Errorf("invalid manifest: %w", err)
	}
	return nil
}
