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
