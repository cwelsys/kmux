package zmx

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/cwel/kmux/internal/config"
)

// Client communicates with zmx CLI, either locally or over SSH.
type Client struct {
	host    string             // SSH alias or "local"
	hostCfg *config.HostConfig // nil for local
}

// NewClient creates a local zmx client.
func NewClient() *Client {
	return &Client{host: "local"}
}

// NewRemoteClient creates a zmx client that executes commands over SSH.
func NewRemoteClient(sshAlias string, cfg *config.HostConfig) *Client {
	return &Client{
		host:    sshAlias,
		hostCfg: cfg,
	}
}

// IsRemote returns true if this client connects to a remote host.
func (c *Client) IsRemote() bool {
	return c.host != "local"
}

// Host returns the host this client connects to ("local" or SSH alias).
func (c *Client) Host() string {
	return c.host
}

// zmxPath returns the path to zmx binary (for remote, may be overridden in config).
func (c *Client) zmxPath() string {
	if c.hostCfg != nil && c.hostCfg.ZmxPath != "" {
		return c.hostCfg.ZmxPath
	}
	return "zmx"
}

// runZmx runs a zmx command, either locally or over SSH.
func (c *Client) runZmx(args ...string) *exec.Cmd {
	if c.IsRemote() {
		// Build SSH command: ssh <alias> "zmx <args>"
		zmxCmd := c.zmxPath() + " " + strings.Join(args, " ")
		return exec.Command("ssh", c.host, zmxCmd)
	}

	// Local: run through login shell to ensure proper PATH
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	shellCmd := "zmx " + strings.Join(args, " ")
	return exec.Command(shell, "-lc", shellCmd)
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
	cmd := c.runZmx("list")
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
	cmd := c.runZmx("kill", name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("zmx kill %s: %w: %s", name, err, stderr.String())
	}
	return nil
}

// AttachCmd returns the command to attach to a zmx session.
// For local: ["zmx", "attach", name, ...]
// For remote: ["kitten", "ssh", host, "-t", "zmx", "attach", name, ...]
func (c *Client) AttachCmd(zmxName string, cmd ...string) []string {
	if zmxName == "" {
		return nil
	}

	zmxPath := c.zmxPath()

	if c.IsRemote() {
		// Remote: use kitten ssh to connect and run zmx
		args := []string{"kitten", "ssh", c.host, "-t", zmxPath, "attach", zmxName}

		// Add command through shell if provided
		for _, cm := range cmd {
			if cm != "" {
				// On remote, use default shell (ssh provides login shell)
				args = append(args, "--", "sh", "-ic", cm)
				break
			}
		}
		return args
	}

	// Local: direct zmx command
	args := []string{zmxPath, "attach", zmxName}

	// Add command through interactive shell (loads user's PATH)
	for _, cm := range cmd {
		if cm != "" {
			shell := os.Getenv("SHELL")
			if shell == "" {
				shell = "/bin/sh"
			}
			args = append(args, shell, "-ic", cm)
			break // only one command supported
		}
	}

	return args
}
