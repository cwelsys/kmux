package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cwel/kmux/internal/daemon/client"
)

// SessionInfo holds display information about a session.
type SessionInfo struct {
	Name       string
	PaneCount  int
	HasRunning bool
}

// Model is the bubbletea model for the TUI.
type Model struct {
	sessions    []SessionInfo
	cursor      int
	filterInput textinput.Model
	filterMode  bool
	showHelp    bool
	confirmKill bool
	width       int
	height      int
	err         error
	quitting    bool
	action      string // "attach" or "kill" - set when exiting to perform action
	client      *client.Client
}

// New creates a new TUI model.
func New(c *client.Client) Model {
	ti := textinput.New()
	ti.Placeholder = "filter..."
	ti.CharLimit = 50

	return Model{
		filterInput: ti,
		client:      c,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return m.loadSessions
}

// loadSessions loads session data from daemon.
func (m Model) loadSessions() tea.Msg {
	sessions, err := m.client.Sessions(true) // TUI shows all sessions including restore points
	if err != nil {
		return errMsg{err}
	}

	var infos []SessionInfo
	for _, s := range sessions {
		infos = append(infos, SessionInfo{
			Name:       s.Name,
			PaneCount:  s.Panes,
			HasRunning: s.Status == "attached",
		})
	}

	return sessionsLoadedMsg{infos}
}

// Message types
type sessionsLoadedMsg struct{ sessions []SessionInfo }
type errMsg struct{ err error }

// SelectedSession returns the currently selected session name, or empty if none.
func (m Model) SelectedSession() string {
	if len(m.sessions) == 0 || m.cursor >= len(m.sessions) {
		return ""
	}
	return m.sessions[m.cursor].Name
}

// Action returns the action to perform after quitting ("attach", "kill", or "").
func (m Model) Action() string {
	return m.action
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case sessionsLoadedMsg:
		m.sessions = msg.sessions
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil
	}

	// Handle text input in filter mode
	if m.filterMode {
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys
	switch msg.String() {
	case "ctrl+c", "q":
		if m.confirmKill || m.showHelp || m.filterMode {
			m.confirmKill = false
			m.showHelp = false
			m.filterMode = false
			m.filterInput.Blur()
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit

	case "esc":
		if m.confirmKill || m.showHelp || m.filterMode {
			m.confirmKill = false
			m.showHelp = false
			m.filterMode = false
			m.filterInput.Blur()
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit

	case "?":
		if !m.filterMode && !m.confirmKill {
			m.showHelp = !m.showHelp
		}
		return m, nil
	}

	// Don't process other keys in overlay modes
	if m.showHelp {
		return m, nil
	}

	if m.confirmKill {
		return m.handleConfirmKill(msg)
	}

	if m.filterMode {
		return m.handleFilterMode(msg)
	}

	// Normal mode navigation
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.sessions)-1 {
			m.cursor++
		}
	case "enter":
		if len(m.sessions) > 0 {
			m.action = "attach"
			m.quitting = true
			return m, tea.Quit
		}
	case "d":
		if len(m.sessions) > 0 {
			m.confirmKill = true
		}
	case "r":
		// TODO: rename functionality
	case "/":
		m.filterMode = true
		m.filterInput.Focus()
		return m, textinput.Blink
	}

	return m, nil
}

func (m Model) handleConfirmKill(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.action = "kill"
		m.quitting = true
		return m, tea.Quit
	case "n", "N", "esc":
		m.confirmKill = false
	}
	return m, nil
}

func (m Model) handleFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.filterMode = false
		m.filterInput.Blur()
		// Apply filter - for now just exit filter mode
		// TODO: filter sessions list
	case "esc":
		m.filterMode = false
		m.filterInput.Blur()
		m.filterInput.SetValue("")
	default:
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		return m, cmd
	}
	return m, nil
}
