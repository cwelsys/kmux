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

	// Test that sessions endpoint works (may pick up real zmx sessions)
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

	// Just verify we got a valid response (may include real zmx sessions)
	// The important thing is no error
}

func TestServer_ExcludeSaved(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")
	dataDir := filepath.Join(tmpDir, "data")

	// Create a saved session (no zmx running)
	st := store.New(dataDir)
	sess := &model.Session{
		Name: "testsession_exclude",
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

	// Verify our test session is NOT in the list (it's a restore point, not running)
	for _, s := range sessions {
		if s.Name == "testsession_exclude" {
			t.Error("restore point 'testsession_exclude' should not appear without IncludeRestorePoints")
		}
	}
}

func TestServer_IncludeRestore(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")
	dataDir := filepath.Join(tmpDir, "data")

	// Create a saved session (no zmx running)
	st := store.New(dataDir)
	sess := &model.Session{
		Name: "testsession_restore",
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

	// Find our test session in the list
	var found *protocol.SessionInfo
	for i := range sessions {
		if sessions[i].Name == "testsession_restore" {
			found = &sessions[i]
			break
		}
	}

	if found == nil {
		t.Fatal("restore point 'testsession_restore' should appear with IncludeRestorePoints")
	}
	if !found.IsRestorePoint {
		t.Error("session should be marked as restore point")
	}
}
