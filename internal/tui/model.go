package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cwel/kmux/internal/model"
	"github.com/cwel/kmux/internal/store"
	"github.com/cwel/kmux/internal/zmx"
)

// SessionInfo holds display information about a session.
type SessionInfo struct {
	Name       string
	SavedAt    time.Time
	PaneCount  int
	HasRunning bool
	Tabs       []model.Tab
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
	store       *store.Store
	zmx         *zmx.Client
}

// New creates a new TUI model.
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "filter..."
	ti.CharLimit = 50

	return Model{
		filterInput: ti,
		store:       store.DefaultStore(),
		zmx:         zmx.NewClient(),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return m.loadSessions
}

// loadSessions loads session data from store and zmx.
func (m Model) loadSessions() tea.Msg {
	saved, err := m.store.ListSessions()
	if err != nil {
		return errMsg{err}
	}

	running, _ := m.zmx.List() // ignore error, just means no running sessions
	runningSet := make(map[string]bool)
	for _, r := range running {
		// Extract session name from "session.tab.window"
		name := extractSessionName(r)
		if name != "" {
			runningSet[name] = true
		}
	}

	var sessions []SessionInfo
	for _, name := range saved {
		sess, err := m.store.LoadSession(name)
		if err != nil {
			continue
		}

		panes := 0
		for _, tab := range sess.Tabs {
			panes += len(tab.Windows)
		}

		sessions = append(sessions, SessionInfo{
			Name:       name,
			SavedAt:    sess.SavedAt,
			PaneCount:  panes,
			HasRunning: runningSet[name],
			Tabs:       sess.Tabs,
		})
	}

	return sessionsLoadedMsg{sessions}
}

// extractSessionName gets the session name from a zmx session name like "project.0.0"
func extractSessionName(zmxName string) string {
	for i, c := range zmxName {
		if c == '.' {
			return zmxName[:i]
		}
	}
	return zmxName
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

// View implements tea.Model.
func (m Model) View() string {
	// TODO: implement in next task
	return ""
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
