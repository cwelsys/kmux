package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/client"
	"github.com/cwel/kmux/internal/project"
	"github.com/cwel/kmux/internal/store"
	"github.com/sahilm/fuzzy"
)

// ItemType distinguishes sessions from projects.
type ItemType int

const (
	ItemSession ItemType = iota
	ItemProject
)

// Item represents either a session or a project in the unified list.
type Item struct {
	Type       ItemType
	Name       string
	Path       string // only for projects
	PaneCount  int    // only for sessions
	HasRunning bool   // only for sessions
	CWD        string // for sessions
	LastSeen   string // for sessions
}

// Model is the bubbletea model for the TUI.
type Model struct {
	sessions      []Item
	projects      []Item
	allItems      []Item // all items unfiltered
	items         []Item // filtered view (or all if no filter)
	cursor        int
	filterInput   textinput.Model
	filterMode    bool
	renameMode    bool
	renameInput   textinput.Model
	showHelp      bool
	confirmKill   bool
	confirmIgnore bool // confirm adding project to ignore list
	width         int
	height        int
	err           error
	quitting      bool
	action        string // "attach", "kill", or "create" - set when exiting to perform action
	client        *client.Client
	cfg           *config.Config

	// Launch mode (layout selection modal)
	launchMode      bool
	launchLayouts   []string // available layouts, index 0 = "(none)"
	launchCursor    int      // selected layout index
	launchNameInput textinput.Model
	launchNameFocus bool // true = name field focused, false = layout list focused
	launchLayout    string
	launchName      string
}

// New creates a new TUI model.
func New(c *client.Client, cfg *config.Config) Model {
	ti := textinput.New()
	ti.Placeholder = "filter..."
	ti.CharLimit = 50

	ri := textinput.New()
	ri.Placeholder = "new name..."
	ri.CharLimit = 50

	li := textinput.New()
	li.Placeholder = "session name..."
	li.CharLimit = 50

	return Model{
		filterInput:     ti,
		renameInput:     ri,
		launchNameInput: li,
		client:          c,
		cfg:             cfg,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return m.loadData
}

// loadData loads session data from daemon and scans for projects.
func (m Model) loadData() tea.Msg {
	sessions, err := m.client.Sessions(true) // TUI shows all sessions including restore points
	if err != nil {
		return errMsg{err}
	}

	var sessionItems []Item
	sessionNames := make(map[string]bool)
	for _, s := range sessions {
		sessionNames[s.Name] = true
		sessionItems = append(sessionItems, Item{
			Type:       ItemSession,
			Name:       s.Name,
			PaneCount:  s.Panes,
			HasRunning: s.Status == "attached" || s.Status == "detached",
			CWD:        s.CWD,
			LastSeen:   s.LastSeen,
		})
	}

	// Scan for projects
	var projectItems []Item
	if m.cfg != nil {
		scanner := project.NewScanner(m.cfg)
		projects := scanner.Scan()
		// Filter out projects that already have sessions
		projects = project.FilterExisting(projects, sessionNames)
		for _, p := range projects {
			projectItems = append(projectItems, Item{
				Type: ItemProject,
				Name: p.Name,
				Path: p.Path,
			})
		}
	}

	return dataLoadedMsg{sessions: sessionItems, projects: projectItems}
}

// Message types
type dataLoadedMsg struct {
	sessions []Item
	projects []Item
}
type errMsg struct{ err error }

// SelectedItem returns the currently selected item, or nil if none.
func (m Model) SelectedItem() *Item {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return nil
	}
	return &m.items[m.cursor]
}

// SelectedSession returns the currently selected session name, or empty if none.
// Only returns a name if the selected item is a session.
func (m Model) SelectedSession() string {
	item := m.SelectedItem()
	if item == nil || item.Type != ItemSession {
		return ""
	}
	return item.Name
}

// SelectedProject returns the currently selected project, or nil if not a project.
func (m Model) SelectedProject() *Item {
	item := m.SelectedItem()
	if item == nil || item.Type != ItemProject {
		return nil
	}
	return item
}

// Action returns the action to perform after quitting ("attach", "kill", or "").
func (m Model) Action() string {
	return m.action
}

// LaunchLayout returns the selected layout for session creation, or empty for none.
func (m Model) LaunchLayout() string {
	return m.launchLayout
}

// LaunchName returns the custom name for session creation, or empty for default.
func (m Model) LaunchName() string {
	return m.launchName
}

// rebuildItems creates the unified items list from sessions and projects.
func (m *Model) rebuildItems() {
	m.allItems = make([]Item, 0, len(m.sessions)+len(m.projects))
	m.allItems = append(m.allItems, m.sessions...)
	m.allItems = append(m.allItems, m.projects...)
	m.applyFilter()
}

// itemNames implements fuzzy.Source for fuzzy matching.
type itemNames []Item

func (s itemNames) String(i int) string { return s[i].Name }
func (s itemNames) Len() int            { return len(s) }

// applyFilter filters items based on current filter input.
func (m *Model) applyFilter() {
	query := m.filterInput.Value()
	if query == "" {
		m.items = m.allItems
		return
	}

	// Fuzzy match existing items
	matches := fuzzy.FindFrom(query, itemNames(m.allItems))
	m.items = make([]Item, len(matches))
	for i, match := range matches {
		m.items[i] = m.allItems[match.Index]
	}
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

	case dataLoadedMsg:
		m.sessions = msg.sessions
		m.projects = msg.projects
		m.rebuildItems()
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

	// Handle text input in rename mode
	if m.renameMode {
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys
	switch msg.String() {
	case "ctrl+c", "q":
		if m.confirmKill || m.confirmIgnore || m.showHelp || m.filterMode || m.renameMode || m.launchMode {
			m.confirmKill = false
			m.confirmIgnore = false
			m.showHelp = false
			m.filterMode = false
			m.filterInput.Blur()
			m.renameMode = false
			m.renameInput.Blur()
			m.launchMode = false
			m.launchNameInput.Blur()
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit

	case "esc":
		if m.confirmKill || m.confirmIgnore || m.showHelp || m.filterMode || m.renameMode || m.launchMode {
			m.confirmKill = false
			m.confirmIgnore = false
			m.showHelp = false
			m.filterMode = false
			m.filterInput.Blur()
			m.renameMode = false
			m.renameInput.Blur()
			m.launchMode = false
			m.launchNameInput.Blur()
			return m, nil
		}
		// If filter is active, clear it instead of quitting
		if m.filterInput.Value() != "" {
			m.filterInput.SetValue("")
			m.applyFilter()
			m.cursor = 0
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit

	case "?":
		if !m.filterMode && !m.confirmKill && !m.confirmIgnore && !m.renameMode && !m.launchMode {
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

	if m.confirmIgnore {
		return m.handleConfirmIgnore(msg)
	}

	if m.filterMode {
		return m.handleFilterMode(msg)
	}

	if m.renameMode {
		return m.handleRenameMode(msg)
	}

	if m.launchMode {
		return m.handleLaunchMode(msg)
	}

	// Normal mode navigation
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "enter":
		item := m.SelectedItem()
		if item != nil {
			if item.Type == ItemSession {
				m.action = "attach"
			} else {
				// Project or Zoxide - create new session
				m.action = "create"
			}
			m.quitting = true
			return m, tea.Quit
		}
	case "d":
		if m.SelectedSession() != "" {
			// Delete session
			m.confirmKill = true
		} else if m.SelectedProject() != nil {
			// Ignore project
			m.confirmIgnore = true
		}
	case "r":
		// Only allow rename on sessions
		if m.SelectedSession() != "" {
			m.renameMode = true
			m.renameInput.SetValue("")
			m.renameInput.Focus()
			return m, textinput.Blink
		}
	case "R":
		// Refresh - reload sessions and rescan projects
		return m, m.loadData
	case "/":
		m.filterMode = true
		m.filterInput.Focus()
		return m, textinput.Blink
	case "l":
		// Launch with options - only for projects
		if project := m.SelectedProject(); project != nil {
			m.launchMode = true
			m.launchCursor = 0
			m.launchNameFocus = false
			// Load available layouts
			layouts, _ := store.ListLayouts()
			m.launchLayouts = append([]string{"(none)"}, layouts...)
			// Pre-fill name with project name
			m.launchNameInput.SetValue(project.Name)
		}
	}

	return m, nil
}

func (m Model) handleLaunchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.launchMode = false
		m.launchNameInput.Blur()
		return m, nil
	case "tab":
		// Toggle focus between layout list and name field
		m.launchNameFocus = !m.launchNameFocus
		if m.launchNameFocus {
			m.launchNameInput.Focus()
			return m, textinput.Blink
		}
		m.launchNameInput.Blur()
		return m, nil
	case "up", "k":
		if !m.launchNameFocus && m.launchCursor > 0 {
			m.launchCursor--
		}
	case "down", "j":
		if !m.launchNameFocus && m.launchCursor < len(m.launchLayouts)-1 {
			m.launchCursor++
		}
	case "enter":
		// Confirm launch
		project := m.SelectedProject()
		if project == nil {
			m.launchMode = false
			return m, nil
		}

		// Set layout (empty string if "(none)" selected)
		if m.launchCursor > 0 {
			m.launchLayout = m.launchLayouts[m.launchCursor]
		} else {
			m.launchLayout = ""
		}

		// Set name (use input value, or project name if empty)
		name := m.launchNameInput.Value()
		if name == "" {
			name = project.Name
		}
		m.launchName = name

		m.launchMode = false
		m.action = "create"
		m.quitting = true
		return m, tea.Quit
	default:
		// Pass other keys to text input if name field is focused
		if m.launchNameFocus {
			var cmd tea.Cmd
			m.launchNameInput, cmd = m.launchNameInput.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) handleConfirmKill(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		session := m.SelectedSession()
		if session == "" {
			m.confirmKill = false
			return m, nil
		}

		// Optimistic update - remove from list immediately for snappy UI
		newSessions := make([]Item, 0, len(m.sessions)-1)
		for _, s := range m.sessions {
			if s.Name != session {
				newSessions = append(newSessions, s)
			}
		}
		m.sessions = newSessions
		m.rebuildItems()

		// Adjust cursor
		if m.cursor >= len(m.items) && m.cursor > 0 {
			m.cursor--
		}

		m.confirmKill = false

		// Kill in background, reload to sync state
		return m, func() tea.Msg {
			m.client.Kill(session)
			return nil // Silently sync - UI already updated
		}
	case "n", "N", "esc":
		m.confirmKill = false
	}
	return m, nil
}

func (m Model) handleConfirmIgnore(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		project := m.SelectedProject()
		if project == nil {
			m.confirmIgnore = false
			return m, nil
		}

		// Add to ignore list
		if err := config.AddIgnorePattern(project.Path); err == nil {
			// Optimistic update - remove from list immediately
			newProjects := make([]Item, 0, len(m.projects)-1)
			for _, p := range m.projects {
				if p.Path != project.Path {
					newProjects = append(newProjects, p)
				}
			}
			m.projects = newProjects
			m.rebuildItems()

			// Adjust cursor
			if m.cursor >= len(m.items) && m.cursor > 0 {
				m.cursor--
			}
		}

		m.confirmIgnore = false
		return m, nil
	case "n", "N", "esc":
		m.confirmIgnore = false
	}
	return m, nil
}

func (m Model) handleFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Exit filter mode, keep filter applied
		m.filterMode = false
		m.filterInput.Blur()
	case "esc":
		// Clear filter and exit
		m.filterMode = false
		m.filterInput.Blur()
		m.filterInput.SetValue("")
		m.applyFilter()
		m.cursor = 0
	default:
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		// Apply filter on each keystroke
		m.applyFilter()
		m.cursor = 0 // Reset cursor when filter changes
		return m, cmd
	}
	return m, nil
}

func (m Model) handleRenameMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		newName := m.renameInput.Value()
		if newName != "" && m.SelectedSession() != "" {
			if err := m.client.Rename(m.SelectedSession(), newName); err == nil {
				// Update the session name in both lists
				for i := range m.sessions {
					if m.sessions[i].Name == m.SelectedSession() {
						m.sessions[i].Name = newName
						break
					}
				}
				m.rebuildItems()
			}
		}
		m.renameMode = false
		m.renameInput.Blur()
		return m, m.loadData
	case "esc":
		m.renameMode = false
		m.renameInput.Blur()
	default:
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}
	return m, nil
}
