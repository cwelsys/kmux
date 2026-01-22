package cmd

import (
	"fmt"

	"github.com/cwel/kmux/internal/manager"
	"github.com/cwel/kmux/internal/state"
	"github.com/cwel/kmux/internal/store"
	"github.com/spf13/cobra"
)

var detachHost string

var detachCmd = &cobra.Command{
	Use:     "detach [session]",
	Aliases: []string{"d"},
	Short:   "Detach from a session",
	Long: `Save session state and close session windows.

If session name is provided, detaches that session.
Otherwise detects current session from the active kitty window.

Use --host to specify which host's session to detach (default: auto-detect or "local").`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s := state.New()
		k := s.KittyClient()
		st := s.Store()

		// Get current kitty state (needed for detection and closing)
		kittyState, err := k.GetState()
		if err != nil {
			return fmt.Errorf("get kitty state: %w", err)
		}

		var sessionName string
		host := detachHost

		if len(args) > 0 {
			sessionName = args[0]
		}

		// Auto-detect session and host from active window if not provided
		if sessionName == "" || host == "" {
			for _, osWin := range kittyState {
				if !osWin.IsActive {
					continue
				}
				for _, tab := range osWin.Tabs {
					if !tab.IsActive {
						continue
					}
					for _, win := range tab.Windows {
						if !win.IsActive {
							continue
						}
						if sessionName == "" {
							sessionName = win.UserVars["kmux_session"]
						}
						if host == "" {
							host = win.UserVars["kmux_host"]
						}
						break
					}
					break
				}
				break
			}
		}

		if host == "" {
			host = "local"
		}

		if sessionName == "" {
			return fmt.Errorf("session name required (provide as argument or run from within a session)")
		}

		if err := store.ValidateSessionName(sessionName); err != nil {
			return fmt.Errorf("invalid session name: %w", err)
		}

		// Derive session from current state using user_vars (filtered by host)
		session := manager.DeriveSession(sessionName, host, kittyState)

		// Save session
		if err := st.SaveSession(session); err != nil {
			return fmt.Errorf("save session: %w", err)
		}

		// Close windows belonging to this session AND host
		for _, osWin := range kittyState {
			for _, tab := range osWin.Tabs {
				for _, win := range tab.Windows {
					if win.UserVars["kmux_session"] != sessionName {
						continue
					}
					winHost := win.UserVars["kmux_host"]
					if winHost == "" {
						winHost = "local"
					}
					if winHost == host {
						k.CloseWindow(win.ID)
					}
				}
			}
		}

		if host != "local" {
			fmt.Printf("Detached from session: %s@%s\n", sessionName, host)
		} else {
			fmt.Printf("Detached from session: %s\n", sessionName)
		}
		return nil
	},
}

func init() {
	detachCmd.Flags().StringVarP(&detachHost, "host", "H", "", "remote host (SSH alias, default: auto-detect)")
	rootCmd.AddCommand(detachCmd)
}
