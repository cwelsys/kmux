package client

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/protocol"
)

// Client is a daemon RPC client.
type Client struct {
	socketPath  string
	kittySocket string // KITTY_LISTEN_ON value
}

// New creates a new daemon client.
func New(socketPath string) *Client {
	// Config socket overrides KITTY_LISTEN_ON if set
	kittySocket := os.Getenv("KITTY_LISTEN_ON")
	if cfg, err := config.LoadConfig(); err == nil && cfg.Kitty.Socket != "" {
		kittySocket = cfg.Kitty.Socket
	}

	return &Client{
		socketPath:  socketPath,
		kittySocket: kittySocket,
	}
}

// IsRunning checks if the daemon is running.
func (c *Client) IsRunning() bool {
	conn, err := net.DialTimeout("unix", c.socketPath, 100*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// EnsureRunning starts the daemon if not running.
func (c *Client) EnsureRunning() error {
	if c.IsRunning() {
		return nil
	}

	// Start daemon
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}

	cmd := exec.Command(executable, "daemon", "start")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	// Wait for socket
	for i := 0; i < 50; i++ { // 5 seconds max
		if c.IsRunning() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("daemon did not start within 5 seconds")
}

// call sends a request and returns the response.
func (c *Client) call(req protocol.Request) (protocol.Response, error) {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return protocol.Response{}, fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return protocol.Response{}, fmt.Errorf("encode: %w", err)
	}

	var resp protocol.Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return protocol.Response{}, fmt.Errorf("decode: %w", err)
	}

	if resp.Error != "" {
		return resp, fmt.Errorf("daemon: %s", resp.Error)
	}

	return resp, nil
}

// Ping checks if the daemon is responsive.
func (c *Client) Ping() error {
	_, err := c.call(protocol.NewRequest(protocol.MethodPing, c.kittySocket))
	return err
}

// Shutdown requests daemon shutdown.
func (c *Client) Shutdown() error {
	_, err := c.call(protocol.NewRequest(protocol.MethodShutdown, c.kittySocket))
	return err
}

// Sessions returns sessions from the daemon.
// If includeRestorePoints is true, also includes restore points (saved sessions without running zmx).
func (c *Client) Sessions(includeRestorePoints bool) ([]protocol.SessionInfo, error) {
	req, err := protocol.NewRequestWithParams(protocol.MethodSessions, c.kittySocket, protocol.SessionsParams{
		IncludeRestorePoints: includeRestorePoints,
	})
	if err != nil {
		return nil, err
	}

	resp, err := c.call(req)
	if err != nil {
		return nil, err
	}

	var sessions []protocol.SessionInfo
	if err := json.Unmarshal(resp.Result, &sessions); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	return sessions, nil
}

// Attach attaches to or creates a session.
func (c *Client) Attach(name, cwd, layout string) error {
	req, err := protocol.NewRequestWithParams(protocol.MethodAttach, c.kittySocket, protocol.AttachParams{
		Name:   name,
		CWD:    cwd,
		Layout: layout,
	})
	if err != nil {
		return err
	}

	resp, err := c.call(req)
	if err != nil {
		return err
	}

	var result protocol.AttachResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("attach failed: %s", result.Message)
	}

	return nil
}

// Detach detaches from a session.
func (c *Client) Detach(name string) error {
	req, err := protocol.NewRequestWithParams(protocol.MethodDetach, c.kittySocket, protocol.DetachParams{
		Name: name,
	})
	if err != nil {
		return err
	}

	_, err = c.call(req)
	return err
}

// Kill kills a session.
func (c *Client) Kill(name string) error {
	req, err := protocol.NewRequestWithParams(protocol.MethodKill, c.kittySocket, protocol.KillParams{
		Name: name,
	})
	if err != nil {
		return err
	}

	_, err = c.call(req)
	return err
}

// Split creates a new split window in a session.
func (c *Client) Split(session, direction, cwd string) (int, error) {
	req, err := protocol.NewRequestWithParams(protocol.MethodSplit, c.kittySocket, protocol.SplitParams{
		Session:   session,
		Direction: direction,
		CWD:       cwd,
	})
	if err != nil {
		return 0, err
	}

	resp, err := c.call(req)
	if err != nil {
		return 0, err
	}

	var result protocol.SplitResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return 0, fmt.Errorf("unmarshal: %w", err)
	}

	if !result.Success {
		return 0, fmt.Errorf("split failed: %s", result.Message)
	}

	return result.WindowID, nil
}

// Resolve looks up which session owns a window.
func (c *Client) Resolve(windowID int) (string, error) {
	req, err := protocol.NewRequestWithParams(protocol.MethodResolve, c.kittySocket, protocol.ResolveParams{
		WindowID: windowID,
	})
	if err != nil {
		return "", err
	}

	resp, err := c.call(req)
	if err != nil {
		return "", err
	}

	var result protocol.ResolveResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("unmarshal: %w", err)
	}

	return result.Session, nil
}

// Rename renames a session.
func (c *Client) Rename(oldName, newName string) error {
	req, err := protocol.NewRequestWithParams(protocol.MethodRename, c.kittySocket, protocol.RenameParams{
		OldName: oldName,
		NewName: newName,
	})
	if err != nil {
		return err
	}

	resp, err := c.call(req)
	if err != nil {
		return err
	}

	var result protocol.RenameResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("rename failed: %s", result.Message)
	}

	return nil
}

// NotifyWindowClosed notifies the daemon that a window was closed.
func (c *Client) NotifyWindowClosed(windowID int, zmxName, session string) error {
	req, err := protocol.NewRequestWithParams(protocol.MethodWindowClosed, c.kittySocket, protocol.WindowClosedParams{
		WindowID: windowID,
		ZmxName:  zmxName,
		Session:  session,
	})
	if err != nil {
		return err
	}

	_, err = c.call(req)
	return err
}
