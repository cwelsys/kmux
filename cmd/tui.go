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
		project := result.SelectedProject()
		if project == nil {
			return nil
		}
		// Use custom name if provided, otherwise project name
		name := result.LaunchName()
		if name == "" {
			name = project.Name
		}
		// Create session from project with name, cwd, and optional layout
		return c.Attach(name, project.Path, result.LaunchLayout())
	case "kill":
		session := result.SelectedSession()
		if session == "" {
			return nil
		}
		return c.Kill(session)
	}

	return nil
}
