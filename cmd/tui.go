package cmd

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/manager"
	"github.com/cwel/kmux/internal/model"
	"github.com/cwel/kmux/internal/state"
	"github.com/cwel/kmux/internal/store"
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
		if session == "" {
			return nil
		}
		return attachSession(s, session, "", "")
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
		return attachSession(s, name, path, result.LaunchLayout())
	case "kill":
		session := result.SelectedSession()
		if session == "" {
			return nil
		}
		return killSessionFromTUI(s, session)
	}

	return nil
}

// attachSession handles attach logic for TUI (mirrors cmd/attach.go logic)
func attachSession(s *state.State, name, cwd, layout string) error {
	// Check if session is already active
	windows, err := s.GetWindowsForSession(name)
	if err == nil && len(windows) > 0 {
		// Session is active - focus existing window
		s.KittyClient().FocusWindow(windows[0].ID)
		fmt.Printf("Focused existing session: %s\n", name)
		return nil
	}

	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Check if session has running zmx (detached)
	zmxSessions, _ := s.SessionZmxSessions(name)

	var session *model.Session

	if len(zmxSessions) > 0 {
		// Detached session - reattach to running zmx
		session, _ = s.Store().LoadSession(name)

		if session == nil {
			// No save file - create layout with windows for each zmx session
			var windows []model.Window
			for _, zmxName := range zmxSessions {
				windows = append(windows, model.Window{
					CWD:     cwd,
					ZmxName: zmxName,
				})
			}
			session = &model.Session{
				Name:    name,
				Host:    "local",
				SavedAt: time.Now(),
				Tabs: []model.Tab{
					{Title: name, Layout: "splits", Windows: windows},
				},
			}
		}
	} else if layout != "" {
		// New session with layout template
		layoutCfg, err := store.LoadLayout(layout)
		if err != nil {
			return fmt.Errorf("load layout: %w", err)
		}
		session = manager.LayoutToSession(layoutCfg, name, cwd)
	} else {
		// Try to load restore point, or create fresh
		session, _ = s.Store().LoadSession(name)
		if session == nil {
			session = &model.Session{
				Name:    name,
				Host:    "local",
				SavedAt: time.Now(),
				Tabs: []model.Tab{
					{Title: name, Layout: "splits", Windows: []model.Window{{CWD: cwd}}},
				},
			}
		}
	}

	// Clear ZmxSessions before rebuilding (RestoreTab populates it)
	session.ZmxSessions = nil

	// Create windows in kitty using RestoreTab
	kc := s.KittyClient()
	var firstWindowID int
	for tabIdx, tab := range session.Tabs {
		_, windowID, err := manager.RestoreTab(kc, session, tabIdx, tab)
		if err != nil {
			return fmt.Errorf("restore tab: %w", err)
		}
		if tabIdx == 0 && windowID > 0 {
			firstWindowID = windowID
		}
	}

	// Focus first window
	if firstWindowID > 0 {
		kc.FocusWindow(firstWindowID)
	}

	fmt.Printf("Attached to session: %s\n", name)
	return nil
}

// killSessionFromTUI kills a session (mirrors cmd/kill.go logic)
func killSessionFromTUI(s *state.State, name string) error {
	// Kill zmx sessions
	zmxSessions, _ := s.SessionZmxSessions(name)
	for _, zmxName := range zmxSessions {
		s.ZmxClient().Kill(zmxName)
	}

	// Close kitty windows
	windows, _ := s.GetWindowsForSession(name)
	for _, win := range windows {
		s.KittyClient().CloseWindow(win.ID)
	}

	// Delete save file
	s.Store().DeleteSession(name)

	fmt.Printf("Killed: %s\n", name)
	return nil
}
