package client

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cwel/kmux/internal/daemon/protocol"
	"github.com/cwel/kmux/internal/daemon/server"
	"github.com/cwel/kmux/internal/model"
	"github.com/cwel/kmux/internal/store"
)

func TestClient_Ping(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	// Start server
	srv := server.New(socketPath, tmpDir)
	go srv.Start()
	defer srv.Stop()
	time.Sleep(50 * time.Millisecond)

	// Create client and ping
	c := New(socketPath)
	if err := c.Ping(); err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

func TestClient_IsRunning(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	c := New(socketPath)

	// Not running initially
	if c.IsRunning() {
		t.Error("IsRunning() = true before server start")
	}

	// Start server
	srv := server.New(socketPath, tmpDir)
	go srv.Start()
	defer srv.Stop()
	time.Sleep(50 * time.Millisecond)

	// Now running
	if !c.IsRunning() {
		t.Error("IsRunning() = false after server start")
	}
}

func TestClient_Sessions(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")
	dataDir := filepath.Join(tmpDir, "data")

	// Create a saved session
	st := store.New(dataDir)
	sess := &model.Session{
		Name: "testsession",
		Tabs: []model.Tab{{Windows: []model.Window{{CWD: "/tmp"}}}},
	}
	st.SaveSession(sess)

	srv := server.New(socketPath, dataDir)
	go srv.Start()
	defer srv.Stop()
	time.Sleep(50 * time.Millisecond)

	c := New(socketPath)
	sessions, err := c.Sessions(true) // include restore points to see saved session
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].Name != "testsession" {
		t.Errorf("got name %q, want testsession", sessions[0].Name)
	}
}

func TestClient_SessionsAll(t *testing.T) {
	// This tests the method signature exists
	c := New("/tmp/test.sock")

	// Method should exist and accept bool parameter
	var _ func(bool) ([]protocol.SessionInfo, error) = c.Sessions
}
