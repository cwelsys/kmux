package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/manager"
	"github.com/cwel/kmux/internal/state"
	"github.com/cwel/kmux/internal/tui"
)

func runTUI() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	s := state.New()

	m := tui.New(s, cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("run TUI: %w", err)
	}

	result := finalModel.(tui.Model)
	action := result.Action()

	if action == "" {
		return nil
	}

	switch action {
	case "attach":
		session := result.SelectedSession()
		host := result.SelectedSessionHost()
		if session == "" {
			return nil
		}
		return attachSessionWithHost(s, session, "", "", host)
	case "create":
		// Determine path and name - either from browser or project selection
		var path, name string
		host := result.SelectedHost()

		if browserPath := result.BrowserPath(); browserPath != "" {
			// From file browser
			path = browserPath
			name = result.LaunchName()
		} else if project := result.SelectedProject(); project != nil {
			// From project list
			path = project.Path
			name = result.LaunchName()
			if name == "" {
				name = project.Name
			}
		} else {
			return nil
		}

		if name == "" {
			return nil
		}

		// Create session with name, cwd, optional layout, and host
		return attachSessionWithHost(s, name, path, result.LaunchLayout(), host)
	case "kill":
		session := result.SelectedSession()
		host := result.SelectedSessionHost()
		if session == "" {
			return nil
		}
		return killSessionWithHost(s, session, host)
	}

	return nil
}

// attachSessionWithHost handles attach logic for TUI with host support
func attachSessionWithHost(s *state.State, name, cwd, layout, host string) error {
	result, err := manager.AttachSession(s, manager.AttachOpts{
		Name:   name,
		Host:   host,
		CWD:    cwd,
		Layout: layout,
	})
	if err != nil {
		return err
	}

	// Print result
	switch result.Action {
	case "focused":
		fmt.Printf("Focused existing session: %s\n", result.SessionName)
	default:
		if result.Host != "local" {
			fmt.Printf("Attached to session: %s@%s\n", result.SessionName, result.Host)
		} else {
			fmt.Printf("Attached to session: %s\n", result.SessionName)
		}
	}
	return nil
}

// killSessionWithHost kills a session on a specific host
func killSessionWithHost(s *state.State, name, host string) error {
	if err := manager.KillSession(s, manager.KillOpts{Name: name, Host: host}); err != nil {
		return err
	}

	if host != "" && host != "local" {
		fmt.Printf("Killed: %s@%s\n", name, host)
	} else {
		fmt.Printf("Killed: %s\n", name)
	}
	return nil
}
