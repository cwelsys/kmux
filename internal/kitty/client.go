package kitty

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Client communicates with kitty via `kitty @` commands.
type Client struct {
	socketPath string // Socket path from config, or empty to use kitty's default discovery
}

// NewClient creates a new kitty client with no socket path.
// Use NewClientWithSocket to specify the socket from config.
func NewClient() *Client {
	return &Client{}
}

// NewClientWithSocket creates a client with an explicit socket path.
// The socket is resolved using environment and filesystem checks.
func NewClientWithSocket(socketPath string) *Client {
	return &Client{socketPath: resolveSocket(socketPath)}
}

// resolveSocket determines the actual kitty socket path.
// Priority: KITTY_LISTEN_ON env → config path with KITTY_PID suffix → exact config path.
func resolveSocket(configured string) string {
	// 1. KITTY_LISTEN_ON is definitive (set by kitty in child processes)
	if listenOn := os.Getenv("KITTY_LISTEN_ON"); listenOn != "" {
		return strings.TrimPrefix(listenOn, "unix:")
	}

	// 2. Kitty appends -<PID> to listen_on paths; construct and verify
	if kittyPID := os.Getenv("KITTY_PID"); kittyPID != "" {
		pidPath := configured + "-" + kittyPID
		if _, err := os.Stat(pidPath); err == nil {
			return pidPath
		}
	}

	// 3. Exact path exists (e.g. macOS --listen-on CLI flag)
	if _, err := os.Stat(configured); err == nil {
		return configured
	}

	// 4. Fallback to configured path as-is (error will surface from kitty)
	return configured
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
	if opts.Bias > 0 {
		args = append(args, "--bias", fmt.Sprintf("%d", opts.Bias))
	}
	// Add environment variables
	for key, val := range opts.Env {
		args = append(args, "--env", key+"="+val)
	}
	// Add user variables (stored on the window, queryable via kitty @ ls)
	for key, val := range opts.Vars {
		args = append(args, "--var", key+"="+val)
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
	if n, _ := fmt.Sscanf(stdout.String(), "%d", &id); n != 1 {
		return 0, fmt.Errorf("kitty @ launch: unexpected output: %q", stdout.String())
	}
	return id, nil
}

// LaunchOpts specifies options for launching a new window.
type LaunchOpts struct {
	Type     string            // "window", "tab", "os-window"
	CWD      string
	Title    string
	Location string            // "first", "after", "before", "neighbor", "last", "vsplit", "hsplit"
	Cmd      []string
	Env      map[string]string // Environment variables to pass to launched window
	Vars     map[string]string // User variables to set on the window (kitty --var)
	Bias     int               // 0-100 percentage for split bias (0 means default/equal)
}

// FocusWindow focuses a window by ID.
func (c *Client) FocusWindow(id int) error {
	args := []string{"@"}
	if c.socketPath != "" {
		args = append(args, "--to", "unix:"+c.socketPath)
	}
	args = append(args, "focus-window", "--match", fmt.Sprintf("id:%d", id))

	cmd := exec.Command("kitty", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kitty @ focus-window: %w: %s", err, stderr.String())
	}
	return nil
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

// GotoLayout changes the layout of the active tab.
func (c *Client) GotoLayout(layout string) error {
	args := []string{"@"}
	if c.socketPath != "" {
		args = append(args, "--to", "unix:"+c.socketPath)
	}
	args = append(args, "goto-layout", layout)

	cmd := exec.Command("kitty", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kitty @ goto-layout: %w: %s", err, stderr.String())
	}
	return nil
}

// SetTabTitle sets the title of a tab by matching a window ID in that tab.
func (c *Client) SetTabTitle(windowID int, title string) error {
	args := []string{"@"}
	if c.socketPath != "" {
		args = append(args, "--to", "unix:"+c.socketPath)
	}
	args = append(args, "set-tab-title", "--match", fmt.Sprintf("id:%d", windowID), title)

	cmd := exec.Command("kitty", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kitty @ set-tab-title: %w: %s", err, stderr.String())
	}
	return nil
}

// FocusTab focuses a tab by matching a window ID in that tab.
func (c *Client) FocusTab(windowID int) error {
	args := []string{"@"}
	if c.socketPath != "" {
		args = append(args, "--to", "unix:"+c.socketPath)
	}
	args = append(args, "focus-tab", "--match", fmt.Sprintf("id:%d", windowID))

	cmd := exec.Command("kitty", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kitty @ focus-tab: %w: %s", err, stderr.String())
	}
	return nil
}

// FindFirstPinnedWindow returns the first window with PINNED user_var set.
// Returns nil if no pinned windows found.
func FindFirstPinnedWindow(state KittyState) *Window {
	for _, osWin := range state {
		for _, tab := range osWin.Tabs {
			for i := range tab.Windows {
				if tab.Windows[i].UserVars["PINNED"] != "" {
					return &tab.Windows[i]
				}
			}
		}
	}
	return nil
}

