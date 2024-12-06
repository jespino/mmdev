package widgets

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170"))

	infoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	// Subtle style for scroll indicators
	subtleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	// Status indicators
	statusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("86"))

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))
)
