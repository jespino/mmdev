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

type ComponentStats struct {
	Total                  int       `json:"total"`
	TotalWords             int       `json:"total_words"`
	TotalChars             int       `json:"total_chars"`
	LastChange             time.Time `json:"last_change"`
	Translated             int       `json:"translated"`
	TranslatedPercent      float64   `json:"translated_percent"`
	TranslatedWords        int       `json:"translated_words"`
	TranslatedWordsPercent float64   `json:"translated_words_percent"`
	TranslatedChars        int       `json:"translated_chars"`
	TranslatedCharsPercent float64   `json:"translated_chars_percent"`
	Fuzzy                  int       `json:"fuzzy"`
	FuzzyPercent           float64   `json:"fuzzy_percent"`
	Failing                int       `json:"failing"`
	FailingPercent         float64   `json:"failing_percent"`
	Approved               int       `json:"approved"`
	ApprovedPercent        float64   `json:"approved_percent"`
	Suggestions            int       `json:"suggestions"`
	Comments               int       `json:"comments"`
}

func NewComponentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "components",
		Short: "List available Weblate components",
		Args:  cobra.NoArgs,
		Annotations: map[string]string{
			"standalone": "true",
		},
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
				fmt.Printf("%s:%s\n", comp.Project.Slug, comp.Slug)
			}

			return nil
		},
	}

	return cmd
}

func NewTranslateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "translate",
		Short: "Manage translations",
		Annotations: map[string]string{
			"standalone": "true",
		},
	}

	cmd.AddCommand(
		NewComponentsCmd(),
		NewStatsCmd(),
		NewComponentStatsCmd(),
	)

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

type ComponentStatsResponse struct {
	Count    int             `json:"count"`
	Next     *string         `json:"next"`
	Previous *string         `json:"previous"`
	Results  []ComponentStats `json:"results"`
}

func getComponentStats(baseURL, token, project, component string) (*ComponentStatsResponse, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	url := joinURL(baseURL, fmt.Sprintf("/api/components/%s/%s/statistics/", project, component))
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

	var statsResp ComponentStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&statsResp); err != nil {
		return nil, err
	}

	return &statsResp, nil
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
func NewComponentStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "component-stats <project:component>",
		Short: "Get translation statistics for a specific component",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			"standalone": "true",
		},
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

			parts := strings.Split(args[0], ":")
			if len(parts) != 2 {
				return fmt.Errorf("invalid format. Use project:component")
			}

			statsResp, err := getComponentStats(cfg.Weblate.URL, cfg.Weblate.Token, parts[0], parts[1])
			if err != nil {
				return fmt.Errorf("failed to get component stats: %w", err)
			}

			fmt.Printf("Statistics for component: %s:%s\n", parts[0], parts[1])
			fmt.Printf("Total results: %d\n\n", statsResp.Count)

			for i, stats := range statsResp.Results {
				if i > 0 {
					fmt.Println("\n---\n")
				}
				fmt.Printf("Total strings: %d (words: %d, chars: %d)\n", stats.Total, stats.TotalWords, stats.TotalChars)
				fmt.Printf("Translated: %d (%.1f%%)\n", stats.Translated, stats.TranslatedPercent)
				fmt.Printf("Fuzzy: %d (%.1f%%)\n", stats.Fuzzy, stats.FuzzyPercent)
				fmt.Printf("Failing checks: %d (%.1f%%)\n", stats.Failing, stats.FailingPercent)
				fmt.Printf("Approved: %d (%.1f%%)\n", stats.Approved, stats.ApprovedPercent)
				fmt.Printf("Suggestions: %d\n", stats.Suggestions)
				fmt.Printf("Comments: %d\n", stats.Comments)
				fmt.Printf("Last change: %s\n", stats.LastChange.Format(time.RFC3339))
			}

			return nil
		},
	}

	return cmd
}

func NewStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats [language]",
		Short: "Get translation status for a specific language",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			"standalone": "true",
		},
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

			fmt.Printf("Translation status for language: %s\n", args[0])
			fmt.Printf("Total strings: %d\n", stats.TotalStrings)
			fmt.Printf("Translated: %d (%.1f%%)\n", stats.TranslatedStrings, stats.TranslatedPercent)
			fmt.Printf("Fuzzy: %d (%.1f%%)\n", stats.FuzzyStrings, stats.FuzzyPercent)

			return nil
		},
	}

	return cmd
}
