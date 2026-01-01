package zmx

import (
	"bytes"
	"fmt"
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
func ParseList(output string) []string {
	output = strings.TrimSpace(output)
	if output == "" || strings.Contains(output, "no sessions found") {
		return nil
	}

	lines := strings.Split(output, "\n")
	var sessions []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			sessions = append(sessions, line)
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
func AttachCmd(sessionName string, cmd ...string) []string {
	if sessionName == "" {
		return nil
	}
	args := []string{"zmx", "attach", sessionName}
	if len(cmd) > 0 {
		args = append(args, cmd...)
	}
	return args
}
