package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cwel/kmux/internal/tui"
)

func runTUI() error {
	m := tui.New()
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("run TUI: %w", err)
	}

	// Handle action after TUI exits
	result := finalModel.(tui.Model)
	session := result.SelectedSession()
	action := result.Action()

	if session == "" || action == "" {
		return nil
	}

	switch action {
	case "attach":
		return runAttach(session)
	case "kill":
		return runKill(session)
	}

	return nil
}

func runAttach(name string) error {
	// Re-run attach logic (same as attachCmd.RunE)
	// This avoids circular dependency - we call the command directly
	args := []string{name}
	attachCmd.SetArgs(args)
	return attachCmd.RunE(attachCmd, args)
}

func runKill(name string) error {
	args := []string{name}
	killCmd.SetArgs(args)
	return killCmd.RunE(killCmd, args)
}
