package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cwel/kmux/internal/model"
)

// Store handles session persistence.
type Store struct {
	baseDir string
}

// New creates a new Store with the given base directory.
func New(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

// DefaultStore returns a Store using the default XDG data directory.
func DefaultStore() *Store {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".local", "share")
	}
	return New(filepath.Join(dataDir, "kmux"))
}

// sessionsDir returns the path to the sessions directory.
func (s *Store) sessionsDir() string {
	return filepath.Join(s.baseDir, "sessions")
}

// sessionPath returns the path to a session file.
func (s *Store) sessionPath(name string) string {
	return filepath.Join(s.sessionsDir(), name+".json")
}

// SaveSession saves a session to disk.
func (s *Store) SaveSession(session *model.Session) error {
	dir := s.sessionsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create sessions dir: %w", err)
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	path := s.sessionPath(session.Name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

// LoadSession loads a session from disk.
func (s *Store) LoadSession(name string) (*model.Session, error) {
	path := s.sessionPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var session model.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	return &session, nil
}

// ListSessions returns the names of all saved sessions.
func (s *Store) ListSessions() ([]string, error) {
	dir := s.sessionsDir()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read sessions dir: %w", err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			names = append(names, e.Name()[:len(e.Name())-5]) // strip .json
		}
	}
	return names, nil
}

// DeleteSession removes a session file.
func (s *Store) DeleteSession(name string) error {
	path := s.sessionPath(name)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove session file: %w", err)
	}
	return nil
}
