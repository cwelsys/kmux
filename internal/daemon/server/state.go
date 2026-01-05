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
// Only ZmxOwnership is persisted - window/session info comes from kitty user_vars.
type PersistedState struct {
	// ZmxOwnership: zmx_name -> session_name (for detached session tracking)
	ZmxOwnership map[string]string `json:"zmx_ownership"`

	// LastSaved: when this state was last persisted
	LastSaved time.Time `json:"last_saved"`
}

// statePath returns the path to the daemon state file.
func (s *Server) statePath() string {
	return filepath.Join(s.dataDir, "daemon-state.json")
}

// saveState persists the daemon's zmx ownership to disk.
func (s *Server) saveState() error {
	s.mu.Lock()
	state := PersistedState{
		ZmxOwnership: make(map[string]string),
		LastSaved:    time.Now(),
	}

	// Copy map to avoid holding lock during I/O
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

	log.Printf("[state] saved daemon state: %d zmx-ownership", len(state.ZmxOwnership))

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

	// Initialize nil map
	if state.ZmxOwnership == nil {
		state.ZmxOwnership = make(map[string]string)
	}

	return &state, nil
}
