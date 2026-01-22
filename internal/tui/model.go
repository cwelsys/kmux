package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/project"
	"github.com/cwel/kmux/internal/state"
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
	Type      ItemType
	Name      string
	Path      string // only for projects
	Host      string // "local" or SSH alias for sessions
	PaneCount int    // only for sessions
	Status    string // only for sessions: "active", "detached", "saved"
	CWD       string // for sessions
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
	state         *state.State
	cfg           *config.Config

	// Host loading state
	loadingHosts map[string]bool // hosts currently being queried
	hostErrors   map[string]error

	// Launch mode (layout selection modal)
	launchMode      bool
	launchLayouts   []string // available layouts, index 0 = "(none)"
	launchCursor    int      // selected layout index
	launchNameInput textinput.Model
	launchNameFocus bool // true = name field focused, false = layout list focused
	launchLayout    string
	launchName      string

	// Host selection for new sessions
	hostMode       bool
	hostList       []string // configured hosts + "local"
	hostCursor     int
	selectedHost   string // selected host for new session

	// Yazi result
	yaziPath string // path selected from yazi
}

// New creates a new TUI model.
func New(s *state.State, cfg *config.Config) Model {
	ti := textinput.New()
	ti.Placeholder = "filter..."
	ti.CharLimit = 50

	ri := textinput.New()
	ri.Placeholder = "new name..."
	ri.CharLimit = 50

	li := textinput.New()
	li.Placeholder = "session name..."
	li.CharLimit = 50

	// Build host list
	hostList := []string{"local"}
	if cfg != nil {
		hostList = append(hostList, cfg.HostNames()...)
	}

	return Model{
		filterInput:     ti,
		renameInput:     ri,
		launchNameInput: li,
		state:           s,
		cfg:             cfg,
		loadingHosts:    make(map[string]bool),
		hostErrors:      make(map[string]error),
		hostList:        hostList,
		selectedHost:    "local",
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return m.loadDataAsync
}

// loadDataAsync starts async loading of sessions from all hosts.
func (m Model) loadDataAsync() tea.Msg {
	// First, load local data synchronously for immediate display
	sessions, err := m.state.Sessions(true)
	if err != nil {
		return errMsg{fmt.Errorf("sessions: %w", err)}
	}

	var sessionItems []Item
	sessionNames := make(map[string]bool)
	for _, s := range sessions {
		sessionNames[s.Name] = true
		host := s.Host
		if host == "" {
			host = "local"
		}
		sessionItems = append(sessionItems, Item{
			Type:      ItemSession,
			Name:      s.Name,
			Host:      host,
			PaneCount: s.Panes,
			Status:    s.Status,
			CWD:       s.CWD,
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

	return dataLoadedMsg{sessions: sessionItems, projects: projectItems, host: "local"}
}

// startRemoteLoading kicks off background queries to remote hosts.
func (m Model) startRemoteLoading() tea.Cmd {
	hosts := m.state.ConfiguredHosts()
	if len(hosts) == 0 {
		return nil
	}

	// Return a batch of commands, one per host
	var cmds []tea.Cmd
	for _, host := range hosts {
		h := host // capture for closure
		cmds = append(cmds, func() tea.Msg {
			return hostLoadingMsg{host: h}
		})
	}

	return tea.Batch(cmds...)
}

// loadHostSessions loads sessions for a specific remote host.
func (m Model) loadHostSessions(host string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Query just this host
		zmxClient := m.state.ZmxClientForHost(host)
		zmxSessions, err := zmxClient.List()
		if err != nil {
			return hostLoadedMsg{host: host, err: err}
		}

		// Build session items from zmx sessions
		var items []Item
		for _, zmxName := range zmxSessions {
			// Parse session name from zmx name (format: session.tab.pane)
			parts := strings.Split(zmxName, ".")
			if len(parts) > 0 {
				sessName := parts[0]
				// Check if we already have this session
				found := false
				for i := range items {
					if items[i].Name == sessName {
						items[i].PaneCount++
						found = true
						break
					}
				}
				if !found {
					items = append(items, Item{
						Type:      ItemSession,
						Name:      sessName,
						Host:      host,
						PaneCount: 1,
						Status:    "detached", // Remote sessions without kitty windows are detached
					})
				}
			}
		}

		_ = ctx // context used for potential timeout
		return hostLoadedMsg{host: host, sessions: items}
	}
}

// Message types
type dataLoadedMsg struct {
	sessions []Item
	projects []Item
	host     string
}

type hostLoadingMsg struct {
	host string
}

type hostLoadedMsg struct {
	host     string
	sessions []Item
	err      error
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

// SelectedSessionHost returns the host of the currently selected session.
func (m Model) SelectedSessionHost() string {
	item := m.SelectedItem()
	if item == nil || item.Type != ItemSession {
		return "local"
	}
	if item.Host == "" {
		return "local"
	}
	return item.Host
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

// SelectedHost returns the host selected for new session creation.
func (m Model) SelectedHost() string {
	if m.selectedHost == "" {
		return "local"
	}
	return m.selectedHost
}

// BrowserPath returns the path selected from yazi, or empty if none.
func (m Model) BrowserPath() string {
	return m.yaziPath
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
		// Start loading remote hosts after local data is ready
		return m, m.startRemoteLoading()

	case hostLoadingMsg:
		m.loadingHosts[msg.host] = true
		return m, m.loadHostSessions(msg.host)

	case hostLoadedMsg:
		delete(m.loadingHosts, msg.host)
		if msg.err != nil {
			m.hostErrors[msg.host] = msg.err
		} else {
			// Add remote sessions
			m.sessions = append(m.sessions, msg.sessions...)
			m.rebuildItems()
		}
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case yaziFinishedMsg:
		if msg.err != nil {
			// Show error to user
			m.err = msg.err
			return m, nil
		}
		if msg.path == "" {
			// User cancelled - just return to TUI
			return m, nil
		}
		// Got a path from yazi - create session
		m.yaziPath = msg.path
		m.launchName = filepath.Base(msg.path)
		m.launchLayout = ""
		m.action = "create"
		m.quitting = true
		return m, tea.Quit

	case yaziRemoteFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if msg.path == "" {
			return m, nil
		}
		// Got a path from remote yazi
		m.yaziPath = msg.path
		m.launchName = filepath.Base(msg.path)
		m.launchLayout = ""
		m.selectedHost = msg.host
		m.action = "create"
		m.quitting = true
		return m, tea.Quit
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
		if m.confirmKill || m.confirmIgnore || m.showHelp || m.filterMode || m.renameMode || m.launchMode || m.hostMode {
			m.confirmKill = false
			m.confirmIgnore = false
			m.showHelp = false
			m.filterMode = false
			m.filterInput.Blur()
			m.renameMode = false
			m.renameInput.Blur()
			m.launchMode = false
			m.launchNameInput.Blur()
			m.hostMode = false
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit

	case "esc":
		if m.confirmKill || m.confirmIgnore || m.showHelp || m.filterMode || m.renameMode || m.launchMode || m.hostMode {
			m.confirmKill = false
			m.confirmIgnore = false
			m.showHelp = false
			m.filterMode = false
			m.filterInput.Blur()
			m.renameMode = false
			m.renameInput.Blur()
			m.launchMode = false
			m.launchNameInput.Blur()
			m.hostMode = false
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
		if !m.filterMode && !m.confirmKill && !m.confirmIgnore && !m.renameMode && !m.launchMode && !m.hostMode {
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

	if m.hostMode {
		return m.handleHostMode(msg)
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
				m.selectedHost = item.Host
			} else {
				// Project - create new session
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
		return m, m.loadDataAsync
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
	case "z":
		// Open yazi file browser (local)
		return m, m.openYazi()
	case "Z":
		// Open host selection for remote browsing
		if len(m.hostList) > 1 {
			m.hostMode = true
			m.hostCursor = 0
		} else {
			// No remote hosts configured, just open local yazi
			return m, m.openYazi()
		}
	}

	return m, nil
}

func (m Model) handleHostMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.hostMode = false
		return m, nil
	case "up", "k":
		if m.hostCursor > 0 {
			m.hostCursor--
		}
	case "down", "j":
		if m.hostCursor < len(m.hostList)-1 {
			m.hostCursor++
		}
	case "enter":
		selectedHost := m.hostList[m.hostCursor]
		m.hostMode = false
		if selectedHost == "local" {
			return m, m.openYazi()
		}
		// Open remote yazi
		return m, m.openYaziRemote(selectedHost)
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

// yaziFinishedMsg is sent when yazi exits
type yaziFinishedMsg struct {
	path string
	err  error
}

// yaziRemoteFinishedMsg is sent when remote yazi exits
type yaziRemoteFinishedMsg struct {
	host string
	path string
	err  error
}

// openYazi spawns yazi using tea.ExecProcess (takes over terminal)
func (m Model) openYazi() tea.Cmd {
	startPath := ""
	if m.cfg != nil {
		startPath = m.cfg.BrowserStartPath()
	}
	if startPath == "" {
		startPath, _ = os.UserHomeDir()
	}

	// Create temp file for yazi to write selection to
	tmpFile := "/tmp/kmux-yazi-choice"
	os.Remove(tmpFile)

	// Build yazi command - run through user's login shell to get proper PATH
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}
	shellCmd := "yazi --chooser-file=" + tmpFile + " " + startPath
	cmd := exec.Command(shell, "-l", "-c", shellCmd)

	// Use tea.ExecProcess to let yazi take over the terminal
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return yaziFinishedMsg{err: err}
		}

		// Check if yazi wrote a selection
		data, err := os.ReadFile(tmpFile)
		os.Remove(tmpFile)
		if err != nil {
			return yaziFinishedMsg{} // No selection (user cancelled)
		}

		path := strings.TrimSpace(string(data))
		return yaziFinishedMsg{path: path}
	})
}

// openYaziRemote spawns yazi over SSH to browse a remote host
func (m Model) openYaziRemote(host string) tea.Cmd {
	tmpFile := "/tmp/kmux-yazi-choice-" + host
	os.Remove(tmpFile)

	// Use kitten ssh to run yazi on remote, with chooser-file writing to a remote temp file
	// Then read the result back
	remoteCmd := "yazi --chooser-file=/tmp/kmux-yazi-choice && cat /tmp/kmux-yazi-choice 2>/dev/null || true"
	cmd := exec.Command("kitten", "ssh", host, "-t", remoteCmd)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return yaziRemoteFinishedMsg{host: host, err: err}
		}

		// The path was printed to stdout by the cat command
		// We need to capture it differently - kitten ssh may not work well with this
		// For now, let's use a simpler approach
		return yaziRemoteFinishedMsg{host: host, path: "", err: fmt.Errorf("remote yazi browsing requires manual path entry")}
	})
}

func (m Model) handleConfirmKill(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		session := m.SelectedSession()
		host := m.SelectedSessionHost()
		if session == "" {
			m.confirmKill = false
			return m, nil
		}

		// Optimistic update - remove from list immediately for snappy UI
		newSessions := make([]Item, 0, len(m.sessions)-1)
		for _, s := range m.sessions {
			if !(s.Name == session && s.Host == host) {
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
			// Get zmx client for the host
			zmxClient := m.state.ZmxClientForHost(host)

			// Kill zmx sessions
			zmxSessions, _ := m.state.SessionZmxSessionsForHost(session, host)
			for _, zmxName := range zmxSessions {
				zmxClient.Kill(zmxName)
			}

			// Only delete local save file
			if host == "local" {
				m.state.Store().DeleteSession(session)
			}
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
			if err := m.state.Store().RenameSession(m.SelectedSession(), newName); err == nil {
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
		return m, m.loadDataAsync
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

// LoadingHosts returns a list of hosts currently being loaded.
func (m Model) LoadingHosts() []string {
	var hosts []string
	for host := range m.loadingHosts {
		hosts = append(hosts, host)
	}
	return hosts
}

// HostErrors returns a map of hosts that failed to load.
func (m Model) HostErrors() map[string]error {
	return m.hostErrors
}
