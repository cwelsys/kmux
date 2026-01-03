package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		home, err := os.UserHomeDir()
		if err != nil {
			// Fallback to empty path if we can't get home directory
			// This will cause operations to fail with clear errors
			return New("")
		}
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

// ValidateSessionName checks if a session name is valid.
// Session names must not be empty, must not contain path separators or special characters,
// and must not be "." or "..".
func ValidateSessionName(name string) error {
	if name == "" || strings.ContainsAny(name, "/\\:*?\"<>|") || name == "." || name == ".." {
		return fmt.Errorf("invalid session name: %q", name)
	}
	return nil
}

// validateSessionName is a deprecated alias for ValidateSessionName.
// Kept for internal backward compatibility.
func validateSessionName(name string) error {
	return ValidateSessionName(name)
}

// SaveSession saves a session to disk.
func (s *Store) SaveSession(session *model.Session) error {
	if err := validateSessionName(session.Name); err != nil {
		return err
	}

	dir := s.sessionsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create sessions dir: %w", err)
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	path := s.sessionPath(session.Name)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename session file: %w", err)
	}

	return nil
}

// LoadSession loads a session from disk.
func (s *Store) LoadSession(name string) (*model.Session, error) {
	if err := validateSessionName(name); err != nil {
		return nil, err
	}

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
	if err := validateSessionName(name); err != nil {
		return err
	}

	path := s.sessionPath(name)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove session file: %w", err)
	}
	return nil
}

// RenameSession renames a session's save file and updates its name.
func (s *Store) RenameSession(oldName, newName string) error {
	oldPath := s.sessionPath(oldName)
	newPath := s.sessionPath(newName)

	// Check old exists
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return fmt.Errorf("session not found: %s", oldName)
	}

	// Check new doesn't exist
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("session already exists: %s", newName)
	}

	// Load, update name, save to new location
	sess, err := s.LoadSession(oldName)
	if err != nil {
		return err
	}
	sess.Name = newName

	if err := s.SaveSession(sess); err != nil {
		return err
	}

	// Remove old file
	return os.Remove(oldPath)
}
