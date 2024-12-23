package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/chzyer/readline"

	anthropic "github.com/adamchol/go-anthropic-sdk"
	"github.com/spf13/cobra"

	"github.com/jespino/mmdev/internal/config"
)

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorYellow = "\033[33m"
)

func formatNumber(n int) string {
	// Handle negative numbers
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}

	// Convert number to string
	str := fmt.Sprintf("%d", n)

	// Add commas
	var result []byte
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}

	return sign + string(result)
}

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

type ComponentStats struct {
	Language               string    `json:"name"`
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
	var showAll bool

	cmd := &cobra.Command{
		Use:   "languages",
		Short: "List available languages",
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

			languages, err := getLanguages(cfg.Weblate.URL, cfg.Weblate.Token)
			if err != nil {
				return fmt.Errorf("failed to get languages: %w", err)
			}

			if showAll {
				fmt.Printf("All languages (%d total):\n\n", languages.Count)
			} else {
				fmt.Printf("Most spoken languages (showing top 50 out of %d):\n\n", languages.Count)
			}

			// Sort languages by population in descending order
			sort.Slice(languages.Results, func(i, j int) bool {
				return languages.Results[i].Population > languages.Results[j].Population
			})

			// Print header
			fmt.Printf("%-20s %-50s %15s\n", "Code", "Name", "Population")
			fmt.Println(strings.Repeat("-", 85))

			// Print top 30 rows
			displayCount := len(languages.Results)
			if !showAll {
				displayCount = 50
				if len(languages.Results) < displayCount {
					displayCount = len(languages.Results)
				}
			}

			for _, lang := range languages.Results[:displayCount] {
				fmt.Printf("%-20s %-50s %15s\n",
					lang.Code,
					lang.Name,
					formatNumber(lang.Population))
			}

			if !showAll && len(languages.Results) > displayCount {
				fmt.Printf("\nNote: %d other languages available (use --all to show all)\n", len(languages.Results)-displayCount)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&showAll, "all", false, "Show all languages instead of just top 50")

	return cmd
}

func getNextTranslationUnitsPage(baseURL, token, project, component, language string, nextURL *string) (*TranslationUnitsResponse, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	url := nextURL
	if url == nil {
		initialURL := joinURL(baseURL, fmt.Sprintf("/api/translations/%s/%s/%s/units/?q=state:<translated", project, component, language))
		url = &initialURL
	}

	req, err := http.NewRequest("GET", *url, nil)
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

	var pageResponse TranslationUnitsResponse
	if err := json.NewDecoder(resp.Body).Decode(&pageResponse); err != nil {
		return nil, err
	}

	return &pageResponse, nil
}

func NewTranslateTranslateCmd() *cobra.Command {
	var useAI bool

	cmd := &cobra.Command{
		Use:   "translate <project:component> <language>",
		Short: "Interactive translation wizard for a component and language",
		Args:  cobra.ExactArgs(2),
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

			project, component := parts[0], parts[1]
			language := args[1]

			// Get first page to get total count
			firstPage, err := getNextTranslationUnitsPage(cfg.Weblate.URL, cfg.Weblate.Token, project, component, language, nil)
			if err != nil {
				return fmt.Errorf("failed to get translation units: %w", err)
			}

			fmt.Printf("Starting translation wizard for %s:%s in %s\n\n", project, component, language)

			nextURL := firstPage.Next
			currentPage := firstPage
			processedCount := 0
			translatedCount := 0

			for {
				// Process current page
				for _, unit := range currentPage.Results {
					if unit.Translated {
						continue
					}

					processedCount++
					var promptBuilder strings.Builder

					promptBuilder.WriteString(fmt.Sprintf("%sTranslating id:%s %s\n", colorCyan, colorReset, unit.Context))
					promptBuilder.WriteString(strings.Repeat("-", 50) + "\n\n")

					if unit.PreviousSource != "" {
						promptBuilder.WriteString(fmt.Sprintf("%sPrevious Source:%s\n%s\n", colorYellow, colorReset, unit.PreviousSource))
					}
					promptBuilder.WriteString(fmt.Sprintf("%sCurrent Source:%s\n%s\n", colorYellow, colorReset, strings.Join(unit.Source, "")))
					if len(unit.Target) > 0 {
						promptBuilder.WriteString(fmt.Sprintf("%sCurrent Translation:%s\n%s\n", colorYellow, colorReset, strings.Join(unit.Target, "")))
					}

					var suggestion string
					if useAI {
						if os.Getenv("ANTHROPIC_API_KEY") == "" {
							return fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
						}

						aiTranslation, err := getAITranslation(unit.Source, unit.Target, unit.Context, unit.Note, language)
						if err != nil {
							promptBuilder.WriteString(fmt.Sprintf("Warning: Failed to get AI translation: %v\n", err))
						} else {
							suggestion = aiTranslation
							promptBuilder.WriteString(fmt.Sprintf("%sAI Suggested Translation:%s\n%s\n", colorYellow, colorReset, suggestion))
						}
					}

					aiHelp := ""
					if useAI {
						aiHelp = ", 'y' or Ctrl+Y=accept AI suggestion"
					}
					promptBuilder.WriteString(fmt.Sprintf("\nEnter translation [%d remaining] (Alt+Enter=newline%s, Ctrl+C=skip, Ctrl+D=quit): ",
						firstPage.Count-translatedCount, aiHelp))

					rl, err := readline.NewEx(&readline.Config{
						Prompt:                 promptBuilder.String(),
						InterruptPrompt:        "^C",
						EOFPrompt:              "^D",
						HistorySearchFold:      true,
						UniqueEditLine:         true,
						DisableAutoSaveHistory: true,
						VimMode:                false,
						Stdin:                  readline.NewCancelableStdin(os.Stdin),
					})
					if err != nil {
						return fmt.Errorf("error initializing readline: %w", err)
					}
					defer rl.Close()

					input, err := rl.ReadlineWithDefault("")
					if err != nil {
						if err == readline.ErrInterrupt {
							fmt.Println("Skipping...")
							continue
						}
						if err == io.EOF {
							fmt.Println("Exiting translation wizard")
							return nil
						}
						// Check for Ctrl+A (ASCII value 1)
						if err.Error() == "char: 1" && suggestion != "" {
							input = suggestion
						} else {
							return fmt.Errorf("error reading input: %w", err)
						}
					}

					if (input == "y" || input == "\x01") && suggestion != "" {
						input = suggestion
					}

					// Submit translation
					err = submitTranslation(cfg.Weblate.URL, cfg.Weblate.Token, unit.ID, input)
					if err != nil {
						return fmt.Errorf("failed to submit translation: %w", err)
					}
					fmt.Println("Translation submitted successfully!")
					translatedCount++
					fmt.Println(strings.Repeat("-", 80))
				}

				// Check if we need to fetch next page
				if len(currentPage.Results) == 0 || nextURL == nil {
					break
				}

				currentPage, err = getNextTranslationUnitsPage(cfg.Weblate.URL, cfg.Weblate.Token, project, component, language, nextURL)
				if err != nil {
					return fmt.Errorf("failed to get next page of translation units: %w", err)
				}
				nextURL = currentPage.Next
			}

			if processedCount == 0 {
				fmt.Println("No untranslated units found!")
			} else {
				fmt.Printf("\nCompleted translation wizard. Translated %d units.\n", translatedCount)
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&useAI, "ai", false, "Use AI to suggest translations")

	return cmd
}

func getAITranslation(source []string, currentTranslation []string, ctx, note string, targetLang string) (string, error) {
	client := anthropic.NewClient(os.Getenv("ANTHROPIC_API_KEY"))

	var prompt strings.Builder
	prompt.WriteString("You are a professional translator for the Mattermost application. ")
	prompt.WriteString(fmt.Sprintf("Translate the following text from English to %s:\n\n", targetLang))
	prompt.WriteString(fmt.Sprintf("Previous source text: %s\n", strings.Join(source, "")))
	prompt.WriteString(fmt.Sprintf("Current source text: %s\n", strings.Join(source, "")))

	if len(currentTranslation) > 0 {
		prompt.WriteString(fmt.Sprintf("Current translation (only modify parts that need to change): %s\n", strings.Join(currentTranslation, "")))
	}

	if ctx != "" {
		prompt.WriteString(fmt.Sprintf("Context: %s\n", ctx))
	}
	if note != "" {
		prompt.WriteString(fmt.Sprintf("Note: %s\n", note))
	}

	prompt.WriteString("\nProvide only the translation, without any explanations or additional text.")

	resp, err := client.CreateMessage(context.Background(), anthropic.MessageRequest{
		Model:     anthropic.Claude3OpusModel,
		MaxTokens: 1024,
		System:    "You are a professional translator for the Mattermost application.",
		Messages: []anthropic.InputMessage{
			{
				Role:    anthropic.MessageRoleUser,
				Content: prompt.String(),
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("AI translation error: %w", err)
	}

	if len(resp.Content) == 0 {
		return "", fmt.Errorf("no content in AI response")
	}
	return resp.Content[0].Text, nil
}

func NewTranslateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "translate",
		Short: "Manage translations",
	}

	cmd.AddCommand(
		NewComponentsCmd(),
		NewComponentStatsCmd(),
		NewLanguagesCmd(),
		NewTranslateTranslateCmd(),
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
	Count    int              `json:"count"`
	Next     *string          `json:"next"`
	Previous *string          `json:"previous"`
	Results  []ComponentStats `json:"results"`
}

type Language struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Direction    string `json:"direction"`
	WebURL       string `json:"web_url"`
	TotalStrings int    `json:"total_strings"`
	Translated   int    `json:"translated"`
	Population   int    `json:"population"`
}

type LanguagesResponse struct {
	Count    int        `json:"count"`
	Next     *string    `json:"next"`
	Previous *string    `json:"previous"`
	Results  []Language `json:"results"`
}

type TranslationUnit struct {
	Translation     string    `json:"translation"`
	Source          []string  `json:"source"`
	PreviousSource  string    `json:"previous_source"`
	Target          []string  `json:"target"`
	IDHash          int       `json:"id_hash"`
	ContentHash     int       `json:"content_hash"`
	Location        string    `json:"location"`
	Context         string    `json:"context"`
	Note            string    `json:"note"`
	Flags           string    `json:"flags"`
	Labels          []string  `json:"labels"`
	State           int       `json:"state"`
	Fuzzy           bool      `json:"fuzzy"`
	Translated      bool      `json:"translated"`
	Approved        bool      `json:"approved"`
	Position        int       `json:"position"`
	HasSuggestion   bool      `json:"has_suggestion"`
	HasComment      bool      `json:"has_comment"`
	HasFailingCheck bool      `json:"has_failing_check"`
	NumWords        int       `json:"num_words"`
	Priority        int       `json:"priority"`
	ID              int       `json:"id"`
	Explanation     string    `json:"explanation"`
	ExtraFlags      string    `json:"extra_flags"`
	WebURL          string    `json:"web_url"`
	SourceUnit      string    `json:"source_unit"`
	Pending         bool      `json:"pending"`
	Timestamp       time.Time `json:"timestamp"`
	LastUpdated     time.Time `json:"last_updated"`
}

type TranslationUnitsResponse struct {
	Count    int               `json:"count"`
	Next     *string           `json:"next"`
	Previous *string           `json:"previous"`
	Results  []TranslationUnit `json:"results"`
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

	var allLanguages LanguagesResponse
	nextURL := joinURL(baseURL, "/api/languages/")

	for nextURL != "" {
		req, err := http.NewRequest("GET", nextURL, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", fmt.Sprintf("Token %s", token))
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		var pageResponse LanguagesResponse
		if err := json.NewDecoder(resp.Body).Decode(&pageResponse); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
		}

		allLanguages.Results = append(allLanguages.Results, pageResponse.Results...)
		allLanguages.Count = pageResponse.Count

		if pageResponse.Next != nil {
			nextURL = *pageResponse.Next
		} else {
			nextURL = ""
		}
	}

	return &allLanguages, nil
}

func submitTranslation(baseURL, token string, unitID int, translation string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	url := joinURL(baseURL, fmt.Sprintf("/api/units/%d/", unitID))

	payload := map[string]interface{}{
		"target": []string{translation},
		"state":  20, // Translated state
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling payload: %w", err)
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Token %s", token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func NewComponentStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "component-stats <project:component>",
		Short: "Get translation statistics for a specific component",
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
