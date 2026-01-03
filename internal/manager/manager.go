package manager

import (
	"strings"
	"time"

	"github.com/cwel/kmux/internal/kitty"
	"github.com/cwel/kmux/internal/model"
	"github.com/cwel/kmux/internal/store"
	"github.com/cwel/kmux/internal/zmx"
)

// Manager coordinates kitty, zmx, and session storage.
type Manager struct {
	kitty *kitty.Client
	zmx   *zmx.Client
	store *store.Store
}

// New creates a new Manager.
func New(k *kitty.Client, z *zmx.Client, s *store.Store) *Manager {
	return &Manager{kitty: k, zmx: z, store: s}
}

// DeriveSession creates a Session from current kitty state.
// mappings provides the authoritative kitty_window_id -> zmx_name lookup.
// windowSessions provides the authoritative kitty_window_id -> session_name lookup.
// Only captures windows that belong to this session.
func DeriveSession(name string, state kitty.KittyState, mappings map[int]string, windowSessions map[int]string) *model.Session {
	session := &model.Session{
		Name:    name,
		Host:    "local",
		SavedAt: time.Now(),
	}

	// Use first OS window (typically there's only one)
	if len(state) == 0 {
		return session
	}
	osWin := state[0]

	for _, tab := range osWin.Tabs {
		// Build window ID to index map for this tab
		windowIDToIdx := make(map[int]int)
		var sessionWindows []model.Window

		for _, win := range tab.Windows {
			if windowSessions[win.ID] != name {
				continue
			}
			idx := len(sessionWindows)
			windowIDToIdx[win.ID] = idx

			// Look up zmx name from authoritative mapping
			zmxName := mappings[win.ID]

			sessionWindows = append(sessionWindows, model.Window{
				CWD:     win.CWD,
				Command: extractCommand(win),
				ZmxName: zmxName,
			})
		}

		if len(sessionWindows) == 0 {
			continue
		}

		modelTab := model.Tab{
			Title:   tab.Title,
			Layout:  tab.Layout,
			Windows: sessionWindows,
		}

		// Parse split tree if this is a splits layout with multiple windows
		if tab.Layout == "splits" && len(sessionWindows) > 1 && tab.LayoutState.Pairs != nil {
			// Build group→window mapping from AllWindows
			groupToWindowID := tab.LayoutState.AllWindows.GroupToWindowID()
			if groupToWindowID != nil {
				splitRoot, err := kitty.PairToSplitNode(tab.LayoutState.Pairs, groupToWindowID, windowIDToIdx)
				if err == nil {
					modelTab.SplitRoot = splitRoot
				}
			}
		}

		session.Tabs = append(session.Tabs, modelTab)
	}

	return session
}

// extractCommand gets the foreground command, filtering out shells and zmx.
func extractCommand(win kitty.Window) string {
	if len(win.ForegroundProcesses) == 0 {
		return ""
	}

	fg := win.ForegroundProcesses[0]
	if len(fg.Cmdline) == 0 {
		return ""
	}

	// Filter out shells and zmx attach
	cmd := fg.Cmdline[0]
	if isShell(cmd) || cmd == "zmx" || strings.HasSuffix(cmd, "/zmx") {
		return ""
	}

	return strings.Join(fg.Cmdline, " ")
}

func isShell(cmd string) bool {
	shells := []string{"zsh", "bash", "fish", "sh", "/bin/zsh", "/bin/bash", "/bin/fish", "/bin/sh"}
	for _, s := range shells {
		if cmd == s || strings.HasSuffix(cmd, "/"+s) {
			return true
		}
	}
	return false
}
