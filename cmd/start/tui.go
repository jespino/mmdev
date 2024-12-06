package start

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170"))

	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
)

type model struct {
	leftViewport  viewport.Model
	rightViewport viewport.Model
	ready         bool
}

func initialModel() model {
	return model{}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		leftCmd  tea.Cmd
		rightCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "left", "h":
			m.leftViewport.SetContent(m.leftViewport.View() + "\nNew left content")
		case "right", "l":
			m.rightViewport.SetContent(m.rightViewport.View() + "\nNew right content")
		}

	case tea.WindowSizeMsg:
		if !m.ready {
			viewportHeight := msg.Height / 2

			m.leftViewport = viewport.New(msg.Width, viewportHeight)
			m.leftViewport.SetContent("Left viewport content\nUse arrow keys to navigate")
			
			m.rightViewport = viewport.New(msg.Width, viewportHeight)
			m.rightViewport.SetContent("Right viewport content\nUse arrow keys to navigate")

			m.ready = true
		}
	}

	m.leftViewport, leftCmd = m.leftViewport.Update(msg)
	m.rightViewport, rightCmd = m.rightViewport.Update(msg)

	return m, tea.Batch(leftCmd, rightCmd)
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	help := helpStyle.Render("↑/↓: scroll • q: quit • h/l: add content")
	
	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s",
		titleStyle.Render("Top View"),
		m.leftViewport.View(),
		titleStyle.Render("Bottom View"),
		m.rightViewport.View(),
		help,
	)
}

func StartTUI() error {
	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	return err
}
