package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// PersistedState is the daemon state that survives restarts.
// This is the AUTHORITATIVE source of truth for window/session mappings.
type PersistedState struct {
	// Mappings: kitty_window_id -> zmx_name
	Mappings map[int]string `json:"mappings"`

	// WindowSessions: kitty_window_id -> session_name
	WindowSessions map[int]string `json:"window_sessions"`

	// ZmxOwnership: zmx_name -> session_name (for rename support)
	ZmxOwnership map[string]string `json:"zmx_ownership"`

	// LastSaved: when this state was last persisted
	LastSaved time.Time `json:"last_saved"`
}

// statePath returns the path to the daemon state file.
func (s *Server) statePath() string {
	return filepath.Join(s.dataDir, "daemon-state.json")
}

// saveState persists the daemon's authoritative mappings to disk.
// Called after every mutation (attach, detach, split, close, rename).
func (s *Server) saveState() error {
	s.mu.Lock()
	state := PersistedState{
		Mappings:       make(map[int]string),
		WindowSessions: make(map[int]string),
		ZmxOwnership:   make(map[string]string),
		LastSaved:      time.Now(),
	}

	// Copy maps to avoid holding lock during I/O
	for k, v := range s.state.Mappings {
		state.Mappings[k] = v
	}
	for k, v := range s.state.WindowSessions {
		state.WindowSessions[k] = v
	}
	for k, v := range s.state.ZmxOwnership {
		state.ZmxOwnership[k] = v
	}
	s.mu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	path := s.statePath()
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename state file: %w", err)
	}

	log.Printf("[state] saved daemon state: %d mappings, %d window-sessions, %d zmx-ownership",
		len(state.Mappings), len(state.WindowSessions), len(state.ZmxOwnership))

	return nil
}

// loadState loads persisted daemon state from disk.
// Returns nil if no state file exists (fresh start).
func (s *Server) loadState() (*PersistedState, error) {
	path := s.statePath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil // Fresh start, no persisted state
	}
	if err != nil {
		return nil, fmt.Errorf("read state file: %w", err)
	}

	var state PersistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}

	// Initialize nil maps
	if state.Mappings == nil {
		state.Mappings = make(map[int]string)
	}
	if state.WindowSessions == nil {
		state.WindowSessions = make(map[int]string)
	}
	if state.ZmxOwnership == nil {
		state.ZmxOwnership = make(map[string]string)
	}

	return &state, nil
}
