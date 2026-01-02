package server

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/protocol"
	"github.com/cwel/kmux/internal/kitty"
	"github.com/cwel/kmux/internal/store"
	"github.com/cwel/kmux/internal/zmx"
)

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
