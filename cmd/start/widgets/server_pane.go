package widgets

import (
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

type ServerPane struct {
	viewport    viewport.Model
	isSelected  bool
	isRunning   bool
	titleStyle  lipgloss.Style
	statusStyle lipgloss.Style
	errorStyle  lipgloss.Style
}

func NewServerPane() ServerPane {
	sv := viewport.New(0, 0)
	sv.Style = lipgloss.NewStyle().Margin(0, 0, 1, 0)

	return ServerPane{
		viewport:    sv,
		titleStyle:  titleStyle,
		statusStyle: statusStyle,
		errorStyle:  errorStyle,
	}
}

func (p *ServerPane) SetContent(content string) {
	p.viewport.SetContent(content)
}

func (p *ServerPane) SetSize(width, height int) {
	p.viewport.Width = width
	p.viewport.Height = height
}

func (p *ServerPane) SetSelected(selected bool) {
	p.isSelected = selected
}

func (p *ServerPane) SetRunning(running bool) {
	p.isRunning = running
}

func (p *ServerPane) View() string {
	var indicator string
	if p.viewport.ScrollPercent() < 1.0 {
		indicator = subtleStyle.Render("↓")
	}

	status := p.statusStyle.Render("●")
	if !p.isRunning {
		status = p.errorStyle.Render("○")
	}

	title := "Server Output: "
	if p.isSelected {
		title = "> " + title
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		p.titleStyle.Render(title)+status+" "+indicator,
		p.viewport.View(),
	)
}
