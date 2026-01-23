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
// On remote hosts (connected via kitten ssh), it falls back to `kitten @`
// which uses TTY-based DCS escape sequences instead of a unix socket.
type Client struct {
	socketPath string // Socket path from config, or empty to use kitty's default discovery
	useKitten  bool   // Use `kitten @` TTY-based remote control (for kitten ssh remotes)
	kittenPath string // Path to kitten binary (when useKitten is true)
}

// NewClient creates a new kitty client with no socket path.
// Use NewClientWithSocket to specify the socket from config.
func NewClient() *Client {
	return newClient("")
}

// NewClientWithSocket creates a client with an explicit socket path.
// The socket is resolved using environment and filesystem checks.
func NewClientWithSocket(socketPath string) *Client {
	return newClient(socketPath)
}

// newClient creates a client, falling back to kitten @ if no valid socket is available
// and we detect we're on a remote host via kitten ssh.
func newClient(socketPath string) *Client {
	resolved := resolveSocket(socketPath)

	// Check if the resolved socket is actually usable
	if hasValidSocket(resolved) {
		return &Client{socketPath: resolved}
	}

	// No valid socket — check if we're on a kitten ssh remote.
	// Detection uses kitty's own heuristic (shell-integration/zsh/kitty-integration):
	// KITTY_WINDOW_ID set + KITTY_PID not set = connected via kitten ssh.
	if os.Getenv("KITTY_WINDOW_ID") != "" && os.Getenv("KITTY_PID") == "" {
		if kittenPath, err := exec.LookPath("kitten"); err == nil {
			return &Client{useKitten: true, kittenPath: kittenPath}
		}
	}

	// Fallback: use socket as-is (will error from kitty if invalid)
	return &Client{socketPath: resolved}
}

// hasValidSocket checks if a resolved socket path is actually reachable.
func hasValidSocket(resolved string) bool {
	if os.Getenv("KITTY_LISTEN_ON") != "" {
		return true
	}
	if resolved != "" {
		if _, err := os.Stat(resolved); err == nil {
			return true
		}
	}
	return false
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

// wrapErr adds context-appropriate hints to kitty remote control errors.
func (c *Client) wrapErr(subcmd string, err error, stderr string) error {
	if c.useKitten {
		return fmt.Errorf("kitten @ %s: %w: %s\n(hint: ensure allow_remote_control is not 'socket-only' in kitty.conf)", subcmd, err, stderr)
	}
	return fmt.Errorf("kitty @ %s: %w: %s", subcmd, err, stderr)
}

// kittyCmd builds an exec.Cmd for a kitty remote control command.
// In kitten mode: kitten @ <args...>
// In socket mode: kitty @ [--to unix:<socket>] <args...>
func (c *Client) kittyCmd(args ...string) *exec.Cmd {
	if c.useKitten {
		fullArgs := append([]string{"@"}, args...)
		return exec.Command(c.kittenPath, fullArgs...)
	}

	fullArgs := []string{"@"}
	if c.socketPath != "" {
		fullArgs = append(fullArgs, "--to", "unix:"+c.socketPath)
	}
	fullArgs = append(fullArgs, args...)
	return exec.Command("kitty", fullArgs...)
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
	cmd := c.kittyCmd("ls")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, c.wrapErr("ls", err, stderr.String())
	}

	return ParseState(stdout.Bytes())
}

// Launch creates a new window/tab in kitty.
func (c *Client) Launch(opts LaunchOpts) (int, error) {
	args := []string{"launch"}

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

	cmd := c.kittyCmd(args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, c.wrapErr("launch", err, stderr.String())
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
	cmd := c.kittyCmd("focus-window", "--match", fmt.Sprintf("id:%d", id))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return c.wrapErr("focus-window", err, stderr.String())
	}
	return nil
}

// CloseWindow closes a window by ID.
func (c *Client) CloseWindow(id int) error {
	cmd := c.kittyCmd("close-window", "--match", fmt.Sprintf("id:%d", id))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return c.wrapErr("close-window", err, stderr.String())
	}
	return nil
}

// CloseTab closes a tab by ID.
func (c *Client) CloseTab(id int) error {
	cmd := c.kittyCmd("close-tab", "--match", fmt.Sprintf("id:%d", id))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return c.wrapErr("close-tab", err, stderr.String())
	}
	return nil
}

// GotoLayout changes the layout of the active tab.
func (c *Client) GotoLayout(layout string) error {
	cmd := c.kittyCmd("goto-layout", layout)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return c.wrapErr("goto-layout", err, stderr.String())
	}
	return nil
}

// SetTabTitle sets the title of a tab by matching a window ID in that tab.
func (c *Client) SetTabTitle(windowID int, title string) error {
	cmd := c.kittyCmd("set-tab-title", "--match", fmt.Sprintf("id:%d", windowID), title)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return c.wrapErr("set-tab-title", err, stderr.String())
	}
	return nil
}

// FocusTab focuses a tab by matching a window ID in that tab.
func (c *Client) FocusTab(windowID int) error {
	cmd := c.kittyCmd("focus-tab", "--match", fmt.Sprintf("id:%d", windowID))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return c.wrapErr("focus-tab", err, stderr.String())
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

