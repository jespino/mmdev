package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/jespino/mmdev/internal/config"
	"github.com/spf13/cobra"
)

func ConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "config",
		Short:        "Configure mmdev settings",
		RunE:         runConfig,
		SilenceUsage: true,
	}
	return cmd
}

func runConfig(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)
	cfg, err := config.LoadConfig()
	if err != nil {
		cfg = &config.Config{}
	}

	fmt.Println("\nJira Configuration")
	fmt.Println("=================")
	fmt.Println("To configure Jira integration, you'll need:")
	fmt.Println("1. Your Jira instance URL (e.g., https://your-domain.atlassian.net)")
	fmt.Println("2. Your Jira email address")
	fmt.Println("3. An API token from https://id.atlassian.com/manage-profile/security/api-tokens")
	fmt.Print("\nWould you like to configure Jira? (y/N): ")

	configureJira, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(configureJira)) == "y" {
		fmt.Printf("Jira URL [%s]: ", cfg.Jira.URL)
		url, _ := reader.ReadString('\n')
		url = strings.TrimSpace(url)
		if url != "" {
			cfg.Jira.URL = url
		}

		fmt.Printf("Jira Username [%s]: ", cfg.Jira.Username)
		username, _ := reader.ReadString('\n')
		username = strings.TrimSpace(username)
		if username != "" {
			cfg.Jira.Username = username
		}

		fmt.Printf("Jira API Token [%s]: ", maskToken(cfg.Jira.Token))
		token, _ := reader.ReadString('\n')
		token = strings.TrimSpace(token)
		if token != "" {
			cfg.Jira.Token = token
		}
	}

	fmt.Println("\nSentry Configuration")
	fmt.Println("===================")
	fmt.Println("To configure Sentry integration, you'll need:")
	fmt.Println("1. A Sentry auth token from https://sentry.io/settings/account/api/auth-tokens/")
	fmt.Println("   - Required scopes: event:read, project:read")
	fmt.Print("\nWould you like to configure Sentry? (y/N): ")

	configureSentry, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(configureSentry)) == "y" {
		fmt.Printf("Sentry API Token [%s]: ", maskToken(cfg.Sentry.Token))
		token, _ := reader.ReadString('\n')
		token = strings.TrimSpace(token)
		if token != "" {
			cfg.Sentry.Token = token
		}
	}

	fmt.Println("\nWeblate Configuration")
	fmt.Println("====================")
	fmt.Println("To configure Weblate integration, you'll need:")
	fmt.Println("1. The Weblate instance URL (e.g., https://translate.mattermost.com)")
	fmt.Println("2. An API token from your Weblate user settings")
	fmt.Print("\nWould you like to configure Weblate? (y/N): ")

	configureWeblate, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(configureWeblate)) == "y" {
		fmt.Printf("Weblate URL [%s]: ", cfg.Weblate.URL)
		url, _ := reader.ReadString('\n')
		url = strings.TrimSpace(url)
		if url != "" {
			cfg.Weblate.URL = url
		}

		fmt.Printf("Weblate API Token [%s]: ", maskToken(cfg.Weblate.Token))
		token, _ := reader.ReadString('\n')
		token = strings.TrimSpace(token)
		if token != "" {
			cfg.Weblate.Token = token
		}
	}

	return config.SaveConfig(cfg)
}

func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
}
