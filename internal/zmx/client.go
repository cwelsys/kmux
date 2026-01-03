package zmx

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Client communicates with zmx CLI.
type Client struct{}

// NewClient creates a new zmx client.
func NewClient() *Client {
	return &Client{}
}

// ParseList parses output from `zmx list`.
// Format: session_name=NAME\tpid=PID\tclients=N
// Sessions with status=Timeout (cleaning up) are filtered out.
func ParseList(output string) []string {
	output = strings.TrimSpace(output)
	if output == "" || strings.Contains(output, "no sessions found") {
		return nil
	}

	lines := strings.Split(output, "\n")
	var sessions []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip sessions that are cleaning up
		if strings.Contains(line, "cleaning up") {
			continue
		}
		// Extract session name from "session_name=NAME\t..."
		if strings.HasPrefix(line, "session_name=") {
			parts := strings.Split(line, "\t")
			if len(parts) > 0 {
				name := strings.TrimPrefix(parts[0], "session_name=")
				if name != "" {
					sessions = append(sessions, name)
				}
			}
		}
	}
	return sessions
}

// List returns all active zmx sessions.
func (c *Client) List() ([]string, error) {
	cmd := exec.Command("zmx", "list")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// zmx list returns error if no sessions, check stderr
		errStr := stderr.String()
		if strings.Contains(errStr, "no sessions found") {
			return nil, nil
		}
		return nil, fmt.Errorf("zmx list: %w: %s", err, errStr)
	}

	return ParseList(stdout.String()), nil
}

// Kill terminates a zmx session.
func (c *Client) Kill(name string) error {
	if name == "" {
		return fmt.Errorf("zmx kill: session name is required")
	}
	cmd := exec.Command("zmx", "kill", name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("zmx kill %s: %w: %s", name, err, stderr.String())
	}
	return nil
}

// AttachCmd returns the command to attach to a zmx session.
// This is used to construct the command passed to kitty launch.
// If kmuxSession is provided, includes a cleanup callback to notify daemon on exit.
func AttachCmd(zmxName string, kmuxSession string, cmd ...string) []string {
	if zmxName == "" {
		return nil
	}

	// Build the zmx attach command
	zmxCmd := "zmx attach " + zmxName

	// Add command through interactive shell (loads user's PATH)
	for _, c := range cmd {
		if c != "" {
			shell := os.Getenv("SHELL")
			if shell == "" {
				shell = "/bin/sh"
			}
			zmxCmd += " " + shell + " -ic " + shellQuote(c)
			break // only one command supported
		}
	}

	// If kmux session provided, wrap with notify-close callback
	if kmuxSession != "" {
		// After zmx exits, notify daemon. Use $KITTY_WINDOW_ID set by kitty.
		// Must use interactive shell to get user's PATH for zmx and kmux
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		fullCmd := zmxCmd + "; kmux internal notify-close $KITTY_WINDOW_ID " + zmxName + " " + kmuxSession
		return []string{shell, "-ic", fullCmd}
	}

	// No session context - just run zmx attach directly
	args := []string{"zmx", "attach", zmxName}
	for _, c := range cmd {
		if c != "" {
			shell := os.Getenv("SHELL")
			if shell == "" {
				shell = "/bin/sh"
			}
			args = append(args, shell, "-ic", c)
			break
		}
	}
	return args
}

// shellQuote returns a shell-safe quoted string.
func shellQuote(s string) string {
	// Use single quotes, escaping any single quotes within
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
