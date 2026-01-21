package manager

import (
	"strings"
	"time"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/kitty"
	"github.com/cwel/kmux/internal/model"
)

// DeriveSession creates a Session from current kitty state.
// Uses kitty window user_vars as source of truth for session membership and zmx names.
func DeriveSession(name string, state kitty.KittyState) *model.Session {
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
			// Use user_vars as source of truth for session membership
			if win.UserVars["kmux_session"] != name {
				continue
			}
			idx := len(sessionWindows)
			windowIDToIdx[win.ID] = idx

			// Get zmx name from user_vars (source of truth)
			zmxName := win.UserVars["kmux_zmx"]

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

// LayoutToSession converts a layout template to a session.
func LayoutToSession(layout *config.Layout, name, cwd string) *model.Session {
	session := &model.Session{
		Name:    name,
		Host:    "local",
		SavedAt: time.Now(),
	}

	for _, ltab := range layout.Tabs {
		tab := model.Tab{
			Title:  ltab.Title,
			Layout: ltab.Layout,
		}

		for _, pane := range ltab.Panes {
			window := model.Window{
				CWD:     cwd,
				Command: pane,
			}
			tab.Windows = append(tab.Windows, window)
		}

		session.Tabs = append(session.Tabs, tab)
	}

	return session
}
