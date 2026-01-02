package client

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cwel/kmux/internal/daemon/server"
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
