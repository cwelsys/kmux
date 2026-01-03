package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/client"
	"github.com/cwel/kmux/internal/tui"
)

func runTUI() error {
	c := client.New(config.SocketPath())

	if err := c.EnsureRunning(); err != nil {
		return fmt.Errorf("daemon: %w", err)
	}

	m := tui.New(c)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("run TUI: %w", err)
	}

	result := finalModel.(tui.Model)
	session := result.SelectedSession()
	action := result.Action()

	if session == "" || action == "" {
		return nil
	}

	switch action {
	case "attach":
		return c.Attach(session, "", "")
	case "kill":
		return c.Kill(session)
	}

	return nil
}
