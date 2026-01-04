package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/protocol"
	"github.com/cwel/kmux/internal/kitty"
	"github.com/cwel/kmux/internal/manager"
	"github.com/cwel/kmux/internal/model"
	"github.com/cwel/kmux/internal/store"
	"github.com/cwel/kmux/internal/zmx"
)

type SessionState struct {
	Name      string
	Status    string // "attached", "detached"
	Panes     int    // number of panes in session
	WindowIDs []int
	ZmxAlive  bool
	LastSeen  time.Time
}

type DaemonState struct {
	Sessions       map[string]*SessionState
	Mappings       map[int]string // kitty_window_id -> zmx_name (AUTHORITATIVE)
	WindowSessions map[int]string // kitty_window_id -> session_name (AUTHORITATIVE)
	ZmxOwnership   map[string]string // zmx_name -> session_name (AUTHORITATIVE for rename)
	KittyState     kitty.KittyState
	ZmxSessions    []string
	LastPoll       time.Time
	LastAutoSave   time.Time
}

// Server is the kmux daemon server.
type Server struct {
	socketPath  string
	dataDir     string
	kittySocket string // stored from first request, used for polling
	listener    net.Listener
	mu          sync.Mutex
	done        chan struct{}

	// Internal clients - daemon owns these
	store *store.Store
	kitty *kitty.Client // default client, updated when kittySocket changes
	zmx   *zmx.Client
	cfg   *config.Config
	state *DaemonState
}

// New creates a new daemon server.
func New(socketPath, dataDir string) *Server {
	cfg, err := config.LoadConfig()
	if err != nil {
		// Fall back to defaults on error
		cfg = config.DefaultConfig()
	}

	// Use config socket if specified, otherwise default
	var kittyClient *kitty.Client
	if cfg.Kitty.Socket != "" {
		kittyClient = kitty.NewClientWithSocket(cfg.Kitty.Socket)
	} else {
		kittyClient = kitty.NewClient()
	}

	return &Server{
		socketPath: socketPath,
		dataDir:    dataDir,
		done:       make(chan struct{}),
		store:      store.New(dataDir),
		kitty:      kittyClient,
		zmx:        zmx.NewClient(),
		cfg:        cfg,
		state: &DaemonState{
			Sessions:       make(map[string]*SessionState),
			Mappings:       make(map[int]string),
			WindowSessions: make(map[int]string),
			ZmxOwnership:   make(map[string]string),
		},
	}
}

// Start starts the daemon server.
func (s *Server) Start() error {
	// Log config on startup
	log.Printf("kmux daemon starting")
	log.Printf("  config: watch_interval=%ds, auto_save_interval=%ds",
		s.cfg.Daemon.WatchInterval, s.cfg.Daemon.AutoSaveInterval)
	if s.cfg.Kitty.Socket != "" {
		log.Printf("  config: kitty_socket=%s", s.cfg.Kitty.Socket)
	}

	// Create socket directory
	dir := filepath.Dir(s.socketPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create socket dir: %w", err)
	}

	// Remove stale socket
	os.Remove(s.socketPath)

	// Initialize state from disk + zmx
	s.initState()

	// Listen
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	go s.runPollingLoop()

	// Accept loop
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return nil // clean shutdown
			default:
				return fmt.Errorf("accept: %w", err)
			}
		}
		go s.handleConn(conn)
	}
}

// Stop stops the daemon server.
func (s *Server) Stop() {
	close(s.done)

	s.mu.Lock()
	if s.listener != nil {
		s.listener.Close()
	}
	s.mu.Unlock()

	os.Remove(s.socketPath)
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	var req protocol.Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		resp := protocol.ErrorResponse(fmt.Sprintf("decode: %v", err))
		json.NewEncoder(conn).Encode(resp)
		return
	}

	resp := s.handleRequest(req)
	json.NewEncoder(conn).Encode(resp)
}

func (s *Server) handleRequest(req protocol.Request) protocol.Response {
	// Create kitty client for this request using the socket from the request
	k := s.kittyForRequest(req)

	switch req.Method {
	case protocol.MethodPing:
		return protocol.SuccessResponse("pong")
	case protocol.MethodSessions:
		var params protocol.SessionsParams
		if len(req.Params) > 0 {
			json.Unmarshal(req.Params, &params)
		}
		return s.handleSessions(k, params)
	case protocol.MethodAttach:
		var params protocol.AttachParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return protocol.ErrorResponse(fmt.Sprintf("invalid params: %v", err))
		}
		return s.handleAttach(k, params)
	case protocol.MethodDetach:
		var params protocol.DetachParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return protocol.ErrorResponse(fmt.Sprintf("invalid params: %v", err))
		}
		return s.handleDetach(k, params)
	case protocol.MethodKill:
		var params protocol.KillParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return protocol.ErrorResponse(fmt.Sprintf("invalid params: %v", err))
		}
		return s.handleKill(k, params)
	case protocol.MethodShutdown:
		go func() {
			s.Stop()
		}()
		return protocol.SuccessResponse("shutting down")
	case protocol.MethodSplit:
		var params protocol.SplitParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return protocol.ErrorResponse(fmt.Sprintf("invalid params: %v", err))
		}
		return s.handleSplit(k, params)
	case protocol.MethodResolve:
		var params protocol.ResolveParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return protocol.ErrorResponse(fmt.Sprintf("invalid params: %v", err))
		}
		return s.handleResolve(params)
	case protocol.MethodRename:
		var params protocol.RenameParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return protocol.ErrorResponse(fmt.Sprintf("invalid params: %v", err))
		}
		return s.handleRename(params)
	case protocol.MethodWindowClosed:
		var params protocol.WindowClosedParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return protocol.ErrorResponse(fmt.Sprintf("invalid params: %v", err))
		}
		return s.handleWindowClosed(params)
	case protocol.MethodCloseFocused:
		return s.handleCloseFocused(k)
	case protocol.MethodCloseTab:
		return s.handleCloseTab(k)
	default:
		return protocol.ErrorResponse(fmt.Sprintf("unknown method: %s", req.Method))
	}
}

// kittyForRequest creates a kitty client for the request's socket and stores it for polling.
// If the request doesn't provide a socket, uses the discovered socket.
func (s *Server) kittyForRequest(req protocol.Request) *kitty.Client {
	if req.KittySocket != "" {
		// Extract path from "unix:/path/to/socket" format
		socket := req.KittySocket
		if len(socket) > 5 && socket[:5] == "unix:" {
			socket = socket[5:]
		}

		// Store socket for polling/auto-save if we don't have one yet
		s.mu.Lock()
		if s.kittySocket == "" || s.kittySocket != socket {
			s.kittySocket = socket
			s.kitty = kitty.NewClientWithSocket(socket)
		}
		s.mu.Unlock()

		return kitty.NewClientWithSocket(socket)
	}
	// No socket in request - use discovered socket
	return s.ensureKittyClient()
}

// ensureKittyClient returns a working kitty client, discovering the socket if needed.
// Called every poll cycle to handle kitty restarts (new PID = new socket).
func (s *Server) ensureKittyClient() *kitty.Client {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If we have a socket, use it
	if s.kittySocket != "" && s.kitty != nil {
		return s.kitty
	}

	// Discover kitty socket by looking for /tmp/mykitty-*
	matches, err := filepath.Glob("/tmp/mykitty-*")
	if err != nil || len(matches) == 0 {
		return nil
	}

	// Find the first valid socket
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSocket != 0 {
			s.kittySocket = m
			s.kitty = kitty.NewClientWithSocket(m)
			return s.kitty
		}
	}

	return nil
}

// initState loads persisted daemon state and verifies against reality.
// The persisted state is AUTHORITATIVE - zmx/kitty are queried for verification only.
func (s *Server) initState() {
	// Load persisted state first
	persisted, err := s.loadState()
	if err != nil {
		log.Printf("[init] WARNING: failed to load persisted state: %v", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Restore persisted mappings
	if persisted != nil {
		log.Printf("[init] loaded persisted state from %v", persisted.LastSaved)
		for k, v := range persisted.Mappings {
			s.state.Mappings[k] = v
		}
		for k, v := range persisted.WindowSessions {
			s.state.WindowSessions[k] = v
		}
		for k, v := range persisted.ZmxOwnership {
			s.state.ZmxOwnership[k] = v
		}
	}

	// Query zmx for verification
	zmxSessions, _ := s.zmx.List()
	zmxSet := make(map[string]bool)
	for _, z := range zmxSessions {
		zmxSet[z] = true
	}

	// Build session states from persisted ownership
	sessionPanes := make(map[string]int)
	for zmxName, sessName := range s.state.ZmxOwnership {
		if zmxSet[zmxName] {
			sessionPanes[sessName]++
		} else {
			// zmx session no longer exists - log discrepancy
			log.Printf("[init] DISCREPANCY: zmx session %q (owned by %q) no longer exists", zmxName, sessName)
			delete(s.state.ZmxOwnership, zmxName)
		}
	}

	// Clean up mappings for windows that may no longer exist
	// (We can't verify kitty windows at init - no socket yet)
	// Polling will clean these up later

	// Adopt orphan zmx sessions that follow our naming convention
	// This handles daemon crashes or state loss - zmx is the source of truth
	for _, zmxName := range zmxSessions {
		if _, owned := s.state.ZmxOwnership[zmxName]; owned {
			continue // already tracked
		}
		sessName := model.ParseZmxSessionName(zmxName)
		if sessName == "" {
			continue // not our naming convention, ignore
		}
		log.Printf("[init] adopting orphan zmx session %q -> session %q", zmxName, sessName)
		s.state.ZmxOwnership[zmxName] = sessName
		sessionPanes[sessName]++
	}

	// Create session entries from ownership
	for name, panes := range sessionPanes {
		s.state.Sessions[name] = &SessionState{
			Name:     name,
			Status:   "detached", // will be updated by first poll
			Panes:    panes,
			ZmxAlive: true,
			LastSeen: time.Now(),
		}
	}

	s.state.ZmxSessions = zmxSessions
	s.state.LastPoll = time.Now()

	log.Printf("[init] initialized with %d sessions from persisted state", len(s.state.Sessions))
}

// layoutToSession converts a layout template to a session.
func layoutToSession(layout *config.Layout, name, cwd string) *model.Session {
	session := &model.Session{
		Name:    name,
		Host:    "local",
		SavedAt: time.Now(),
	}

	for _, ltab := range layout.Tabs {
		tab := model.Tab{
			Title:  ltab.Title,
			Layout: ltab.Layout,
		}

		// Create windows from panes
		for _, pane := range ltab.Panes {
			window := model.Window{
				CWD:     cwd,
				Command: pane,
			}
			tab.Windows = append(tab.Windows, window)
		}

		session.Tabs = append(session.Tabs, tab)
	}

	return session
}

func (s *Server) handleSessions(k *kitty.Client, params protocol.SessionsParams) protocol.Response {
	// Build session list from daemon's authoritative state
	s.mu.Lock()
	var sessions []protocol.SessionInfo
	for name, sess := range s.state.Sessions {
		info := protocol.SessionInfo{
			Name:           name,
			Status:         sess.Status,
			Panes:          sess.Panes,
			IsRestorePoint: false,
		}

		// Get CWD from first window
		if len(sess.WindowIDs) > 0 {
			if win := s.findWindowInState(sess.WindowIDs[0]); win != nil {
				info.CWD = win.CWD
			}
		}

		// Format last seen
		if !sess.LastSeen.IsZero() {
			info.LastSeen = formatLastSeen(sess.LastSeen)
		}

		sessions = append(sessions, info)
	}
	s.mu.Unlock()

	// Add restore points if requested (save files for sessions not currently running)
	if params.IncludeRestorePoints {
		// Build set of running session names
		s.mu.Lock()
		runningSessions := make(map[string]bool)
		for name := range s.state.Sessions {
			runningSessions[name] = true
		}
		s.mu.Unlock()

		saved, _ := s.store.ListSessions()
		for _, name := range saved {
			if runningSessions[name] {
				continue // already listed as running
			}

			panes := 1
			cwd := ""
			if sess, err := s.store.LoadSession(name); err == nil {
				panes = 0
				for _, tab := range sess.Tabs {
					panes += len(tab.Windows)
					// Get CWD from first window of first tab
					if cwd == "" && len(tab.Windows) > 0 {
						cwd = tab.Windows[0].CWD
					}
				}
			}

			sessions = append(sessions, protocol.SessionInfo{
				Name:           name,
				Status:         "saved",
				Panes:          panes,
				IsRestorePoint: true,
				CWD:            cwd,
			})
		}
	}

	return protocol.SuccessResponse(sessions)
}

// findWindowInState looks up a window by ID in the cached kitty state.
func (s *Server) findWindowInState(windowID int) *kitty.Window {
	for _, osWin := range s.state.KittyState {
		for _, tab := range osWin.Tabs {
			for i := range tab.Windows {
				if tab.Windows[i].ID == windowID {
					return &tab.Windows[i]
				}
			}
		}
	}
	return nil
}

// formatLastSeen returns a human-readable relative time.
func formatLastSeen(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func (s *Server) handleAttach(k *kitty.Client, params protocol.AttachParams) protocol.Response {
	name := params.Name

	if err := store.ValidateSessionName(name); err != nil {
		return protocol.ErrorResponse(err.Error())
	}

	cwd := params.CWD
	if cwd == "" {
		cwd = "/"
	}

	var session *model.Session

	// Check if session is already running (in daemon's authoritative state)
	s.mu.Lock()
	existingSession, sessionRunning := s.state.Sessions[name]
	s.mu.Unlock()

	if sessionRunning && existingSession.ZmxAlive {
		// Session is running - reattach (ignore layout)
		session, _ = s.store.LoadSession(name)
		if session == nil {
			// Create minimal session for running zmx
			session = &model.Session{
				Name:    name,
				Host:    "local",
				SavedAt: time.Now(),
				Tabs: []model.Tab{
					{Title: name, Layout: "splits", Windows: []model.Window{{CWD: cwd}}},
				},
			}
		}
	} else if params.Layout != "" {
		// New session with layout template
		layout, err := store.LoadLayout(params.Layout)
		if err != nil {
			return protocol.ErrorResponse(fmt.Sprintf("load layout: %v", err))
		}
		session = layoutToSession(layout, name, cwd)
	} else {
		// Try to load restore point, or create fresh
		session, _ = s.store.LoadSession(name)
		if session == nil {
			session = &model.Session{
				Name:    name,
				Host:    "local",
				SavedAt: time.Now(),
				Tabs: []model.Tab{
					{Title: name, Layout: "splits", Windows: []model.Window{{CWD: cwd}}},
				},
			}
		}
	}

	// Clear ZmxSessions before rebuilding
	session.ZmxSessions = nil

	// Create windows in kitty using RestoreTab
	var firstWindowID int
	var allCreations []manager.WindowCreate
	for tabIdx, tab := range session.Tabs {
		creations, windowID, err := manager.RestoreTab(k, session, tabIdx, tab)
		if err != nil {
			return protocol.ErrorResponse(fmt.Sprintf("restore tab: %v", err))
		}
		allCreations = append(allCreations, creations...)
		if tabIdx == 0 && windowID > 0 {
			firstWindowID = windowID
		}
	}

	// Focus first window
	if firstWindowID > 0 {
		k.FocusWindow(firstWindowID)
	}

	// RECORD all mappings
	s.mu.Lock()
	for _, c := range allCreations {
		s.state.Mappings[c.KittyWindowID] = c.ZmxName
		s.state.WindowSessions[c.KittyWindowID] = name
		s.state.ZmxOwnership[c.ZmxName] = name // zmx -> session for rename support
	}
	// Update session state
	panes := 0
	for _, tab := range session.Tabs {
		panes += len(tab.Windows)
	}
	s.state.Sessions[name] = &SessionState{
		Name:     name,
		Status:   "attached",
		Panes:    panes,
		ZmxAlive: true,
		LastSeen: time.Now(),
	}
	s.mu.Unlock()

	// Persist daemon state (authoritative mappings)
	if err := s.saveState(); err != nil {
		log.Printf("[attach] WARNING: failed to persist state: %v", err)
	}

	// NOTE: We do NOT save the session restore point here.
	// Saving on attach would overwrite the user's saved layout.
	// Restore points are created on detach and periodic auto-save only.

	return protocol.SuccessResponse(protocol.AttachResult{
		Success: true,
		Message: fmt.Sprintf("Attached to session: %s", name),
	})
}

func (s *Server) handleDetach(k *kitty.Client, params protocol.DetachParams) protocol.Response {
	name := params.Name

	if err := store.ValidateSessionName(name); err != nil {
		return protocol.ErrorResponse(err.Error())
	}

	// Get current kitty state
	state, err := k.GetState()
	if err != nil {
		return protocol.ErrorResponse(fmt.Sprintf("get kitty state: %v", err))
	}

	// Derive session from current state using mappings
	s.mu.Lock()
	mappings := s.state.Mappings
	windowSessions := s.state.WindowSessions
	s.mu.Unlock()

	session := manager.DeriveSession(name, state, mappings, windowSessions)

	// Save session
	if err := s.store.SaveSession(session); err != nil {
		return protocol.ErrorResponse(fmt.Sprintf("save session: %v", err))
	}

	// Close windows belonging to this session
	if len(state) > 0 {
		for _, tab := range state[0].Tabs {
			for _, win := range tab.Windows {
				if s.state.WindowSessions[win.ID] == name {
					k.CloseWindow(win.ID)
				}
			}
		}
	}

	// Update internal state
	s.mu.Lock()
	if sess, ok := s.state.Sessions[name]; ok {
		sess.Status = "detached"
		sess.WindowIDs = nil
		sess.LastSeen = time.Now()
	}
	// Clear window mappings for closed windows
	for _, tab := range state[0].Tabs {
		for _, win := range tab.Windows {
			if s.state.WindowSessions[win.ID] == name {
				delete(s.state.Mappings, win.ID)
				delete(s.state.WindowSessions, win.ID)
			}
		}
	}
	s.mu.Unlock()

	// Persist daemon state
	if err := s.saveState(); err != nil {
		log.Printf("[detach] WARNING: failed to persist state: %v", err)
	}

	return protocol.SuccessResponse(protocol.AttachResult{
		Success: true,
		Message: fmt.Sprintf("Detached from session: %s", name),
	})
}

func (s *Server) handleKill(k *kitty.Client, params protocol.KillParams) protocol.Response {
	name := params.Name

	if err := store.ValidateSessionName(name); err != nil {
		return protocol.ErrorResponse(err.Error())
	}

	// Kill all zmx sessions that belong to this session (from authoritative ownership map)
	s.mu.Lock()
	var zmxToKill []string
	for zmxName, sessName := range s.state.ZmxOwnership {
		if sessName == name {
			zmxToKill = append(zmxToKill, zmxName)
		}
	}
	s.mu.Unlock()

	for _, zmxName := range zmxToKill {
		log.Printf("[kill] killing zmx session %s", zmxName)
		s.zmx.Kill(zmxName)
	}

	// Clean up ZmxOwnership entries for killed zmx sessions
	s.mu.Lock()
	for _, zmxName := range zmxToKill {
		delete(s.state.ZmxOwnership, zmxName)
	}
	s.mu.Unlock()

	// Close any kitty windows for this session
	state, _ := k.GetState()
	if len(state) > 0 {
		for _, tab := range state[0].Tabs {
			for _, win := range tab.Windows {
				if s.state.WindowSessions[win.ID] == name {
					k.CloseWindow(win.ID)
				}
			}
		}
	}

	// Delete saved session
	s.store.DeleteSession(name)

	// Remove from internal state
	s.mu.Lock()
	delete(s.state.Sessions, name)
	// Clean up window mappings for this session
	for windowID, sessName := range s.state.WindowSessions {
		if sessName == name {
			delete(s.state.Mappings, windowID)
			delete(s.state.WindowSessions, windowID)
		}
	}
	s.mu.Unlock()

	// Persist daemon state
	if err := s.saveState(); err != nil {
		log.Printf("[kill] WARNING: failed to persist state: %v", err)
	}

	return protocol.SuccessResponse(protocol.AttachResult{
		Success: true,
		Message: fmt.Sprintf("Killed session: %s", name),
	})
}

func (s *Server) handleSplit(k *kitty.Client, params protocol.SplitParams) protocol.Response {
	if k == nil {
		return protocol.ErrorResponse("no kitty connection available")
	}

	sessionName := params.Session // empty = native split

	// Validate direction
	location := ""
	switch params.Direction {
	case "vertical", "v":
		location = "vsplit"
	case "horizontal", "h":
		location = "hsplit"
	default:
		return protocol.ErrorResponse(fmt.Sprintf("invalid direction: %s (use 'vertical' or 'horizontal')", params.Direction))
	}

	// CWD - use provided or default to home
	cwd := params.CWD
	if cwd == "" {
		cwd, _ = os.UserHomeDir()
	}

	// If no session, create a native kitty split (no zmx)
	if sessionName == "" {
		opts := kitty.LaunchOpts{
			Type:     "window",
			Location: location,
			CWD:      cwd,
		}

		windowID, err := k.Launch(opts)
		if err != nil {
			return protocol.ErrorResponse(fmt.Sprintf("launch split: %v", err))
		}

		return protocol.SuccessResponse(protocol.SplitResult{
			Success:  true,
			WindowID: windowID,
			Message:  fmt.Sprintf("Created native %s split", params.Direction),
		})
	}

	// In a kmux session - create zmx-backed split
	state, err := k.GetState()
	if err != nil {
		return protocol.ErrorResponse(fmt.Sprintf("get kitty state: %v", err))
	}

	if len(state) == 0 {
		return protocol.ErrorResponse("no kitty windows found")
	}

	// Find windows for this session, grouped by kitty tab
	// We need session-relative tab index (not kitty tab index)
	type tabInfo struct {
		kittyTabID int
		windowIDs  []int
	}
	var sessionTabs []tabInfo

	for _, osWin := range state {
		for _, tab := range osWin.Tabs {
			var windowsInTab []int
			for _, win := range tab.Windows {
				if s.state.WindowSessions[win.ID] == sessionName {
					windowsInTab = append(windowsInTab, win.ID)
				}
			}
			if len(windowsInTab) > 0 {
				sessionTabs = append(sessionTabs, tabInfo{
					kittyTabID: tab.ID,
					windowIDs:  windowsInTab,
				})
			}
		}
	}

	if len(sessionTabs) == 0 {
		return protocol.ErrorResponse(fmt.Sprintf("no windows found for session: %s", sessionName))
	}

	// For now, assume single-tab sessions (tab index = 0)
	// The new window will be at index = current window count in first tab
	sessionTabIdx := 0
	windowIdx := len(sessionTabs[0].windowIDs)

	// Build zmx session name: {session}.{session_tab_idx}.{window_idx}
	zmxName := fmt.Sprintf("%s.%d.%d", sessionName, sessionTabIdx, windowIdx)
	zmxCmd := zmx.AttachCmd(zmxName, sessionName)

	// Launch the split window with zmx
	opts := kitty.LaunchOpts{
		Type:     "window",
		Location: location,
		CWD:      cwd,
		Cmd:      zmxCmd,
		Env:      nil,
	}

	windowID, err := k.Launch(opts)
	if err != nil {
		return protocol.ErrorResponse(fmt.Sprintf("launch split: %v", err))
	}

	// RECORD the mapping - this is the authoritative source
	s.mu.Lock()
	s.state.Mappings[windowID] = zmxName
	s.state.WindowSessions[windowID] = sessionName
	s.state.ZmxOwnership[zmxName] = sessionName // zmx -> session for rename support
	if sess, ok := s.state.Sessions[sessionName]; ok {
		sess.Panes++
		sess.LastSeen = time.Now()
	}
	s.mu.Unlock()

	// Persist daemon state
	if err := s.saveState(); err != nil {
		log.Printf("[split] WARNING: failed to persist state: %v", err)
	}

	return protocol.SuccessResponse(protocol.SplitResult{
		Success:  true,
		WindowID: windowID,
		Message:  fmt.Sprintf("Created %s split in session %s", params.Direction, sessionName),
	})
}

func (s *Server) handleResolve(params protocol.ResolveParams) protocol.Response {
	s.mu.Lock()
	session := s.state.WindowSessions[params.WindowID]
	zmxName := s.state.Mappings[params.WindowID]
	s.mu.Unlock()

	return protocol.SuccessResponse(protocol.ResolveResult{
		Session: session,
		ZmxName: zmxName,
	})
}

func (s *Server) handleRename(params protocol.RenameParams) protocol.Response {
	oldName := params.OldName
	newName := params.NewName

	// Validate names
	if err := store.ValidateSessionName(oldName); err != nil {
		return protocol.ErrorResponse(fmt.Sprintf("invalid old name: %v", err))
	}
	if err := store.ValidateSessionName(newName); err != nil {
		return protocol.ErrorResponse(fmt.Sprintf("invalid new name: %v", err))
	}

	// Check old exists
	s.mu.Lock()
	oldSession, exists := s.state.Sessions[oldName]
	if !exists {
		s.mu.Unlock()
		return protocol.ErrorResponse(fmt.Sprintf("session not found: %s", oldName))
	}

	// Check new doesn't exist
	if _, exists := s.state.Sessions[newName]; exists {
		s.mu.Unlock()
		return protocol.ErrorResponse(fmt.Sprintf("session already exists: %s", newName))
	}

	// Update in-memory state
	oldSession.Name = newName
	s.state.Sessions[newName] = oldSession
	delete(s.state.Sessions, oldName)

	// Update WindowSessions mappings
	for windowID, sessName := range s.state.WindowSessions {
		if sessName == oldName {
			s.state.WindowSessions[windowID] = newName
		}
	}

	// Update ZmxOwnership mappings
	for zmxName, sessName := range s.state.ZmxOwnership {
		if sessName == oldName {
			s.state.ZmxOwnership[zmxName] = newName
		}
	}
	s.mu.Unlock()

	// Rename save file
	if err := s.store.RenameSession(oldName, newName); err != nil {
		// Non-fatal - session might not have a save file yet
	}

	// Persist daemon state
	if err := s.saveState(); err != nil {
		log.Printf("[rename] WARNING: failed to persist state: %v", err)
	}

	return protocol.SuccessResponse(protocol.RenameResult{
		Success: true,
		Message: fmt.Sprintf("Renamed session: %s -> %s", oldName, newName),
	})
}

func (s *Server) handleWindowClosed(params protocol.WindowClosedParams) protocol.Response {
	log.Printf("[event] window closed: windowID=%d zmxName=%s session=%s",
		params.WindowID, params.ZmxName, params.Session)

	s.mu.Lock()

	// Remove from mappings
	delete(s.state.Mappings, params.WindowID)
	delete(s.state.WindowSessions, params.WindowID)

	// Update session state
	if sess, exists := s.state.Sessions[params.Session]; exists {
		// Count remaining windows for this session
		windowCount := 0
		for _, sessName := range s.state.WindowSessions {
			if sessName == params.Session {
				windowCount++
			}
		}

		if windowCount == 0 {
			// Check if zmx sessions are still running using ownership map
			zmxAlive := false
			panes := 0
			for zmxName, sessName := range s.state.ZmxOwnership {
				if sessName == params.Session {
					// Verify zmx is actually running
					zmxSessions, _ := s.zmx.List()
					for _, z := range zmxSessions {
						if z == zmxName {
							zmxAlive = true
							panes++
							break
						}
					}
				}
			}

			if zmxAlive {
				sess.Status = "detached"
				sess.Panes = panes
				log.Printf("[event] session %s now detached (panes=%d)", params.Session, panes)
			} else {
				// No windows, no zmx - remove session
				delete(s.state.Sessions, params.Session)
				log.Printf("[event] session %s removed (no windows, no zmx)", params.Session)
			}
		} else {
			sess.Panes = windowCount
		}
	}
	s.mu.Unlock()

	// Persist daemon state
	if err := s.saveState(); err != nil {
		log.Printf("[window-closed] WARNING: failed to persist state: %v", err)
	}

	return protocol.SuccessResponse(map[string]bool{"ok": true})
}

func (s *Server) handleCloseFocused(k *kitty.Client) protocol.Response {
	if k == nil {
		return protocol.ErrorResponse("no kitty connection available")
	}

	// Get current kitty state
	state, err := k.GetState()
	if err != nil {
		return protocol.ErrorResponse(fmt.Sprintf("get kitty state: %v", err))
	}

	// Find the focused window
	var focusedWindow *kitty.Window
	for _, osWin := range state {
		if !osWin.IsActive {
			continue
		}
		for _, tab := range osWin.Tabs {
			if !tab.IsActive {
				continue
			}
			for i := range tab.Windows {
				if tab.Windows[i].IsActive {
					focusedWindow = &tab.Windows[i]
					break
				}
			}
		}
	}

	if focusedWindow == nil {
		return protocol.ErrorResponse("no focused window found")
	}

	windowID := focusedWindow.ID
	log.Printf("[close] closing focused window %d", windowID)

	// Check if this is a kmux window
	s.mu.Lock()
	zmxName := s.state.Mappings[windowID]
	session := s.state.WindowSessions[windowID]
	s.mu.Unlock()

	// If kmux window, kill zmx session
	if zmxName != "" {
		log.Printf("[close] killing zmx session %s", zmxName)
		s.zmx.Kill(zmxName)
	}

	// Close the kitty window
	if err := k.CloseWindow(windowID); err != nil {
		log.Printf("[close] error closing window: %v", err)
	}

	// Update mappings
	s.mu.Lock()
	delete(s.state.Mappings, windowID)
	delete(s.state.WindowSessions, windowID)
	if zmxName != "" {
		delete(s.state.ZmxOwnership, zmxName)
	}

	// Update session pane count
	if session != "" {
		if sess, exists := s.state.Sessions[session]; exists {
			windowCount := 0
			for _, sessName := range s.state.WindowSessions {
				if sessName == session {
					windowCount++
				}
			}
			sess.Panes = windowCount
			if windowCount == 0 {
				sess.Status = "detached"
			}
		}
	}
	s.mu.Unlock()

	// Persist daemon state
	if err := s.saveState(); err != nil {
		log.Printf("[close] WARNING: failed to persist state: %v", err)
	}

	return protocol.SuccessResponse(protocol.CloseResult{
		Success:  true,
		WindowID: windowID,
		Session:  session,
		Message:  "Window closed",
	})
}

func (s *Server) handleCloseTab(k *kitty.Client) protocol.Response {
	if k == nil {
		return protocol.ErrorResponse("no kitty connection available")
	}

	// Get current kitty state
	state, err := k.GetState()
	if err != nil {
		return protocol.ErrorResponse(fmt.Sprintf("get kitty state: %v", err))
	}

	// Find the focused tab and all its windows
	var focusedTab *kitty.Tab
	for _, osWin := range state {
		if !osWin.IsActive {
			continue
		}
		for i := range osWin.Tabs {
			if osWin.Tabs[i].IsActive {
				focusedTab = &osWin.Tabs[i]
				break
			}
		}
	}

	if focusedTab == nil {
		return protocol.ErrorResponse("no focused tab found")
	}

	log.Printf("[close-tab] closing tab %d with %d windows", focusedTab.ID, len(focusedTab.Windows))

	// Kill zmx sessions for all windows in this tab
	s.mu.Lock()
	var sessionsAffected = make(map[string]bool)
	for _, win := range focusedTab.Windows {
		if zmxName := s.state.Mappings[win.ID]; zmxName != "" {
			log.Printf("[close-tab] killing zmx session %s", zmxName)
			s.zmx.Kill(zmxName)
			delete(s.state.ZmxOwnership, zmxName)
		}
		if session := s.state.WindowSessions[win.ID]; session != "" {
			sessionsAffected[session] = true
		}
		delete(s.state.Mappings, win.ID)
		delete(s.state.WindowSessions, win.ID)
	}
	s.mu.Unlock()

	// Close the tab (using first window ID as match)
	if len(focusedTab.Windows) > 0 {
		if err := k.CloseTab(focusedTab.Windows[0].ID); err != nil {
			log.Printf("[close-tab] error closing tab: %v", err)
		}
	}

	// Update session states
	s.mu.Lock()
	for session := range sessionsAffected {
		if sess, exists := s.state.Sessions[session]; exists {
			windowCount := 0
			for _, sessName := range s.state.WindowSessions {
				if sessName == session {
					windowCount++
				}
			}
			sess.Panes = windowCount
			if windowCount == 0 {
				sess.Status = "detached"
			}
		}
	}
	s.mu.Unlock()

	// Persist daemon state
	if err := s.saveState(); err != nil {
		log.Printf("[close-tab] WARNING: failed to persist state: %v", err)
	}

	var sessionName string
	for s := range sessionsAffected {
		sessionName = s
		break
	}

	return protocol.SuccessResponse(protocol.CloseResult{
		Success:  true,
		WindowID: focusedTab.ID,
		Session:  sessionName,
		Message:  "Tab closed",
	})
}

func (s *Server) runPollingLoop() {
	pollTicker := time.NewTicker(time.Duration(s.cfg.Daemon.WatchInterval) * time.Second)
	saveTicker := time.NewTicker(time.Duration(s.cfg.Daemon.AutoSaveInterval) * time.Second)
	defer pollTicker.Stop()
	defer saveTicker.Stop()

	for {
		select {
		case <-s.done:
			s.autoSaveAll()
			return
		case <-pollTicker.C:
			s.pollState()
		case <-saveTicker.C:
			s.autoSaveAll()
		}
	}
}

func (s *Server) pollState() {
	// Poll zmx for verification
	zmxSessions, _ := s.zmx.List()
	zmxSet := make(map[string]bool)
	for _, z := range zmxSessions {
		zmxSet[z] = true
	}

	// Poll kitty - discover/verify socket each poll cycle
	kittyClient := s.ensureKittyClient()

	var kittyState kitty.KittyState
	currentWindowIDs := make(map[int]bool)

	if kittyClient != nil {
		var err error
		kittyState, err = kittyClient.GetState()
		if err != nil {
			// Current socket failed - clear it so next poll rediscovers
			s.mu.Lock()
			s.kittySocket = ""
			s.kitty = nil
			s.mu.Unlock()
		} else {
			for _, osWin := range kittyState {
				for _, tab := range osWin.Tabs {
					for _, win := range tab.Windows {
						currentWindowIDs[win.ID] = true
					}
				}
			}
		}
	}

	// Verify and update state
	s.mu.Lock()

	s.state.ZmxSessions = zmxSessions
	s.state.KittyState = kittyState
	s.state.LastPoll = time.Now()

	stateChanged := false

	// Check for discrepancies: zmx sessions we own that are gone
	for zmxName, sessName := range s.state.ZmxOwnership {
		if !zmxSet[zmxName] {
			log.Printf("[poll] DISCREPANCY: zmx session %q (owned by %q) no longer exists - removing from ownership",
				zmxName, sessName)
			delete(s.state.ZmxOwnership, zmxName)
			stateChanged = true
		}
	}

	// Adopt orphan zmx sessions that follow our naming convention
	for _, zmxName := range zmxSessions {
		if _, owned := s.state.ZmxOwnership[zmxName]; owned {
			continue // already tracked
		}
		sessName := model.ParseZmxSessionName(zmxName)
		if sessName == "" {
			continue // not our naming convention, ignore
		}
		log.Printf("[poll] adopting orphan zmx session %q -> session %q", zmxName, sessName)
		s.state.ZmxOwnership[zmxName] = sessName
		stateChanged = true
	}

	// Clean up mappings for windows that no longer exist
	for windowID, zmxName := range s.state.Mappings {
		if !currentWindowIDs[windowID] {
			log.Printf("[poll] window %d no longer exists - removing mapping to %q", windowID, zmxName)
			delete(s.state.Mappings, windowID)
			stateChanged = true
		}
	}
	for windowID, sessName := range s.state.WindowSessions {
		if !currentWindowIDs[windowID] {
			log.Printf("[poll] window %d no longer exists - removing session association %q", windowID, sessName)
			delete(s.state.WindowSessions, windowID)
			stateChanged = true
		}
	}

	// Build session status from ownership (authoritative) and verify against reality
	kittyWindowsBySession := make(map[string][]int)
	for windowID, sessName := range s.state.WindowSessions {
		if currentWindowIDs[windowID] {
			kittyWindowsBySession[sessName] = append(kittyWindowsBySession[sessName], windowID)
		}
	}

	// Count zmx panes per session from ownership
	zmxPanesBySession := make(map[string]int)
	for zmxName, sessName := range s.state.ZmxOwnership {
		if zmxSet[zmxName] {
			zmxPanesBySession[sessName]++
		}
	}

	// Create session entries for sessions in ownership but not in Sessions
	// (handles adopted orphans)
	for sessName, panes := range zmxPanesBySession {
		if _, exists := s.state.Sessions[sessName]; !exists {
			log.Printf("[poll] creating session entry for %q (%d panes)", sessName, panes)
			s.state.Sessions[sessName] = &SessionState{
				Name:     sessName,
				Status:   "detached",
				Panes:    panes,
				ZmxAlive: true,
				LastSeen: time.Now(),
			}
			stateChanged = true
		}
	}

	// Update session states
	for name, sess := range s.state.Sessions {
		windowIDs := kittyWindowsBySession[name]
		zmxPanes := zmxPanesBySession[name]

		oldStatus := sess.Status
		oldPanes := sess.Panes

		sess.ZmxAlive = zmxPanes > 0
		sess.WindowIDs = windowIDs

		if len(windowIDs) > 0 {
			sess.Status = "attached"
			sess.Panes = len(windowIDs)
			sess.LastSeen = time.Now()
		} else if sess.ZmxAlive {
			sess.Status = "detached"
			sess.Panes = zmxPanes
			sess.LastSeen = time.Now()
		} else {
			// No windows, no zmx - session is gone
			log.Printf("[poll] session %s: removed (no windows, no zmx)", name)
			delete(s.state.Sessions, name)
			stateChanged = true
			continue
		}

		// Log state changes
		if oldStatus != sess.Status || oldPanes != sess.Panes {
			log.Printf("[poll] session %s: %s/%d -> %s/%d",
				name, oldStatus, oldPanes, sess.Status, sess.Panes)
		}
	}

	s.mu.Unlock()

	// Persist state if changes were detected
	if stateChanged {
		if err := s.saveState(); err != nil {
			log.Printf("[poll] WARNING: failed to persist state after cleanup: %v", err)
		}
	}
}

func (s *Server) autoSaveAll() {
	s.mu.Lock()
	kittyClient := s.kitty
	kittyState := s.state.KittyState
	var attachedSessions []string
	for name, sess := range s.state.Sessions {
		if sess.Status == "attached" {
			attachedSessions = append(attachedSessions, name)
		}
	}
	s.state.LastAutoSave = time.Now()
	s.mu.Unlock()

	// Can't auto-save without kitty state
	if kittyClient == nil || len(kittyState) == 0 {
		return
	}

	// Save each attached session
	for _, name := range attachedSessions {
		s.mu.Lock()
		mappings := s.state.Mappings
		windowSessions := s.state.WindowSessions
		s.mu.Unlock()

		session := manager.DeriveSession(name, kittyState, mappings, windowSessions)
		if len(session.Tabs) > 0 {
			s.store.SaveSession(session)
		}
	}
}

