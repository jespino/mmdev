package widgets

const helpText = `
Keyboard shortcuts:
  q, ctrl+c    Quit
  ?, h         Toggle help
  :            Focus command input
  tab, esc     Toggle input focus
  ↑/↓          Scroll output
  ctrl+l       Clear output
  
Commands:
  clear        Clear all output
  help         Show this help
  restart      Restart both processes
  status       Show process status
Press any key to close help
`

type HelpWindow struct {
	visible bool
}

func NewHelpWindow() HelpWindow {
	return HelpWindow{}
}

func (h *HelpWindow) Toggle() {
	h.visible = !h.visible
}

func (h *HelpWindow) View() string {
	if !h.visible {
		return ""
	}
	return helpText
}
