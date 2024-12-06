package widgets

import (
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

type ClientPane struct {
	viewport    viewport.Model
	isSelected  bool
	isRunning   bool
	titleStyle  lipgloss.Style
	statusStyle lipgloss.Style
	errorStyle  lipgloss.Style
}

func NewClientPane() ClientPane {
	cv := viewport.New(0, 0)
	cv.Style = lipgloss.NewStyle().Margin(0, 0, 1, 0)

	return ClientPane{
		viewport:    cv,
		titleStyle:  titleStyle,
		statusStyle: statusStyle,
		errorStyle:  errorStyle,
	}
}

func (p *ClientPane) SetContent(content string) {
	p.viewport.SetContent(content)
}

func (p *ClientPane) SetSize(width, height int) {
	p.viewport.Width = width
	p.viewport.Height = height
}

func (p *ClientPane) SetSelected(selected bool) {
	p.isSelected = selected
}

func (p *ClientPane) SetRunning(running bool) {
	p.isRunning = running
}

func (p *ClientPane) View() string {
	var indicator string
	if p.viewport.ScrollPercent() < 1.0 {
		indicator = subtleStyle.Render("↓")
	}

	status := p.statusStyle.Render("●")
	if !p.isRunning {
		status = p.errorStyle.Render("○")
	}

	title := "Client Output: "
	if p.isSelected {
		title = "> " + title
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		p.titleStyle.Render(title)+status+" "+indicator,
		p.viewport.View(),
	)
}
