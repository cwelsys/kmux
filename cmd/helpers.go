package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cwel/kmux/internal/state"
)

// autoDetectSessionHost finds which host has a session with the given name.
// Returns:
// - The host if session exists on exactly one host
// - User's choice via fzf if session exists on multiple hosts
// - "local" if session doesn't exist anywhere (will create new)
func autoDetectSessionHost(s *state.State, name string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Query all hosts for sessions
	allSessions, _ := s.AllSessions(ctx, true)

	// Find which hosts have this session
	var hostsWithSession []string
	for _, sess := range allSessions {
		if sess.Name == name {
			hostsWithSession = append(hostsWithSession, sess.Host)
		}
	}

	switch len(hostsWithSession) {
	case 0:
		return "local"
	case 1:
		return hostsWithSession[0]
	default:
		// Multiple hosts - use fzf to pick
		return pickHostWithFzf(name, hostsWithSession)
	}
}

// pickHostWithFzf prompts user to select a host using fzf.
func pickHostWithFzf(sessionName string, hosts []string) string {
	input := strings.Join(hosts, "\n")
	height := fmt.Sprintf("%d", len(hosts)+2) // just enough for entries + prompt
	cmd := exec.Command("fzf",
		"--height", height,
		"--no-info",
		"--prompt", fmt.Sprintf("%s @ ", sessionName))
	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		// fzf cancelled or not available - default to first host
		fmt.Printf("Multiple hosts have session '%s': %s\n", sessionName, strings.Join(hosts, ", "))
		fmt.Printf("Using: %s (specify with --host to override)\n", hosts[0])
		return hosts[0]
	}

	return strings.TrimSpace(string(output))
}
