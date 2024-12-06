package start

import (
	"github.com/charmbracelet/bubbles/textinput"
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
	commandInput   textinput.Model
	ready          bool
	selectedPane   string
	commandMode    bool
}

func initialModel() model {
	commandInput := textinput.New()
	commandInput.Prompt = ": "
	return model{
		selectedPane: "server",
		commandMode:  false,
		commandInput: commandInput,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) runCommand(cmd string) (tea.Model, tea.Cmd) {
	// Handle command execution here
	if cmd == "q" || cmd == "quit" {
		return m, tea.Quit
	}
	return m, nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		serverCmd tea.Cmd
		clientCmd tea.Cmd
		cmdCmd    tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.commandMode {
			switch msg.String() {
			case "ctrl+c":
				m.commandMode = false
				m.commandInput.SetValue("")
				return m, nil
			case "enter":
				m.commandMode = false
				value := m.commandInput.Value()
				m.commandInput.SetValue("")
				return m.runCommand(value)
			case "esc":
				m.commandMode = false
				m.commandInput.SetValue("")
				return m, nil
			}
			m.commandInput, cmdCmd = m.commandInput.Update(msg)
		} else {
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "ctrl+c":
				return m, tea.Quit
			case ":":
				m.commandMode = true
				m.commandInput.SetValue("")
				m.commandInput.Focus()
				return m, nil
			case "tab":
				if m.selectedPane == "server" {
					m.selectedPane = "client"
				} else {
					m.selectedPane = "server"
				}
				return m, nil
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

	return m, tea.Batch(serverCmd, clientCmd, cmdCmd)
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var bottomBar string
	if m.commandMode {
		bottomBar = m.commandInput.View()
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
