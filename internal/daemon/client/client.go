package client

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/cwel/kmux/internal/daemon/protocol"
)

// Client is a daemon RPC client.
type Client struct {
	socketPath string
}

// New creates a new daemon client.
func New(socketPath string) *Client {
	return &Client{socketPath: socketPath}
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
	_, err := c.call(protocol.NewRequest(protocol.MethodPing))
	return err
}

// Shutdown requests daemon shutdown.
func (c *Client) Shutdown() error {
	_, err := c.call(protocol.NewRequest(protocol.MethodShutdown))
	return err
}

// Sessions returns all sessions from the daemon.
func (c *Client) Sessions() ([]protocol.SessionInfo, error) {
	resp, err := c.call(protocol.NewRequest(protocol.MethodSessions))
	if err != nil {
		return nil, err
	}

	var sessions []protocol.SessionInfo
	if err := json.Unmarshal(resp.Result, &sessions); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	return sessions, nil
}
