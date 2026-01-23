package manager

import (
	"time"

	"github.com/cwel/kmux/internal/kitty"
	"github.com/cwel/kmux/internal/model"
	"github.com/cwel/kmux/internal/state"
	"github.com/cwel/kmux/internal/store"
)

// AttachOpts holds options for AttachSession.
type AttachOpts struct {
	Name         string // Session name (required)
	Host         string // "local" or SSH alias (defaults to "local")
	CWD          string // Working directory for new sessions
	Layout       string // Layout template name (optional)
	BeforePinned bool   // Position new tabs before pinned tabs
}

// AttachResult holds the result of an attach operation.
type AttachResult struct {
	Action      string // "focused", "reattached", "created"
	SessionName string
	Host        string
	WindowID    int
}

// AttachSession attaches to or creates a session.
// Returns the result of the operation or an error.
func AttachSession(s *state.State, opts AttachOpts) (*AttachResult, error) {
	host := opts.Host
	if host == "" {
		host = "local"
	}

	k := s.KittyClient()
	zmxClient := s.ZmxClientForHost(host)

	// Check if session is already active (on this host)
	windows, err := s.GetWindowsForSessionOnHost(opts.Name, host)
	if err == nil && len(windows) > 0 {
		// Session is active - focus existing window
		k.FocusWindow(windows[0].ID)
		return &AttachResult{
			Action:      "focused",
			SessionName: opts.Name,
			Host:        host,
			WindowID:    windows[0].ID,
		}, nil
	}

	// Check if session has running zmx (detached)
	zmxSessions, _ := s.SessionZmxSessionsForHost(opts.Name, host)

	var session *model.Session

	if len(zmxSessions) > 0 {
		// Detached session - reattach to running zmx
		session = loadSessionFromHost(s, opts.Name, host)

		if session == nil {
			// No save file (or wrong host) - create layout with windows for each zmx session
			var modelWindows []model.Window
			for _, zmxName := range zmxSessions {
				modelWindows = append(modelWindows, model.Window{
					CWD:     opts.CWD,
					ZmxName: zmxName,
				})
			}
			session = &model.Session{
				Name:    opts.Name,
				Host:    host,
				SavedAt: time.Now(),
				Tabs: []model.Tab{
					{Title: opts.Name, Layout: "splits", Windows: modelWindows},
				},
			}
		}
	} else if opts.Layout != "" {
		// New session with layout template
		layout, err := store.LoadLayout(opts.Layout)
		if err != nil {
			return nil, err
		}
		session = LayoutToSession(layout, opts.Name, opts.CWD)
		session.Host = host
	} else {
		// Try to load restore point, or create fresh
		session = loadSessionFromHost(s, opts.Name, host)
		if session == nil {
			session = &model.Session{
				Name:    opts.Name,
				Host:    host,
				SavedAt: time.Now(),
				Tabs: []model.Tab{
					{Title: opts.Name, Layout: "splits", Windows: []model.Window{{CWD: opts.CWD}}},
				},
			}
		}
	}

	// Clear ZmxSessions before rebuilding (RestoreTab populates it)
	session.ZmxSessions = nil

	// Check for pinned tabs - new tabs should be created before them
	var pinnedWindow *kitty.Window
	if opts.BeforePinned {
		kittyState, _ := k.GetState()
		pinnedWindow = kitty.FindFirstPinnedWindow(kittyState)
	}

	// Create windows in kitty using RestoreTab
	var firstWindowID int
	for tabIdx, tab := range session.Tabs {
		restoreOpts := RestoreTabOpts{
			ZmxClient: zmxClient,
			Host:      host,
		}

		// For the first tab, position before pinned tabs if any
		if tabIdx == 0 && pinnedWindow != nil {
			// Focus the pinned tab so new tab is created relative to it
			k.FocusTab(pinnedWindow.ID)
			restoreOpts.TabLocation = "before"
		}

		_, windowID, err := RestoreTab(k, session, tabIdx, tab, restoreOpts)
		if err != nil {
			return nil, err
		}
		if tabIdx == 0 && windowID > 0 {
			firstWindowID = windowID
		}
	}

	// Focus first window
	if firstWindowID > 0 {
		k.FocusWindow(firstWindowID)
	}

	action := "created"
	if len(zmxSessions) > 0 {
		action = "reattached"
	}

	return &AttachResult{
		Action:      action,
		SessionName: opts.Name,
		Host:        host,
		WindowID:    firstWindowID,
	}, nil
}

// KillOpts holds options for KillSession.
type KillOpts struct {
	Name string // Session name (required)
	Host string // "local" or SSH alias (defaults to "local")
}

// KillSession terminates a session completely.
// Comprehensively collects zmx from: save file, naming convention, kitty user_vars.
func KillSession(s *state.State, opts KillOpts) error {
	host := opts.Host
	if host == "" {
		host = "local"
	}

	k := s.KittyClient()
	zmxClient := s.ZmxClientForHost(host)
	st := s.Store()

	// Collect zmx sessions to kill from save file and naming convention
	zmxToKill := make(map[string]bool)

	// Check save file first
	if sess, err := st.LoadSession(opts.Name); err == nil {
		for _, zmxName := range sess.ZmxSessions {
			zmxToKill[zmxName] = true
		}
		for _, tab := range sess.Tabs {
			for _, win := range tab.Windows {
				if win.ZmxName != "" {
					zmxToKill[win.ZmxName] = true
				}
			}
		}
	}

	// Query zmx and find sessions matching naming convention
	zmxSessions, _ := zmxClient.List()
	for _, zmxName := range zmxSessions {
		if model.ParseZmxSessionName(zmxName) == opts.Name {
			zmxToKill[zmxName] = true
		}
	}

	// Get kitty state to find windows for this session
	kittyState, _ := k.GetState()

	// Close windows (filtered by host) and collect any zmx names from user_vars
	for _, osWin := range kittyState {
		for _, tab := range osWin.Tabs {
			for _, win := range tab.Windows {
				if win.UserVars["kmux_session"] != opts.Name {
					continue
				}
				winHost := win.UserVars["kmux_host"]
				if winHost == "" {
					winHost = "local"
				}
				if winHost != host {
					continue
				}
				// Add zmx name if present
				if zmxName := win.UserVars["kmux_zmx"]; zmxName != "" {
					zmxToKill[zmxName] = true
				}
				// Close the kitty window
				k.CloseWindow(win.ID)
			}
		}
	}

	// Kill all zmx sessions for this session (using the correct host's zmx client)
	for zmxName := range zmxToKill {
		zmxClient.Kill(zmxName)
	}

	// Delete saved session (save files are always stored locally)
	st.DeleteSession(opts.Name)

	return nil
}

// loadSessionFromHost loads a session from the appropriate host.
// For local: reads local store. For remote: fetches via SSH.
func loadSessionFromHost(s *state.State, name, host string) *model.Session {
	if host == "local" {
		return loadSessionForHost(s.Store(), name, host)
	}

	client := s.RemoteKmuxClient(host)
	if client == nil {
		return nil
	}

	session, err := client.GetSession(name)
	if err != nil {
		return nil
	}

	return session
}

// loadSessionForHost loads a save file only if its Host matches the target host.
// Returns nil if no save file exists or if the save file is for a different host.
func loadSessionForHost(st *store.Store, name, host string) *model.Session {
	session, err := st.LoadSession(name)
	if err != nil || session == nil {
		return nil
	}

	savedHost := session.Host
	if savedHost == "" {
		savedHost = "local"
	}
	if savedHost != host {
		return nil
	}

	return session
}
