// Package state provides stateless session queries from kitty + zmx + save files.
package state

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
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
	Host           string // "local" or SSH alias
	Status         string // "active", "detached", "saved"
	Panes          int
	IsRestorePoint bool
	CWD            string
	LastSeen       time.Time
}

// SessionResult holds the result of querying a host for sessions.
type SessionResult struct {
	Host     string
	Sessions []SessionInfo
	Error    error
}

// State provides stateless queries combining kitty, zmx, and save files.
type State struct {
	kitty     *kitty.Client
	localZmx  *zmx.Client
	remoteZmx map[string]*zmx.Client // SSH alias -> client
	store     *store.Store
	cfg       *config.Config
}

// New creates a new State with default clients.
func New() *State {
	cfg, _ := config.LoadConfig()
	socketPath := ""
	if cfg != nil && cfg.Kitty.Socket != "" {
		socketPath = cfg.Kitty.Socket
	}

	// Build remote zmx clients from config
	remoteZmx := make(map[string]*zmx.Client)
	if cfg != nil {
		for alias := range cfg.Hosts {
			hostCfg := cfg.GetHost(alias)
			remoteZmx[alias] = zmx.NewRemoteClient(alias, hostCfg)
		}
	}

	return &State{
		kitty:     kitty.NewClientWithSocket(socketPath),
		localZmx:  zmx.NewClient(),
		remoteZmx: remoteZmx,
		store:     store.DefaultStore(),
		cfg:       cfg,
	}
}

// ZmxClientForHost returns the zmx client for a given host.
// Returns the local client if host is "local" or empty.
func (s *State) ZmxClientForHost(host string) *zmx.Client {
	if host == "" || host == "local" {
		return s.localZmx
	}
	if client, ok := s.remoteZmx[host]; ok {
		return client
	}
	// Unknown host - create a new client on demand
	var hostCfg *config.HostConfig
	if s.cfg != nil {
		hostCfg = s.cfg.GetHost(host)
	}
	client := zmx.NewRemoteClient(host, hostCfg)
	s.remoteZmx[host] = client
	return client
}

// ConfiguredHosts returns the list of configured remote hosts.
func (s *State) ConfiguredHosts() []string {
	if s.cfg == nil {
		return nil
	}
	return s.cfg.HostNames()
}

// Sessions returns the list of all sessions (local only, synchronous).
// Logic:
// 1. Query kitty → group windows by user_vars["kmux_session"] → active sessions
// 2. Query zmx → find running zmx sessions not attached to kitty windows
// 3. For unattached zmx: check save files or derive from naming convention → detached sessions
// 4. If includeRestorePoints: add save files with no running zmx → saved sessions
func (s *State) Sessions(includeRestorePoints bool) ([]SessionInfo, error) {
	return s.sessionsForHost("local", includeRestorePoints)
}

// sessionsForHost returns sessions for a specific host.
func (s *State) sessionsForHost(host string, includeRestorePoints bool) ([]SessionInfo, error) {
	if s == nil {
		return nil, fmt.Errorf("state is nil")
	}
	if s.kitty == nil {
		return nil, fmt.Errorf("kitty client is nil")
	}

	zmxClient := s.ZmxClientForHost(host)
	if zmxClient == nil {
		return nil, fmt.Errorf("zmx client is nil")
	}

	// 1. Query kitty for active windows
	// Note: Remote sessions also have kitty windows locally (with kmux_host user_var)
	kittyState, kittyErr := s.kitty.GetState()

	// 2. Query zmx for running sessions
	zmxSessions, zmxErr := zmxClient.List()
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
					winHost := win.UserVars["kmux_host"]
					if winHost == "" {
						winHost = "local"
					}
					// Only count windows for this host
					if sessName != "" && winHost == host {
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
			Host:   host,
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
	saveFileHosts := make(map[string]string) // session name -> host from save file

	for _, savedName := range savedSessions {
		sess, err := s.store.LoadSession(savedName)
		if err != nil {
			continue
		}
		// Track the host this save file belongs to
		saveFileHosts[savedName] = sess.Host
		if saveFileHosts[savedName] == "" {
			saveFileHosts[savedName] = "local"
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
			Host:   host,
			Status: "detached",
			Panes:  panes,
			CWD:    cwd,
		})
		seenSessions[name] = true
	}

	// 4. Add restore points (save files with no running zmx)
	// Only include save files whose Host matches the requested host
	if includeRestorePoints {
		for _, savedName := range savedSessions {
			if seenSessions[savedName] {
				continue // already active or detached
			}
			savedHost := saveFileHosts[savedName]
			if savedHost != host {
				continue // save file is for a different host
			}
			sessions = append(sessions, SessionInfo{
				Name:           savedName,
				Host:           savedHost,
				Status:         "saved",
				Panes:          saveFilePanes[savedName],
				IsRestorePoint: true,
				CWD:            saveFileCWDs[savedName],
			})
		}
	}

	// Return error if both kitty and zmx failed (only relevant for local)
	if host == "local" && kittyErr != nil && zmxErr != nil {
		return nil, kittyErr
	}

	return sessions, zmxErr
}

// SessionsAsync returns a channel that receives session results as hosts respond.
// Local sessions are returned immediately, remote hosts are queried in parallel.
// The channel is closed when all hosts have responded or context is cancelled.
func (s *State) SessionsAsync(ctx context.Context, includeRestorePoints bool) <-chan SessionResult {
	results := make(chan SessionResult, 1+len(s.remoteZmx))

	go func() {
		defer close(results)

		// Get local sessions first (synchronous, should be fast)
		localSessions, err := s.sessionsForHost("local", includeRestorePoints)
		select {
		case results <- SessionResult{Host: "local", Sessions: localSessions, Error: err}:
		case <-ctx.Done():
			return
		}

		// Query remote hosts in parallel
		var wg sync.WaitGroup
		for alias := range s.remoteZmx {
			wg.Add(1)
			go func(host string) {
				defer wg.Done()

				sessions, err := s.sessionsForHost(host, false)
				select {
				case results <- SessionResult{Host: host, Sessions: sessions, Error: err}:
				case <-ctx.Done():
				}
			}(alias)
		}

		wg.Wait()
	}()

	return results
}

// AllSessions returns sessions from all hosts (blocks until all complete).
func (s *State) AllSessions(ctx context.Context, includeRestorePoints bool) ([]SessionInfo, error) {
	results := s.SessionsAsync(ctx, includeRestorePoints)

	var allSessions []SessionInfo
	var firstErr error

	for result := range results {
		if result.Error != nil && firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", result.Host, result.Error)
		}
		allSessions = append(allSessions, result.Sessions...)
	}

	return allSessions, firstErr
}

// FindWindowSession returns the session info for a kitty window.
func (s *State) FindWindowSession(windowID int) (*SessionInfo, string, string, error) {
	kittyState, err := s.kitty.GetState()
	if err != nil {
		return nil, "", "", err
	}

	// Find the window
	for _, osWin := range kittyState {
		for _, tab := range osWin.Tabs {
			for _, win := range tab.Windows {
				if win.ID == windowID {
					sessName := win.UserVars["kmux_session"]
					zmxName := win.UserVars["kmux_zmx"]
					host := win.UserVars["kmux_host"]
					if host == "" {
						host = "local"
					}
					if sessName == "" {
						return nil, "", "", nil // not a kmux window
					}

					// Count windows for this session on this host
					panes := 0
					cwd := ""
					for _, osWin2 := range kittyState {
						for _, tab2 := range osWin2.Tabs {
							for _, win2 := range tab2.Windows {
								winHost := win2.UserVars["kmux_host"]
								if winHost == "" {
									winHost = "local"
								}
								if win2.UserVars["kmux_session"] == sessName && winHost == host {
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
						Host:   host,
						Status: "active",
						Panes:  panes,
						CWD:    cwd,
					}, zmxName, host, nil
				}
			}
		}
	}

	return nil, "", "", nil // window not found
}

// GetCurrentSession returns the session for the current window (from KITTY_WINDOW_ID env).
func (s *State) GetCurrentSession() (*SessionInfo, string, string, error) {
	windowIDStr := os.Getenv("KITTY_WINDOW_ID")
	if windowIDStr == "" {
		return nil, "", "", nil
	}

	windowID, err := strconv.Atoi(windowIDStr)
	if err != nil {
		return nil, "", "", nil
	}

	return s.FindWindowSession(windowID)
}

// SessionZmxSessions returns the running zmx session names for a session.
func (s *State) SessionZmxSessions(name string) ([]string, error) {
	return s.SessionZmxSessionsForHost(name, "local")
}

// SessionZmxSessionsForHost returns the running zmx session names for a session on a specific host.
func (s *State) SessionZmxSessionsForHost(name, host string) ([]string, error) {
	zmxClient := s.ZmxClientForHost(host)
	zmxSessions, err := zmxClient.List()
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
	return s.GetWindowsForSessionOnHost(name, "local")
}

// GetWindowsForSessionOnHost returns all kitty windows belonging to a session on a specific host.
func (s *State) GetWindowsForSessionOnHost(name, host string) ([]kitty.Window, error) {
	kittyState, err := s.kitty.GetState()
	if err != nil {
		return nil, err
	}

	var windows []kitty.Window
	for _, osWin := range kittyState {
		for _, tab := range osWin.Tabs {
			for _, win := range tab.Windows {
				winHost := win.UserVars["kmux_host"]
				if winHost == "" {
					winHost = "local"
				}
				if win.UserVars["kmux_session"] == name && winHost == host {
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

// ZmxClient returns the local zmx client for direct operations.
func (s *State) ZmxClient() *zmx.Client {
	return s.localZmx
}

// Store returns the store for direct operations.
func (s *State) Store() *store.Store {
	return s.store
}

// Config returns the config for direct operations.
func (s *State) Config() *config.Config {
	return s.cfg
}
