package start

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("241"))
	titleSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("170"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

type model struct {
	serverViewport viewport.Model
	clientViewport viewport.Model
	ready          bool
	selectedPane   string
	commandMode    bool
	commandInput   string
}

func initialModel() model {
	return model{
		selectedPane: "server",
		commandMode: false,
		commandInput: "",
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		serverCmd tea.Cmd
		clientCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			if m.commandMode {
				m.commandInput += "q"
				return m, nil
			}
			return m, tea.Quit
		case "ctrl+c":
			if m.commandMode {
				m.commandMode = false
				m.commandInput = ""
				return m, nil
			}
			return m, tea.Quit
		case ":":
			if !m.commandMode {
				m.commandMode = true
				m.commandInput = ""
				return m, nil
			}
			m.commandInput += ":"
			return m, nil
		case "enter":
			if m.commandMode {
				// Handle command execution here
				if m.commandInput == "q" || m.commandInput == "quit" {
					return m, tea.Quit
				}
				m.commandMode = false
				m.commandInput = ""
				return m, nil
			}
		case "backspace":
			if m.commandMode && len(m.commandInput) > 0 {
				m.commandInput = m.commandInput[:len(m.commandInput)-1]
				return m, nil
			}
		case "esc":
			if m.commandMode {
				m.commandMode = false
				m.commandInput = ""
				return m, nil
			}
		case "tab":
			if m.selectedPane == "server" {
				m.selectedPane = "client"
			} else {
				m.selectedPane = "server"
			}
			return m, nil
		case "a":
			if m.selectedPane == "server" {
				m.serverViewport.SetContent(m.serverViewport.View() + "\nNew server content")
			} else {
				m.clientViewport.SetContent(m.clientViewport.View() + "\nNew client content")
			}
		}

	case tea.WindowSizeMsg:
		if !m.ready {
			viewportWidth := msg.Width / 2

			m.serverViewport = viewport.New(viewportWidth, msg.Height-2)
			m.clientViewport = viewport.New(viewportWidth, msg.Height-2)
			m.ready = true
		}
	}

	m.serverViewport.GotoBottom()
	m.clientViewport.GotoBottom()

	if m.selectedPane == "server" {
		m.serverViewport, serverCmd = m.serverViewport.Update(msg)
	} else {
		m.clientViewport, clientCmd = m.clientViewport.Update(msg)
	}

	return m, tea.Batch(serverCmd, clientCmd)
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var bottomBar string
	if m.commandMode {
		bottomBar = ":" + m.commandInput
	} else {
		bottomBar = helpStyle.Render("↑/↓: scroll • q: quit • a: add content • tab: switch • :: command")
	}

	titleServer := titleStyle.Render("Server")
	titleClient := titleStyle.Render("Client")
	if m.selectedPane == "server" {
		titleServer = titleSelectedStyle.Render("Server")
	} else {
		titleClient = titleSelectedStyle.Render("Client")
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.JoinVertical(lipgloss.Left,
				titleServer,
				m.serverViewport.View(),
			),
			lipgloss.JoinVertical(lipgloss.Left,
				titleClient,
				m.clientViewport.View(),
			),
		),
		bottomBar,
	)
}

func StartTUI() error {
	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
	)

	_, err := p.Run()
	return err
}
