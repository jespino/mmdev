package translate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jespino/mmdev/internal/config"
)

type Component struct {
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	Project struct {
		Slug string `json:"slug"`
	} `json:"project"`
}

type ComponentsResponse struct {
	Results []Component `json:"results"`
}

type TranslationStats struct {
	TranslatedPercent float64 `json:"translated_percent"`
	FuzzyPercent      float64 `json:"fuzzy_percent"`
	TotalStrings      int     `json:"total_strings"`
	TranslatedStrings int     `json:"translated"`
	FuzzyStrings      int     `json:"fuzzy"`
}

func NewComponentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "components",
		Short: "List available Weblate components",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if cfg.Weblate.URL == "" {
				return fmt.Errorf("Weblate URL not configured. Set WEBLATE_URL environment variable or configure in ~/.mmdev.toml")
			}

			if cfg.Weblate.Token == "" {
				return fmt.Errorf("Weblate token not configured. Set WEBLATE_TOKEN environment variable or configure in ~/.mmdev.toml")
			}

			components, err := getComponents(cfg.Weblate.URL, cfg.Weblate.Token)
			if err != nil {
				return fmt.Errorf("failed to get components: %w", err)
			}

			for _, comp := range components.Results {
				fmt.Printf("%s: %s %s\n", comp.Slug, comp.Project.Slug, comp.Name)
			}

			return nil
		},
	}

	cmd.AddCommand(NewComponentsCmd())
	return cmd
}

func NewTranslateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "translate [language]",
		Short: "Get translation status for a specific language",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if cfg.Weblate.URL == "" {
				return fmt.Errorf("Weblate URL not configured. Set WEBLATE_URL environment variable or configure in ~/.mmdev.toml")
			}

			if cfg.Weblate.Token == "" {
				return fmt.Errorf("Weblate token not configured. Set WEBLATE_TOKEN environment variable or configure in ~/.mmdev.toml")
			}

			// Get the translation stats
			stats, err := getTranslationStats(cfg.Weblate.URL, cfg.Weblate.Token, args[0])
			if err != nil {
				return fmt.Errorf("failed to get translation stats: %w", err)
			}

			// Print the translation stats
			fmt.Printf("Translation status for language: %s\n", args[0])
			fmt.Printf("Total strings: %d\n", stats.TotalStrings)
			fmt.Printf("Translated: %d (%.1f%%)\n", stats.TranslatedStrings, stats.TranslatedPercent)
			fmt.Printf("Fuzzy: %d (%.1f%%)\n", stats.FuzzyStrings, stats.FuzzyPercent)

			return nil
		},
	}

	return cmd
}

func joinURL(base, path string) string {
	base = strings.TrimSuffix(base, "/")
	path = strings.TrimPrefix(path, "/")
	return base + "/" + path
}

func getComponents(baseURL, token string) (*ComponentsResponse, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	url := joinURL(baseURL, "/api/components/")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Token %s", token))
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var components ComponentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&components); err != nil {
		return nil, err
	}

	return &components, nil
}

func getTranslationStats(baseURL, token, language string) (*TranslationStats, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	url := joinURL(baseURL, fmt.Sprintf("/api/components/mattermost/mattermost-server/%s/statistics/", language))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Token %s", token))
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var stats TranslationStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, err
	}

	return &stats, nil
}
