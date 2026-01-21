// Package state provides stateless session queries from kitty + zmx + save files.
package state

import (
	"fmt"
	"os"
	"time"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/kitty"
	"github.com/cwel/kmux/internal/model"
	"github.com/cwel/kmux/internal/store"
	"github.com/cwel/kmux/internal/zmx"
)

// SessionInfo represents a session's current state.
type SessionInfo struct {
	Name           string
	Status         string // "active", "detached", "saved"
	Panes          int
	IsRestorePoint bool
	CWD            string
	LastSeen       time.Time
}

// State provides stateless queries combining kitty, zmx, and save files.
type State struct {
	kitty *kitty.Client
	zmx   *zmx.Client
	store *store.Store
}

// New creates a new State with default clients.
func New() *State {
	cfg, _ := config.LoadConfig()
	socketPath := ""
	if cfg != nil && cfg.Kitty.Socket != "" {
		socketPath = cfg.Kitty.Socket
	}
	return &State{
		kitty: kitty.NewClientWithSocket(socketPath),
		zmx:   zmx.NewClient(),
		store: store.DefaultStore(),
	}
}

// Sessions returns the list of all sessions.
// Logic:
// 1. Query kitty → group windows by user_vars["kmux_session"] → active sessions
// 2. Query zmx → find running zmx sessions not attached to kitty windows
// 3. For unattached zmx: check save files or derive from naming convention → detached sessions
// 4. If includeRestorePoints: add save files with no running zmx → saved sessions
func (s *State) Sessions(includeRestorePoints bool) ([]SessionInfo, error) {
	if s == nil {
		return nil, fmt.Errorf("state is nil")
	}
	if s.kitty == nil {
		return nil, fmt.Errorf("kitty client is nil")
	}
	if s.zmx == nil {
		return nil, fmt.Errorf("zmx client is nil")
	}

	// 1. Query kitty for active windows
	kittyState, kittyErr := s.kitty.GetState()

	// 2. Query zmx for running sessions
	zmxSessions, zmxErr := s.zmx.List()
	zmxSet := make(map[string]bool)
	for _, z := range zmxSessions {
		zmxSet[z] = true
	}

	// Build maps for active sessions from kitty
	// sessionWindows: session name -> window IDs
	// attachedZmx: zmx names that are attached to kitty windows
	sessionWindows := make(map[string][]int)
	sessionCWDs := make(map[string]string)
	attachedZmx := make(map[string]bool)

	if kittyErr == nil {
		for _, osWin := range kittyState {
			for _, tab := range osWin.Tabs {
				for _, win := range tab.Windows {
					sessName := win.UserVars["kmux_session"]
					zmxName := win.UserVars["kmux_zmx"]
					if sessName != "" {
						sessionWindows[sessName] = append(sessionWindows[sessName], win.ID)
						if sessionCWDs[sessName] == "" {
							sessionCWDs[sessName] = win.CWD
						}
						if zmxName != "" {
							attachedZmx[zmxName] = true
						}
					}
				}
			}
		}
	}

	// Build result
	var sessions []SessionInfo
	seenSessions := make(map[string]bool)

	// Active sessions (have kitty windows)
	for name, windowIDs := range sessionWindows {
		sessions = append(sessions, SessionInfo{
			Name:   name,
			Status: "active",
			Panes:  len(windowIDs),
			CWD:    sessionCWDs[name],
		})
		seenSessions[name] = true
	}

	// 3. Find detached sessions (zmx running but no kitty windows)
	// First, load all save files to check zmx→session mappings
	saveFilesByZmx := make(map[string]string) // zmx name -> session name from save file
	savedSessions, _ := s.store.ListSessions()
	saveFilePanes := make(map[string]int)
	saveFileCWDs := make(map[string]string)

	for _, savedName := range savedSessions {
		sess, err := s.store.LoadSession(savedName)
		if err != nil {
			continue
		}
		// Map zmx sessions to this save file's session name
		for _, zmxName := range sess.ZmxSessions {
			saveFilesByZmx[zmxName] = savedName
		}
		// Also map individual window zmx names
		panes := 0
		for _, tab := range sess.Tabs {
			for _, win := range tab.Windows {
				if win.ZmxName != "" {
					saveFilesByZmx[win.ZmxName] = savedName
				}
				panes++
				if saveFileCWDs[savedName] == "" {
					saveFileCWDs[savedName] = win.CWD
				}
			}
		}
		saveFilePanes[savedName] = panes
	}

	// Find zmx sessions not attached to kitty windows -> detached
	detachedBySession := make(map[string]int) // session name -> pane count
	for _, zmxName := range zmxSessions {
		if attachedZmx[zmxName] {
			continue // attached to a kitty window
		}

		// Determine session name: check save files first, then ownership file, then derive from naming
		sessName := saveFilesByZmx[zmxName]
		if sessName == "" {
			sessName = store.GetSessionForZmx(zmxName)
		}
		if sessName == "" {
			sessName = model.ParseZmxSessionName(zmxName)
		}
		if sessName == "" {
			continue // unknown zmx session, ignore
		}

		if seenSessions[sessName] {
			continue // already listed as active
		}

		detachedBySession[sessName]++
	}

	// Add detached sessions
	for name, panes := range detachedBySession {
		cwd := saveFileCWDs[name]
		sessions = append(sessions, SessionInfo{
			Name:   name,
			Status: "detached",
			Panes:  panes,
			CWD:    cwd,
		})
		seenSessions[name] = true
	}

	// 4. Add restore points (save files with no running zmx)
	if includeRestorePoints {
		for _, savedName := range savedSessions {
			if seenSessions[savedName] {
				continue // already active or detached
			}
			sessions = append(sessions, SessionInfo{
				Name:           savedName,
				Status:         "saved",
				Panes:          saveFilePanes[savedName],
				IsRestorePoint: true,
				CWD:            saveFileCWDs[savedName],
			})
		}
	}

	// Return error if both kitty and zmx failed
	if kittyErr != nil && zmxErr != nil {
		return nil, kittyErr
	}

	return sessions, nil
}

// FindWindowSession returns the session info for a kitty window.
func (s *State) FindWindowSession(windowID int) (*SessionInfo, string, error) {
	kittyState, err := s.kitty.GetState()
	if err != nil {
		return nil, "", err
	}

	// Find the window
	for _, osWin := range kittyState {
		for _, tab := range osWin.Tabs {
			for _, win := range tab.Windows {
				if win.ID == windowID {
					sessName := win.UserVars["kmux_session"]
					zmxName := win.UserVars["kmux_zmx"]
					if sessName == "" {
						return nil, "", nil // not a kmux window
					}

					// Count windows for this session
					panes := 0
					cwd := ""
					for _, osWin2 := range kittyState {
						for _, tab2 := range osWin2.Tabs {
							for _, win2 := range tab2.Windows {
								if win2.UserVars["kmux_session"] == sessName {
									panes++
									if cwd == "" {
										cwd = win2.CWD
									}
								}
							}
						}
					}

					return &SessionInfo{
						Name:   sessName,
						Status: "active",
						Panes:  panes,
						CWD:    cwd,
					}, zmxName, nil
				}
			}
		}
	}

	return nil, "", nil // window not found
}

// GetCurrentSession returns the session for the current window (from KITTY_WINDOW_ID env).
func (s *State) GetCurrentSession() (*SessionInfo, string, error) {
	windowIDStr := os.Getenv("KITTY_WINDOW_ID")
	if windowIDStr == "" {
		return nil, "", nil
	}

	windowID, err := parseWindowID(windowIDStr)
	if err != nil {
		return nil, "", nil
	}

	return s.FindWindowSession(windowID)
}

// parseWindowID parses a window ID string into an int.
func parseWindowID(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, os.ErrInvalid
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// SessionZmxSessions returns the running zmx session names for a session.
func (s *State) SessionZmxSessions(name string) ([]string, error) {
	zmxSessions, err := s.zmx.List()
	if err != nil {
		return nil, err
	}

	// Check save file first for the canonical list
	sess, err := s.store.LoadSession(name)
	if err == nil && len(sess.ZmxSessions) > 0 {
		// Filter to only running zmx sessions
		zmxSet := make(map[string]bool)
		for _, z := range zmxSessions {
			zmxSet[z] = true
		}

		var running []string
		for _, zmxName := range sess.ZmxSessions {
			if zmxSet[zmxName] {
				running = append(running, zmxName)
			}
		}
		return running, nil
	}

	// No save file or no ZmxSessions - check ownership file and naming convention
	var matches []string
	for _, zmxName := range zmxSessions {
		// Check ownership file first
		if store.GetSessionForZmx(zmxName) == name {
			matches = append(matches, zmxName)
			continue
		}
		// Fall back to naming convention
		if model.ParseZmxSessionName(zmxName) == name {
			matches = append(matches, zmxName)
		}
	}
	return matches, nil
}

// GetWindowsForSession returns all kitty windows belonging to a session.
func (s *State) GetWindowsForSession(name string) ([]kitty.Window, error) {
	kittyState, err := s.kitty.GetState()
	if err != nil {
		return nil, err
	}

	var windows []kitty.Window
	for _, osWin := range kittyState {
		for _, tab := range osWin.Tabs {
			for _, win := range tab.Windows {
				if win.UserVars["kmux_session"] == name {
					windows = append(windows, win)
				}
			}
		}
	}
	return windows, nil
}

// KittyClient returns the kitty client for direct operations.
func (s *State) KittyClient() *kitty.Client {
	return s.kitty
}

// ZmxClient returns the zmx client for direct operations.
func (s *State) ZmxClient() *zmx.Client {
	return s.zmx
}

// Store returns the store for direct operations.
func (s *State) Store() *store.Store {
	return s.store
}
