package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Ownership tracks zmx session name → kmux session name mappings.
// This is needed because zmx sessions keep their original names when a session is renamed.
// For example, if you rename session "foo" to "bar", the zmx sessions are still named
// "foo.0.0", "foo.0.1", etc. This file tracks that those zmx sessions belong to "bar".
type Ownership struct {
	// ZmxToSession maps zmx session names to kmux session names
	ZmxToSession map[string]string `json:"zmx_to_session"`
}

var (
	ownershipMu   sync.Mutex
	ownershipPath string
)

func init() {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			dataDir = filepath.Join(home, ".local", "share")
		}
	}
	ownershipPath = filepath.Join(dataDir, "kmux", "zmx-ownership.json")
}

// LoadOwnership loads the ownership mapping from disk.
func LoadOwnership() (*Ownership, error) {
	ownershipMu.Lock()
	defer ownershipMu.Unlock()

	data, err := os.ReadFile(ownershipPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Ownership{ZmxToSession: make(map[string]string)}, nil
		}
		return nil, err
	}

	var o Ownership
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, err
	}
	if o.ZmxToSession == nil {
		o.ZmxToSession = make(map[string]string)
	}
	return &o, nil
}

// SaveOwnership saves the ownership mapping to disk.
func SaveOwnership(o *Ownership) error {
	ownershipMu.Lock()
	defer ownershipMu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(ownershipPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := ownershipPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, ownershipPath)
}

// GetSessionForZmx returns the session name for a zmx session.
// Returns empty string if not found (caller should fall back to parsing the zmx name).
func GetSessionForZmx(zmxName string) string {
	o, err := LoadOwnership()
	if err != nil {
		return ""
	}
	return o.ZmxToSession[zmxName]
}

// SetSessionForZmx sets the session name for a zmx session.
func SetSessionForZmx(zmxName, sessionName string) error {
	o, err := LoadOwnership()
	if err != nil {
		return err
	}
	o.ZmxToSession[zmxName] = sessionName
	return SaveOwnership(o)
}

// RenameSessionOwnership updates all zmx mappings from oldName to newName.
func RenameSessionOwnership(oldName, newName string) error {
	o, err := LoadOwnership()
	if err != nil {
		return err
	}

	// Update all entries that point to oldName
	for zmxName, sessName := range o.ZmxToSession {
		if sessName == oldName {
			o.ZmxToSession[zmxName] = newName
		}
	}

	return SaveOwnership(o)
}

// RemoveSessionOwnership removes all zmx mappings for a session.
func RemoveSessionOwnership(sessionName string) error {
	o, err := LoadOwnership()
	if err != nil {
		return err
	}

	// Remove all entries that point to this session
	for zmxName, sessName := range o.ZmxToSession {
		if sessName == sessionName {
			delete(o.ZmxToSession, zmxName)
		}
	}

	return SaveOwnership(o)
}

// RemoveZmxOwnership removes a specific zmx session from ownership.
func RemoveZmxOwnership(zmxName string) error {
	o, err := LoadOwnership()
	if err != nil {
		return err
	}
	delete(o.ZmxToSession, zmxName)
	return SaveOwnership(o)
}

// GetZmxSessionsForSession returns all zmx session names owned by a session.
func GetZmxSessionsForSession(sessionName string) []string {
	o, err := LoadOwnership()
	if err != nil {
		return nil
	}

	var zmxNames []string
	for zmxName, sessName := range o.ZmxToSession {
		if sessName == sessionName {
			zmxNames = append(zmxNames, zmxName)
		}
	}
	return zmxNames
}
