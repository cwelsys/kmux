package server

import (
	"encoding/json"
	"fmt"
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
	Sessions     map[string]*SessionState
	KittyState   kitty.KittyState
	ZmxSessions  []string
	LastPoll     time.Time
	LastAutoSave time.Time
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
	return &Server{
		socketPath: socketPath,
		dataDir:    dataDir,
		done:       make(chan struct{}),
		store:      store.New(dataDir),
		kitty:      kitty.NewClient(),
		zmx:        zmx.NewClient(),
		cfg:        config.DefaultConfig(),
		state: &DaemonState{
			Sessions: make(map[string]*SessionState),
		},
	}
}

// Start starts the daemon server.
func (s *Server) Start() error {
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

// initState discovers running zmx sessions only (not save files)
func (s *Server) initState() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get running zmx sessions and extract session names
	zmxSessions, _ := s.zmx.List()
	zmxBySession := make(map[string]bool)
	for _, z := range zmxSessions {
		// Extract session name from "sessionname.tab.window"
		for i, c := range z {
			if c == '.' {
				zmxBySession[z[:i]] = true
				break
			}
		}
	}

	// Only create entries for running zmx sessions
	for name := range zmxBySession {
		// Count panes from zmx processes
		panes := 0
		prefix := name + "."
		for _, z := range zmxSessions {
			if len(z) > len(prefix) && z[:len(prefix)] == prefix {
				panes++
			}
		}
		if panes == 0 {
			panes = 1
		}

		s.state.Sessions[name] = &SessionState{
			Name:     name,
			Status:   "detached", // running zmx but no kitty windows yet
			Panes:    panes,
			ZmxAlive: true,
			LastSeen: time.Now(),
		}
	}

	s.state.ZmxSessions = zmxSessions
	s.state.LastPoll = time.Now()
}

// countZmxPanes counts zmx sessions with the given prefix
func countZmxPanes(zmxSessions []string, sessionName string) int {
	prefix := sessionName + "."
	count := 0
	for _, z := range zmxSessions {
		if len(z) > len(prefix) && z[:len(prefix)] == prefix {
			count++
		}
	}
	if count == 0 {
		return 1
	}
	return count
}

func (s *Server) handleSessions(k *kitty.Client, params protocol.SessionsParams) protocol.Response {
	s.mu.Lock()
	defer s.mu.Unlock()

	var sessions []protocol.SessionInfo

	// Add running sessions
	for _, sess := range s.state.Sessions {
		sessions = append(sessions, protocol.SessionInfo{
			Name:           sess.Name,
			Status:         sess.Status,
			Panes:          sess.Panes,
			IsRestorePoint: false,
		})
	}

	// Add restore points if requested
	if params.IncludeRestorePoints {
		saved, _ := s.store.ListSessions()
		runningNames := make(map[string]bool)
		for _, sess := range s.state.Sessions {
			runningNames[sess.Name] = true
		}

		for _, name := range saved {
			if runningNames[name] {
				continue // already listed as running
			}

			// Load to get pane count
			panes := 1
			if sess, err := s.store.LoadSession(name); err == nil {
				panes = 0
				for _, tab := range sess.Tabs {
					panes += len(tab.Windows)
				}
			}

			sessions = append(sessions, protocol.SessionInfo{
				Name:           name,
				Status:         "saved",
				Panes:          panes,
				IsRestorePoint: true,
			})
		}
	}

	return protocol.SuccessResponse(sessions)
}

func (s *Server) handleAttach(k *kitty.Client, params protocol.AttachParams) protocol.Response {
	name := params.Name

	if err := store.ValidateSessionName(name); err != nil {
		return protocol.ErrorResponse(err.Error())
	}

	// Try to load existing session
	session, err := s.store.LoadSession(name)
	if err != nil {
		// Create new session
		cwd := params.CWD
		if cwd == "" {
			cwd = "/"
		}
		session = &model.Session{
			Name:    name,
			Host:    "local",
			SavedAt: time.Now(),
			Tabs: []model.Tab{
				{
					Title:  name,
					Layout: "splits",
					Windows: []model.Window{
						{CWD: cwd},
					},
				},
			},
		}
	}

	// Clear ZmxSessions before rebuilding
	session.ZmxSessions = nil

	// Create windows in kitty using RestoreTab
	var firstWindowID int
	for tabIdx, tab := range session.Tabs {
		windowID, err := manager.RestoreTab(k, session, tabIdx, tab)
		if err != nil {
			return protocol.ErrorResponse(fmt.Sprintf("restore tab: %v", err))
		}
		if tabIdx == 0 {
			firstWindowID = windowID
		}
	}

	// Focus first window
	if firstWindowID > 0 {
		k.FocusWindow(firstWindowID)
	}

	// Save session to disk
	if err := s.store.SaveSession(session); err != nil {
		return protocol.ErrorResponse(fmt.Sprintf("save session: %v", err))
	}

	// Update internal state
	s.mu.Lock()
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

	// Derive session from current state
	session := manager.DeriveSession(name, state)

	// Save session
	if err := s.store.SaveSession(session); err != nil {
		return protocol.ErrorResponse(fmt.Sprintf("save session: %v", err))
	}

	// Close windows belonging to this session
	if len(state) > 0 {
		for _, tab := range state[0].Tabs {
			for _, win := range tab.Windows {
				if win.Env["KMUX_SESSION"] == name {
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
	s.mu.Unlock()

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

	// Kill all zmx sessions that belong to this session (by prefix matching)
	// This is more robust than relying on saved session.ZmxSessions
	// since splits created after initial attach won't be in the saved list
	running, _ := s.zmx.List()
	for _, r := range running {
		if len(r) > len(name) && r[:len(name)+1] == name+"." {
			s.zmx.Kill(r)
		}
	}

	// Close any kitty windows for this session
	state, _ := k.GetState()
	if len(state) > 0 {
		for _, tab := range state[0].Tabs {
			for _, win := range tab.Windows {
				if win.Env["KMUX_SESSION"] == name {
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
	s.mu.Unlock()

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

	// Find the tab containing windows for this session
	var targetTabIdx int = -1
	var windowCount int
	for _, osWin := range state {
		for tabIdx, tab := range osWin.Tabs {
			for _, win := range tab.Windows {
				if win.Env["KMUX_SESSION"] == sessionName {
					targetTabIdx = tabIdx
					windowCount++
				}
			}
		}
	}

	if targetTabIdx == -1 {
		return protocol.ErrorResponse(fmt.Sprintf("no windows found for session: %s", sessionName))
	}

	// Build zmx session name: {session}.{tab}.{window}
	zmxName := fmt.Sprintf("%s.%d.%d", sessionName, targetTabIdx, windowCount)
	zmxCmd := zmx.AttachCmd(zmxName, "")

	// Launch the split window with zmx
	opts := kitty.LaunchOpts{
		Type:     "window",
		Location: location,
		CWD:      cwd,
		Cmd:      zmxCmd,
		Env:      map[string]string{"KMUX_SESSION": sessionName},
	}

	windowID, err := k.Launch(opts)
	if err != nil {
		return protocol.ErrorResponse(fmt.Sprintf("launch split: %v", err))
	}

	// Update internal state
	s.mu.Lock()
	if sess, ok := s.state.Sessions[sessionName]; ok {
		sess.Panes++
		sess.LastSeen = time.Now()
	}
	s.mu.Unlock()

	return protocol.SuccessResponse(protocol.SplitResult{
		Success:  true,
		WindowID: windowID,
		Message:  fmt.Sprintf("Created %s split in session %s", params.Direction, sessionName),
	})
}

func (s *Server) runPollingLoop() {
	pollTicker := time.NewTicker(time.Duration(s.cfg.WatchInterval) * time.Second)
	saveTicker := time.NewTicker(time.Duration(s.cfg.AutoSaveInterval) * time.Second)
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
	// Poll zmx
	zmxSessions, _ := s.zmx.List()

	// Extract session names from zmx
	zmxBySession := make(map[string]bool)
	for _, z := range zmxSessions {
		for i, c := range z {
			if c == '.' {
				zmxBySession[z[:i]] = true
				break
			}
		}
	}

	// Poll kitty - discover/verify socket each poll cycle
	kittyClient := s.ensureKittyClient()

	var kittyState kitty.KittyState
	kittyWindowsBySession := make(map[string][]int) // session name -> window IDs

	if kittyClient != nil {
		var err error
		kittyState, err = kittyClient.GetState()
		if err != nil {
			// Current socket failed - clear it so next poll rediscovers
			s.mu.Lock()
			s.kittySocket = ""
			s.kitty = nil
			s.mu.Unlock()
		} else if len(kittyState) > 0 {
			// Extract windows by KMUX_SESSION
			for _, osWin := range kittyState {
				for _, tab := range osWin.Tabs {
					for _, win := range tab.Windows {
						if sessName := win.Env["KMUX_SESSION"]; sessName != "" {
							kittyWindowsBySession[sessName] = append(kittyWindowsBySession[sessName], win.ID)
						}
					}
				}
			}
		}
	}

	// Update state
	s.mu.Lock()

	prevWindowIDs := make(map[string][]int)
	for name, sess := range s.state.Sessions {
		prevWindowIDs[name] = sess.WindowIDs
	}

	s.state.ZmxSessions = zmxSessions
	s.state.KittyState = kittyState
	s.state.LastPoll = time.Now()

	// Discover new sessions from zmx
	for name := range zmxBySession {
		if _, exists := s.state.Sessions[name]; !exists {
			// Count panes from zmx processes
			panes := countZmxPanes(zmxSessions, name)
			s.state.Sessions[name] = &SessionState{
				Name:     name,
				Status:   "detached",
				Panes:    panes,
				ZmxAlive: true,
				LastSeen: time.Now(),
			}
		}
	}

	// Discover new sessions from kitty
	for name := range kittyWindowsBySession {
		if _, exists := s.state.Sessions[name]; !exists {
			s.state.Sessions[name] = &SessionState{
				Name:     name,
				Status:   "attached",
				Panes:    len(kittyWindowsBySession[name]),
				ZmxAlive: zmxBySession[name],
				LastSeen: time.Now(),
			}
		}
	}

	// Update existing sessions
	var sessionsToSave []string
	for name, sess := range s.state.Sessions {
		sess.ZmxAlive = zmxBySession[name]
		windowIDs := kittyWindowsBySession[name]
		sess.WindowIDs = windowIDs

		// Update pane count from windows or zmx
		if len(windowIDs) > 0 {
			sess.Panes = len(windowIDs)
		} else if sess.ZmxAlive {
			// For detached sessions, count zmx processes
			sess.Panes = countZmxPanes(s.state.ZmxSessions, name)
		}

		// Determine status
		hasWindows := len(windowIDs) > 0
		prevHadWindows := len(prevWindowIDs[name]) > 0

		if hasWindows {
			sess.Status = "attached"
			sess.LastSeen = time.Now()
		} else if sess.ZmxAlive {
			// Windows disappeared but zmx still running - save immediately
			if prevHadWindows {
				sessionsToSave = append(sessionsToSave, name)
			}
			sess.Status = "detached"
			sess.LastSeen = time.Now()
		} else {
			// No windows, no zmx - session is gone
			delete(s.state.Sessions, name)
		}
	}

	s.mu.Unlock()

	// Save sessions that lost windows (outside lock to avoid deadlock)
	for _, name := range sessionsToSave {
		s.saveSession(name)
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
		session := manager.DeriveSession(name, kittyState)
		if len(session.Tabs) > 0 {
			s.store.SaveSession(session)
		}
	}
}

// saveSession derives and saves a session from current kitty state
func (s *Server) saveSession(name string) {
	s.mu.Lock()
	kittyState := s.state.KittyState
	s.mu.Unlock()

	if len(kittyState) == 0 {
		return
	}

	session := manager.DeriveSession(name, kittyState)
	if len(session.Tabs) > 0 {
		s.store.SaveSession(session)
	}
}
