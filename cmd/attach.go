package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cwel/kmux/internal/kitty"
	"github.com/cwel/kmux/internal/manager"
	"github.com/cwel/kmux/internal/model"
	"github.com/cwel/kmux/internal/state"
	"github.com/cwel/kmux/internal/store"
	"github.com/spf13/cobra"
)

var (
	attachLayout string
	attachCWD    string
)

var attachCmd = &cobra.Command{
	Use:     "attach [name | path [name]]",
	Aliases: []string{"a"},
	Short:   "Attach to a session",
	Long: `Attach to an existing session or create a new one.

Examples:
  kmux a                    # session named after current directory
  kmux a myproject          # session named "myproject"
  kmux a ~/src/foo          # session "foo" starting in ~/src/foo
  kmux a ~/src/foo bar      # session "bar" starting in ~/src/foo`,
	Args: cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, cwd, err := resolveAttachArgs(args, attachCWD)
		if err != nil {
			return err
		}

		if err := store.ValidateSessionName(name); err != nil {
			return err
		}

		s := state.New()
		k := s.KittyClient()
		st := s.Store()

		// Check if session is already active
		windows, err := s.GetWindowsForSession(name)
		if err == nil && len(windows) > 0 {
			// Session is active - focus existing window
			k.FocusWindow(windows[0].ID)
			fmt.Printf("Focused existing session: %s\n", name)
			return nil
		}

		// Check if session has running zmx (detached)
		zmxSessions, _ := s.SessionZmxSessions(name)

		var session *model.Session

		if len(zmxSessions) > 0 {
			// Detached session - reattach to running zmx
			session, _ = st.LoadSession(name)

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
		} else if attachLayout != "" {
			// New session with layout template
			layout, err := store.LoadLayout(attachLayout)
			if err != nil {
				return fmt.Errorf("load layout: %w", err)
			}
			session = manager.LayoutToSession(layout, name, cwd)
		} else {
			// Try to load restore point, or create fresh
			session, _ = st.LoadSession(name)
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

		// Check for pinned tabs - new tabs should be created before them
		kittyState, _ := k.GetState()
		pinnedWindow := kitty.FindFirstPinnedWindow(kittyState)

		// Create windows in kitty using RestoreTab
		var firstWindowID int
		for tabIdx, tab := range session.Tabs {
			var opts manager.RestoreTabOpts

			// For the first tab, position before pinned tabs if any
			if tabIdx == 0 && pinnedWindow != nil {
				// Focus the pinned tab so new tab is created relative to it
				k.FocusTab(pinnedWindow.ID)
				opts.TabLocation = "before"
			}

			_, windowID, err := manager.RestoreTab(k, session, tabIdx, tab, opts)
			if err != nil {
				return fmt.Errorf("restore tab: %w", err)
			}
			if tabIdx == 0 && windowID > 0 {
				firstWindowID = windowID
			}
		}

		// Focus first window
		if firstWindowID > 0 {
			k.FocusWindow(firstWindowID)
		}

		fmt.Printf("Attached to session: %s\n", name)
		return nil
	},
}

// isPath returns true if the argument looks like a path (starts with /, ~, or .)
func isPath(arg string) bool {
	return strings.HasPrefix(arg, "/") ||
		strings.HasPrefix(arg, "~") ||
		strings.HasPrefix(arg, ".")
}

// expandPath expands ~ to home directory and resolves to absolute path.
func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[1:])
	}
	return filepath.Abs(path)
}

// resolveAttachArgs determines session name and cwd from command arguments.
// Args patterns:
//   - 0 args: name = cwd basename, cwd = current
//   - 1 arg (path): name = path basename, cwd = path
//   - 1 arg (name): name = arg, cwd = current
//   - 2 args: name = args[1], cwd = args[0] (path)
func resolveAttachArgs(args []string, cwdOverride string) (name, cwd string, err error) {
	// Start with current directory
	cwd, err = os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("get cwd: %w", err)
	}

	switch len(args) {
	case 0:
		// No args: derive name from cwd
		name = filepath.Base(cwd)

	case 1:
		if isPath(args[0]) {
			// Single path arg: derive name from path, use path as cwd
			cwd, err = expandPath(args[0])
			if err != nil {
				return "", "", fmt.Errorf("expand path: %w", err)
			}
			name = filepath.Base(cwd)
		} else {
			// Single name arg: use as session name
			name = args[0]
		}

	case 2:
		// Two args: path + name
		cwd, err = expandPath(args[0])
		if err != nil {
			return "", "", fmt.Errorf("expand path: %w", err)
		}
		name = args[1]
	}

	// Override cwd if flag provided
	if cwdOverride != "" {
		cwd, err = expandPath(cwdOverride)
		if err != nil {
			return "", "", fmt.Errorf("expand cwd override: %w", err)
		}
	}

	return name, cwd, nil
}

func init() {
	attachCmd.Flags().StringVarP(&attachLayout, "layout", "l", "", "create session from layout template")
	attachCmd.Flags().StringVarP(&attachCWD, "cwd", "C", "", "working directory for panes (overrides path)")
	rootCmd.AddCommand(attachCmd)
}
