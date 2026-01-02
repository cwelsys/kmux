package server

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cwel/kmux/internal/daemon/protocol"
	"github.com/cwel/kmux/internal/model"
	"github.com/cwel/kmux/internal/store"
)

func TestServer_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	srv := New(socketPath, tmpDir)

	// Start in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	// Wait for socket to be ready
	time.Sleep(50 * time.Millisecond)

	// Verify socket exists
	if _, err := os.Stat(socketPath); err != nil {
		t.Fatalf("socket not created: %v", err)
	}

	// Stop server
	srv.Stop()

	// Wait for Start to return
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Start returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Error("Start did not return after Stop")
	}
}

func TestServer_Ping(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	srv := New(socketPath, tmpDir)

	go srv.Start()
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Connect and send ping
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	req := protocol.NewRequest(protocol.MethodPing, "")
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	var resp protocol.Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}

	var result string
	json.Unmarshal(resp.Result, &result)
	if result != "pong" {
		t.Errorf("got %q, want pong", result)
	}
}

func TestServer_Sessions(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	// No saved sessions, no zmx - should return empty list
	srv := New(socketPath, tmpDir)
	go srv.Start()
	defer srv.Stop()
	time.Sleep(50 * time.Millisecond)

	// Request sessions
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	req := protocol.NewRequest(protocol.MethodSessions, "")
	json.NewEncoder(conn).Encode(req)

	var resp protocol.Response
	json.NewDecoder(conn).Decode(&resp)

	if resp.Error != "" {
		t.Fatalf("error: %s", resp.Error)
	}

	var sessions []protocol.SessionInfo
	json.Unmarshal(resp.Result, &sessions)

	// Should be empty - no zmx running
	if len(sessions) != 0 {
		t.Errorf("got %d sessions, want 0", len(sessions))
	}
}

func TestServer_ExcludeSaved(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")
	dataDir := filepath.Join(tmpDir, "data")

	// Create a saved session (no zmx running)
	st := store.New(dataDir)
	sess := &model.Session{
		Name: "testsession",
		Tabs: []model.Tab{{Windows: []model.Window{{CWD: "/tmp"}}}},
	}
	if err := st.SaveSession(sess); err != nil {
		t.Fatalf("save session: %v", err)
	}

	srv := New(socketPath, dataDir)
	go srv.Start()
	defer srv.Stop()
	time.Sleep(50 * time.Millisecond)

	// Request sessions WITHOUT --all
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	req, _ := protocol.NewRequestWithParams(protocol.MethodSessions, "", protocol.SessionsParams{
		IncludeRestorePoints: false,
	})
	json.NewEncoder(conn).Encode(req)

	var resp protocol.Response
	json.NewDecoder(conn).Decode(&resp)

	if resp.Error != "" {
		t.Fatalf("error: %s", resp.Error)
	}

	var sessions []protocol.SessionInfo
	json.Unmarshal(resp.Result, &sessions)

	// Should be empty - no zmx running, just a save file
	if len(sessions) != 0 {
		t.Errorf("got %d sessions, want 0 (saved sessions should be excluded)", len(sessions))
	}
}

func TestServer_IncludeRestore(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")
	dataDir := filepath.Join(tmpDir, "data")

	// Create a saved session (no zmx running)
	st := store.New(dataDir)
	sess := &model.Session{
		Name: "testsession",
		Tabs: []model.Tab{{Windows: []model.Window{{CWD: "/tmp"}}}},
	}
	if err := st.SaveSession(sess); err != nil {
		t.Fatalf("save session: %v", err)
	}

	srv := New(socketPath, dataDir)
	go srv.Start()
	defer srv.Stop()
	time.Sleep(50 * time.Millisecond)

	// Request sessions WITH --all
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	req, _ := protocol.NewRequestWithParams(protocol.MethodSessions, "", protocol.SessionsParams{
		IncludeRestorePoints: true,
	})
	json.NewEncoder(conn).Encode(req)

	var resp protocol.Response
	json.NewDecoder(conn).Decode(&resp)

	if resp.Error != "" {
		t.Fatalf("error: %s", resp.Error)
	}

	var sessions []protocol.SessionInfo
	json.Unmarshal(resp.Result, &sessions)

	// Should include the restore point
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].Name != "testsession" {
		t.Errorf("got name %q, want testsession", sessions[0].Name)
	}
	if !sessions[0].IsRestorePoint {
		t.Error("session should be marked as restore point")
	}
}
