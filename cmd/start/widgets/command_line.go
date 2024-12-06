package widgets

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type CommandLine struct {
	input         textinput.Model
	suggestions   []string
	history       []string
	historyPos    int
	infoStyle     lipgloss.Style
	statusMessage string
}

func NewCommandLine() CommandLine {
	ti := textinput.New()
	ti.Placeholder = "Enter command..."
	ti.Focus()

	return CommandLine{
		input:      ti,
		history:    make([]string, 0),
		historyPos: -1,
		infoStyle:  infoStyle,
	}
}

func (c *CommandLine) View() string {
	statusBar := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render(c.statusMessage + " | ? for help | : for command | q to quit")

	return lipgloss.JoinVertical(lipgloss.Left,
		c.infoStyle.Render("Command (Enter to execute):"),
		c.input.View(),
		statusBar,
	)
}

func (c *CommandLine) SetStatusMessage(msg string) {
	c.statusMessage = msg
}

func (c *CommandLine) AddToHistory(cmd string) {
	c.history = append(c.history, cmd)
	c.historyPos = -1
}

func (c *CommandLine) Value() string {
	return c.input.Value()
}

func (c *CommandLine) SetValue(value string) {
	c.input.SetValue(value)
}

func (c *CommandLine) Focus() {
	c.input.Focus()
}

func (c *CommandLine) Blur() {
	c.input.Blur()
}

func (c *CommandLine) Focused() bool {
	return c.input.Focused()
}
