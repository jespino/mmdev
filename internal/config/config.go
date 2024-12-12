package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Jira       JiraConfig       `toml:"jira"`
	Sentry     SentryConfig     `toml:"sentry"`
}

type SentryConfig struct {
	Token string `toml:"token"`
}

type JiraConfig struct {
	URL      string `toml:"url"`
	Username string `toml:"username"`
	Token    string `toml:"token"`
}


func LoadConfig() (*Config, error) {
	config := &Config{}

	// Check environment variables first
	config.Jira.URL = os.Getenv("JIRA_URL")
	config.Jira.Username = os.Getenv("JIRA_USER")
	config.Jira.Token = os.Getenv("JIRA_TOKEN")
	config.Sentry.Token = os.Getenv("SENTRY_TOKEN")

	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Try to load config file
	configPath := filepath.Join(homeDir, ".mmdev.toml")
	if _, err := os.Stat(configPath); err == nil {
		if _, err := toml.DecodeFile(configPath, config); err != nil {
			return nil, fmt.Errorf("failed to decode config file: %w", err)
		}
	}

	return config, nil
}

func SaveConfig(config *Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".mmdev.toml")
	f, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return nil
}
