package translate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/jespino/mmdev/internal/config"
)

type TranslationStats struct {
	TranslatedPercent float64 `json:"translated_percent"`
	FuzzyPercent      float64 `json:"fuzzy_percent"`
	TotalStrings      int     `json:"total_strings"`
	TranslatedStrings int     `json:"translated"`
	FuzzyStrings      int     `json:"fuzzy"`
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

			stats, err := getTranslationStats(cfg.Weblate.URL, cfg.Weblate.Token, args[0])
			if err != nil {
				return fmt.Errorf("failed to get translation stats: %w", err)
			}

			// Print the stats
			fmt.Printf("Translation status for language: %s\n", args[0])
			fmt.Printf("Total strings: %d\n", stats.TotalStrings)
			fmt.Printf("Translated: %d (%.1f%%)\n", stats.TranslatedStrings, stats.TranslatedPercent)
			fmt.Printf("Fuzzy: %d (%.1f%%)\n", stats.FuzzyStrings, stats.FuzzyPercent)

			return nil
		},
	}

	return cmd
}

func getTranslationStats(baseURL, token, language string) (*TranslationStats, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	url := fmt.Sprintf("%s/api/components/mattermost/mattermost-server/%s/statistics/", baseURL, language)
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
