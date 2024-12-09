package dates

import (
	"fmt"
	"sort"
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

	fmt.Println("Upcoming Mattermost Release Timeline:")
	fmt.Println("=================================")

	// Calculate working days (excluding weekends)
	workingDaysBefore := func(date time.Time, days int) time.Time {
		result := date
		for days > 0 {
			result = result.AddDate(0, 0, -1)
			if result.Weekday() != time.Saturday && result.Weekday() != time.Sunday {
				days--
			}
		}
		return result
	}

	// Create a slice to store all dates
	type releaseDate struct {
		date    time.Time
		version string
		event   string
	}
	var dates []releaseDate

	for _, version := range project.Versions {
		if version.ReleaseDate == "" {
			continue
		}

		releaseDateDate, err := time.Parse("2006-01-02", version.ReleaseDate)
		if err != nil {
			continue
		}

		// Skip past releases
		if releaseDateDate.Before(now) {
			continue
		}

		// Only show releases in next 2 months
		if releaseDateDate.After(now.AddDate(0, 2, 0)) {
			continue
		}

		dates = append(dates, releaseDate{date: releaseDateDate, version: version.Name, event: "Self-Managed Release"})
		dates = append(dates, releaseDate{date: workingDaysBefore(releaseDateDate, 2), version: version.Name, event: "Cloud Dedicated Release"})
		dates = append(dates, releaseDate{date: workingDaysBefore(releaseDateDate, 3), version: version.Name, event: "Cloud Enterprise Release"})
		dates = append(dates, releaseDate{date: workingDaysBefore(releaseDateDate, 5), version: version.Name, event: "Cloud Professional"})
		dates = append(dates, releaseDate{date: workingDaysBefore(releaseDateDate, 6), version: version.Name, event: "Cloud Freemium"})
		dates = append(dates, releaseDate{date: workingDaysBefore(releaseDateDate, 7), version: version.Name, event: "Cloud Beta"})
		dates = append(dates, releaseDate{date: workingDaysBefore(releaseDateDate, 8), version: version.Name, event: "Release Approval"})
		dates = append(dates, releaseDate{date: workingDaysBefore(releaseDateDate, 10), version: version.Name, event: "Code Freeze"})
		dates = append(dates, releaseDate{date: workingDaysBefore(releaseDateDate, 18), version: version.Name, event: "Release Qualification"})
		dates = append(dates, releaseDate{date: workingDaysBefore(releaseDateDate, 19), version: version.Name, event: "Judgment Day"})
		dates = append(dates, releaseDate{date: workingDaysBefore(releaseDateDate, 24), version: version.Name, event: "Feature Complete"})
	}

	// Sort dates by date
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].date.Before(dates[j].date)
	})

	// Print sorted dates
	for _, d := range dates {
		fmt.Printf("%s: %-23s (%s)\n",
			d.date.Format("Monday, January 2, 2006"),
			d.event,
			d.version)
	}

	return nil
}
