package translate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
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
	Language              string    `json:"name"`
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

func NewLanguagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "languages",
		Short: "List available languages",
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

			languages, err := getLanguages(cfg.Weblate.URL, cfg.Weblate.Token)
			if err != nil {
				return fmt.Errorf("failed to get languages: %w", err)
			}

			fmt.Printf("Available languages (%d):\n\n", languages.Count)

			// Print header
			fmt.Printf("%-10s %-30s %10s %12s %10s\n",
				"Code", "Name", "Direction", "Total", "Translated")
			fmt.Println(strings.Repeat("-", 75))

			// Print each row
			for _, lang := range languages.Results {
				fmt.Printf("%-10s %-30s %10s %12d %10d\n",
					lang.Code,
					lang.Name,
					lang.Direction,
					lang.TotalStrings,
					lang.Translated)
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
		NewLanguagesCmd(),
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

type Language struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Direction    string `json:"direction"`
	WebURL       string `json:"web_url"`
	TotalStrings int    `json:"total_strings"`
	Translated   int    `json:"translated"`
}

type LanguagesResponse struct {
	Count    int        `json:"count"`
	Next     *string    `json:"next"`
	Previous *string    `json:"previous"`
	Results  []Language `json:"results"`
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

func getLanguages(baseURL, token string) (*LanguagesResponse, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	url := joinURL(baseURL, "/api/languages/")
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

	var languages LanguagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&languages); err != nil {
		return nil, err
	}

	return &languages, nil
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

			fmt.Printf("Statistics for component: %s:%s (%d results)\n\n", parts[0], parts[1], statsResp.Count)

			// Sort results by translated percentage in descending order
			sort.Slice(statsResp.Results, func(i, j int) bool {
				return statsResp.Results[i].TranslatedPercent > statsResp.Results[j].TranslatedPercent
			})

			// Print header
			fmt.Printf("%-20s %8s %12s %8s %10s %10s %12s %10s\n",
				"Language", "Total", "Translated%", "Fuzzy%", "Failing%", "Approved%", "Suggestions", "Comments")
			fmt.Println(strings.Repeat("-", 95))

			// Print each row
			for _, stats := range statsResp.Results {
				fmt.Printf("%-20s %8d %11.1f%% %7.1f%% %9.1f%% %9.1f%% %12d %10d\n",
					stats.Language,
					stats.Total,
					stats.TranslatedPercent,
					stats.FuzzyPercent,
					stats.FailingPercent,
					stats.ApprovedPercent,
					stats.Suggestions,
					stats.Comments)
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
