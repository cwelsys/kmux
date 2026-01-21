package cmd

import (
	"fmt"

	"github.com/cwel/kmux/internal/manager"
	"github.com/cwel/kmux/internal/state"
	"github.com/cwel/kmux/internal/store"
	"github.com/spf13/cobra"
)

var detachCmd = &cobra.Command{
	Use:     "detach [session]",
	Aliases: []string{"d"},
	Short:   "Detach from a session",
	Long: `Save session state and close session windows.

If session name is provided, detaches that session.
Otherwise uses $KITTY_WINDOW_ID to determine current session.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s := state.New()
		k := s.KittyClient()
		st := s.Store()

		var sessionName string
		if len(args) > 0 {
			sessionName = args[0]
		} else {
			// Try to resolve session from current window
			if sessInfo, _, err := s.GetCurrentSession(); err == nil && sessInfo != nil {
				sessionName = sessInfo.Name
			}
		}

		if sessionName == "" {
			return fmt.Errorf("session name required (provide as argument or run from within a session)")
		}

		if err := store.ValidateSessionName(sessionName); err != nil {
			return fmt.Errorf("invalid session name: %w", err)
		}

		// Get current kitty state
		kittyState, err := k.GetState()
		if err != nil {
			return fmt.Errorf("get kitty state: %w", err)
		}

		// Derive session from current state using user_vars
		session := manager.DeriveSession(sessionName, kittyState)

		// Save session
		if err := st.SaveSession(session); err != nil {
			return fmt.Errorf("save session: %w", err)
		}

		// Close windows belonging to this session (check user_vars)
		for _, osWin := range kittyState {
			for _, tab := range osWin.Tabs {
				for _, win := range tab.Windows {
					if win.UserVars["kmux_session"] == sessionName {
						k.CloseWindow(win.ID)
					}
				}
			}
		}

		fmt.Printf("Detached from session: %s\n", sessionName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(detachCmd)
}
