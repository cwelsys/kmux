package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/client"
	"github.com/cwel/kmux/internal/tui"
)

func runTUI() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	c := client.New(config.SocketPath())

	if err := c.EnsureRunning(); err != nil {
		return fmt.Errorf("daemon: %w", err)
	}

	m := tui.New(c, cfg)
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
		if session == "" {
			return nil
		}
		return c.Attach(session, "", "")
	case "create":
		// Determine path and name - either from browser or project selection
		var path, name string

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

		// Create session with name, cwd, and optional layout
		return c.Attach(name, path, result.LaunchLayout())
	case "kill":
		session := result.SelectedSession()
		if session == "" {
			return nil
		}
		return c.Kill(session)
	}

	return nil
}
