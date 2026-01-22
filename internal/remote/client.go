package remote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/model"
	"github.com/cwel/kmux/internal/state"
)

// SessionInfo mirrors state.SessionInfo for JSON deserialization from remote.
type SessionInfo = state.SessionInfo

// Client communicates with a remote kmux instance over SSH.
type Client struct {
	host    string
	hostCfg *config.HostConfig
}

// NewClient creates a remote kmux client.
func NewClient(sshAlias string, cfg *config.HostConfig) *Client {
	return &Client{host: sshAlias, hostCfg: cfg}
}

// kmuxPath returns the path to kmux binary on the remote.
func (c *Client) kmuxPath() string {
	if c.hostCfg != nil && c.hostCfg.KmuxPath != "" {
		return c.hostCfg.KmuxPath
	}
	return "kmux"
}

// runKmux executes a kmux command on the remote host.
func (c *Client) runKmux(args ...string) *exec.Cmd {
	kmuxCmd := c.kmuxPath()
	for _, a := range args {
		kmuxCmd += " " + a
	}
	return exec.Command("ssh", c.host, kmuxCmd)
}

// ListSessions returns sessions from the remote host.
func (c *Client) ListSessions() ([]SessionInfo, error) {
	cmd := c.runKmux("session", "list")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("remote kmux session list: %w: %s", err, stderr.String())
	}

	var sessions []SessionInfo
	if err := json.Unmarshal(stdout.Bytes(), &sessions); err != nil {
		return nil, fmt.Errorf("parse remote sessions: %w", err)
	}

	return sessions, nil
}

// GetSession returns a session's save file from the remote host.
func (c *Client) GetSession(name string) (*model.Session, error) {
	cmd := c.runKmux("session", "get", name)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("remote kmux session get %s: %w: %s", name, err, stderr.String())
	}

	var session model.Session
	if err := json.Unmarshal(stdout.Bytes(), &session); err != nil {
		return nil, fmt.Errorf("parse remote session: %w", err)
	}

	return &session, nil
}

// SaveSession sends a session layout to the remote host for storage.
func (c *Client) SaveSession(session *model.Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	cmd := c.runKmux("session", "save", session.Name)
	cmd.Stdin = bytes.NewReader(data)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remote kmux session save %s: %w: %s", session.Name, err, stderr.String())
	}

	return nil
}

// DeleteSession deletes a session save file on the remote host.
func (c *Client) DeleteSession(name string) error {
	cmd := c.runKmux("session", "delete", name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remote kmux session delete %s: %w: %s", name, err, stderr.String())
	}

	return nil
}

// Kill tells the remote kmux to kill a session (zmx + save file).
func (c *Client) Kill(name string) error {
	cmd := c.runKmux("kill", name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remote kmux kill %s: %w: %s", name, err, stderr.String())
	}

	return nil
}
