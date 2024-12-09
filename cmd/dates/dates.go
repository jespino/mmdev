package dates

import (
	"fmt"
	"time"

	jira "github.com/andygrunwald/go-jira"
	"github.com/jespino/mmdev/internal/config"
	"github.com/spf13/cobra"
)

func DatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dates",
		Short: "Show next Mattermost release dates",
		RunE:  runDates,
	}
	return cmd
}

func runDates(cmd *cobra.Command, args []string) error {
	// Load configuration
	config, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %v", err)
	}

	if config.Jira.URL == "" {
		return fmt.Errorf("Jira URL not configured. Set it in ~/.mmdev.toml or JIRA_URL environment variable")
	}
	if config.Jira.Username == "" {
		return fmt.Errorf("Jira username not configured. Set it in ~/.mmdev.toml or JIRA_USER environment variable")
	}
	if config.Jira.Token == "" {
		return fmt.Errorf("Jira token not configured. Set it in ~/.mmdev.toml or JIRA_TOKEN environment variable")
	}

	// Create Jira client
	tp := jira.BasicAuthTransport{
		Username: config.Jira.Username,
		Password: config.Jira.Token,
	}
	client, err := jira.NewClient(tp.Client(), config.Jira.URL)
	if err != nil {
		return fmt.Errorf("error creating Jira client: %v", err)
	}

	// Get current date
	now := time.Now()

	// Search for versions for the next 2 months
	project, _, err := client.Project.Get("MM")
	if err != nil {
		return fmt.Errorf("error searching Jira: %v", err)
	}

	if len(project.Versions) == 0 {
		fmt.Println("No upcoming releases found")
		return nil
	}

	fmt.Println("Upcoming Mattermost Releases:")
	fmt.Println("============================")

	for _, version := range project.Versions {
		if version.ReleaseDate == "" {
			continue
		}

		releaseDate, err := time.Parse("2006-01-02", version.ReleaseDate)
		if err != nil {
			continue
		}

		// Skip past releases
		if releaseDate.Before(now) {
			continue
		}

		// Only show releases in next 2 months
		if releaseDate.After(now.AddDate(0, 2, 0)) {
			continue
		}

		fmt.Printf("%s: %s\n", releaseDate.Format("Monday, January 2, 2006"), version.Name)
	}

	return nil
}
