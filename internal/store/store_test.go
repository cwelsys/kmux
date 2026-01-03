package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cwel/kmux/internal/model"
)

func TestSaveAndLoadSession(t *testing.T) {
	// Use temp directory
	tmpDir := t.TempDir()

	store := New(tmpDir)

	session := &model.Session{
		Name:    "testproject",
		Host:    "local",
		SavedAt: time.Now().Truncate(time.Second), // Truncate for comparison
		Tabs: []model.Tab{
			{
				Title:  "main",
				Layout: "splits",
				Windows: []model.Window{
					{CWD: "/tmp", Command: "nvim"},
				},
			},
		},
		ZmxSessions: []string{"testproject.0.0"},
	}

	// Save
	err := store.SaveSession(session)
	if err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	// Verify file exists
	path := filepath.Join(tmpDir, "sessions", "testproject.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("session file not created at %s", path)
	}

	// Load
	loaded, err := store.LoadSession("testproject")
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}

	if loaded.Name != session.Name {
		t.Errorf("Name = %s, want %s", loaded.Name, session.Name)
	}
	if len(loaded.Tabs) != 1 {
		t.Errorf("Tabs count = %d, want 1", len(loaded.Tabs))
	}
}

func TestListSessions(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)

	// Empty initially
	names, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(names))
	}

	// Add some sessions
	for _, name := range []string{"alpha", "beta", "gamma"} {
		store.SaveSession(&model.Session{Name: name, Host: "local"})
	}

	names, err = store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(names) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(names))
	}
}

func TestRenameSession(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	// Create a session
	sess := &model.Session{Name: "old", Host: "local"}
	if err := s.SaveSession(sess); err != nil {
		t.Fatal(err)
	}

	// Rename it
	if err := s.RenameSession("old", "new"); err != nil {
		t.Fatal(err)
	}

	// Old should not exist
	if _, err := s.LoadSession("old"); err == nil {
		t.Error("expected old session to not exist")
	}

	// New should exist
	loaded, err := s.LoadSession("new")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "new" {
		t.Errorf("expected name 'new', got %q", loaded.Name)
	}
}
