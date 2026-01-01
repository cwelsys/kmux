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
		modelTab := model.Tab{
			Title:  tab.Title,
			Layout: tab.Layout,
		}

		for _, win := range tab.Windows {
			modelWin := model.Window{
				CWD:     win.CWD,
				Command: extractCommand(win),
			}
			modelTab.Windows = append(modelTab.Windows, modelWin)
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
	if isShell(cmd) || cmd == "zmx" {
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
