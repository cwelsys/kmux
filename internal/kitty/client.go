package kitty

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

// Client communicates with kitty via `kitty @` commands.
type Client struct {
	socketPath string // Empty means use default (KITTY_LISTEN_ON)
}

// NewClient creates a new kitty client.
func NewClient() *Client {
	return &Client{}
}

// NewClientWithSocket creates a client that connects to a specific socket.
func NewClientWithSocket(path string) *Client {
	return &Client{socketPath: path}
}

// ParseState parses JSON output from `kitty @ ls`.
func ParseState(data []byte) (KittyState, error) {
	var state KittyState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse kitty state: %w", err)
	}
	return state, nil
}

// GetState retrieves the current kitty state.
func (c *Client) GetState() (KittyState, error) {
	args := []string{"@", "ls"}
	if c.socketPath != "" {
		args = []string{"@", "--to", "unix:" + c.socketPath, "ls"}
	}

	cmd := exec.Command("kitty", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("kitty @ ls: %w: %s", err, stderr.String())
	}

	return ParseState(stdout.Bytes())
}

// Launch creates a new window/tab in kitty.
func (c *Client) Launch(opts LaunchOpts) (int, error) {
	args := []string{"@"}
	if c.socketPath != "" {
		args = append(args, "--to", "unix:"+c.socketPath)
	}
	args = append(args, "launch")

	if opts.Type != "" {
		args = append(args, "--type", opts.Type)
	}
	if opts.CWD != "" {
		args = append(args, "--cwd", opts.CWD)
	}
	if opts.Title != "" {
		args = append(args, "--title", opts.Title)
	}
	if opts.Location != "" {
		args = append(args, "--location", opts.Location)
	}
	if len(opts.Cmd) > 0 {
		args = append(args, opts.Cmd...)
	}

	cmd := exec.Command("kitty", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("kitty @ launch: %w: %s", err, stderr.String())
	}

	// Parse window ID from output
	var id int
	fmt.Sscanf(stdout.String(), "%d", &id)
	return id, nil
}

// LaunchOpts specifies options for launching a new window.
type LaunchOpts struct {
	Type     string   // "window", "tab", "os-window"
	CWD      string
	Title    string
	Location string   // "first", "after", "before", "neighbor", "last", "vsplit", "hsplit"
	Cmd      []string
}

// CloseWindow closes a window by ID.
func (c *Client) CloseWindow(id int) error {
	args := []string{"@"}
	if c.socketPath != "" {
		args = append(args, "--to", "unix:"+c.socketPath)
	}
	args = append(args, "close-window", "--match", fmt.Sprintf("id:%d", id))

	cmd := exec.Command("kitty", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kitty @ close-window: %w: %s", err, stderr.String())
	}
	return nil
}

// CloseTab closes a tab by ID.
func (c *Client) CloseTab(id int) error {
	args := []string{"@"}
	if c.socketPath != "" {
		args = append(args, "--to", "unix:"+c.socketPath)
	}
	args = append(args, "close-tab", "--match", fmt.Sprintf("id:%d", id))

	cmd := exec.Command("kitty", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kitty @ close-tab: %w: %s", err, stderr.String())
	}
	return nil
}
