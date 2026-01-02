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
	Status    string // "attached", "detached", "saved"
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
	socketPath string
	dataDir    string
	listener   net.Listener
	mu         sync.Mutex
	done       chan struct{}

	// Internal clients - daemon owns these
	store *store.Store
	kitty *kitty.Client
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
	switch req.Method {
	case protocol.MethodPing:
		return protocol.SuccessResponse("pong")
	case protocol.MethodSessions:
		return s.handleSessions()
	case protocol.MethodAttach:
		var params protocol.AttachParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return protocol.ErrorResponse(fmt.Sprintf("invalid params: %v", err))
		}
		return s.handleAttach(params)
	case protocol.MethodDetach:
		var params protocol.DetachParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return protocol.ErrorResponse(fmt.Sprintf("invalid params: %v", err))
		}
		return s.handleDetach(params)
	case protocol.MethodKill:
		var params protocol.KillParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return protocol.ErrorResponse(fmt.Sprintf("invalid params: %v", err))
		}
		return s.handleKill(params)
	case protocol.MethodShutdown:
		go func() {
			s.Stop()
		}()
		return protocol.SuccessResponse("shutting down")
	default:
		return protocol.ErrorResponse(fmt.Sprintf("unknown method: %s", req.Method))
	}
}

func (s *Server) handleSessions() protocol.Response {
	names, err := s.store.ListSessions()
	if err != nil {
		return protocol.ErrorResponse(fmt.Sprintf("list sessions: %v", err))
	}

	// Get running zmx sessions
	running, _ := s.zmx.List()
	runningSet := make(map[string]bool)
	for _, r := range running {
		// Extract session name from "sessionname.tab.window"
		for i, c := range r {
			if c == '.' {
				runningSet[r[:i]] = true
				break
			}
		}
	}

	var sessions []protocol.SessionInfo
	for _, name := range names {
		sess, err := s.store.LoadSession(name)
		panes := 0
		if err == nil {
			for _, tab := range sess.Tabs {
				panes += len(tab.Windows)
			}
		}

		status := "saved"
		if runningSet[name] {
			status = "running"
		}

		sessions = append(sessions, protocol.SessionInfo{
			Name:   name,
			Status: status,
			Panes:  panes,
		})
	}

	return protocol.SuccessResponse(sessions)
}

func (s *Server) handleAttach(params protocol.AttachParams) protocol.Response {
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

	// Create windows in kitty
	var firstWindowID int
	for tabIdx, tab := range session.Tabs {
		for winIdx, win := range tab.Windows {
			zmxName := session.ZmxSessionName(tabIdx, winIdx)
			zmxCmd := zmx.AttachCmd(zmxName, win.Command)

			launchType := "tab"
			if winIdx > 0 {
				launchType = "window"
			}

			opts := kitty.LaunchOpts{
				Type:  launchType,
				CWD:   win.CWD,
				Title: tab.Title,
				Cmd:   zmxCmd,
				Env:   map[string]string{"KMUX_SESSION": name},
			}

			windowID, err := s.kitty.Launch(opts)
			if err != nil {
				return protocol.ErrorResponse(fmt.Sprintf("launch window: %v", err))
			}

			if tabIdx == 0 && winIdx == 0 {
				firstWindowID = windowID
			}

			session.ZmxSessions = append(session.ZmxSessions, zmxName)
		}
	}

	// Focus first window
	if firstWindowID > 0 {
		s.kitty.FocusWindow(firstWindowID)
	}

	// Save session
	if err := s.store.SaveSession(session); err != nil {
		return protocol.ErrorResponse(fmt.Sprintf("save session: %v", err))
	}

	return protocol.SuccessResponse(protocol.AttachResult{
		Success: true,
		Message: fmt.Sprintf("Attached to session: %s", name),
	})
}

func (s *Server) handleDetach(params protocol.DetachParams) protocol.Response {
	name := params.Name

	if err := store.ValidateSessionName(name); err != nil {
		return protocol.ErrorResponse(err.Error())
	}

	// Get current kitty state
	state, err := s.kitty.GetState()
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
					s.kitty.CloseWindow(win.ID)
				}
			}
		}
	}

	return protocol.SuccessResponse(protocol.AttachResult{
		Success: true,
		Message: fmt.Sprintf("Detached from session: %s", name),
	})
}

func (s *Server) handleKill(params protocol.KillParams) protocol.Response {
	name := params.Name

	if err := store.ValidateSessionName(name); err != nil {
		return protocol.ErrorResponse(err.Error())
	}

	// Load session to get zmx session names
	session, err := s.store.LoadSession(name)
	if err != nil {
		// Session might not be saved, but zmx sessions might exist
		// Try to find them by prefix
		running, _ := s.zmx.List()
		for _, r := range running {
			if len(r) > len(name) && r[:len(name)+1] == name+"." {
				s.zmx.Kill(r)
			}
		}
	} else {
		// Kill all zmx sessions
		for _, zmxName := range session.ZmxSessions {
			s.zmx.Kill(zmxName)
		}
	}

	// Close any kitty windows for this session
	state, _ := s.kitty.GetState()
	if len(state) > 0 {
		for _, tab := range state[0].Tabs {
			for _, win := range tab.Windows {
				if win.Env["KMUX_SESSION"] == name {
					s.kitty.CloseWindow(win.ID)
				}
			}
		}
	}

	// Delete saved session
	s.store.DeleteSession(name)

	return protocol.SuccessResponse(protocol.AttachResult{
		Success: true,
		Message: fmt.Sprintf("Killed session: %s", name),
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
	// Poll kitty
	kittyState, _ := s.kitty.GetState()

	// Poll zmx
	zmxSessions, _ := s.zmx.List()
	zmxSet := make(map[string]bool)
	for _, z := range zmxSessions {
		zmxSet[z] = true
	}

	// Update state
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.KittyState = kittyState
	s.state.ZmxSessions = zmxSessions
	s.state.LastPoll = time.Now()

	// Track which sessions have windows
	sessionWindows := make(map[string][]int)
	if len(kittyState) > 0 {
		for _, tab := range kittyState[0].Tabs {
			for _, win := range tab.Windows {
				if name := win.Env["KMUX_SESSION"]; name != "" {
					sessionWindows[name] = append(sessionWindows[name], win.ID)
				}
			}
		}
	}

	// Update session states
	saved, _ := s.store.ListSessions()
	for _, name := range saved {
		sess, ok := s.state.Sessions[name]
		if !ok {
			sess = &SessionState{Name: name}
			s.state.Sessions[name] = sess
		}

		sess.WindowIDs = sessionWindows[name]
		sess.ZmxAlive = false
		for _, z := range zmxSessions {
			if len(z) > len(name) && z[:len(name)+1] == name+"." {
				sess.ZmxAlive = true
				break
			}
		}

		if len(sess.WindowIDs) > 0 {
			sess.Status = "attached"
		} else if sess.ZmxAlive {
			sess.Status = "detached"
		} else {
			sess.Status = "saved"
		}

		sess.LastSeen = time.Now()
	}
}

func (s *Server) autoSaveAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for name, sess := range s.state.Sessions {
		if sess.Status == "attached" && len(s.state.KittyState) > 0 {
			session := manager.DeriveSession(name, s.state.KittyState)
			s.store.SaveSession(session)
		}
	}

	s.state.LastAutoSave = time.Now()
}
