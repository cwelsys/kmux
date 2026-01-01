package cmd

import (
	"fmt"
	"os"

	"github.com/cwel/kmux/internal/kitty"
	"github.com/cwel/kmux/internal/manager"
	"github.com/cwel/kmux/internal/store"
	"github.com/spf13/cobra"
)

var detachCmd = &cobra.Command{
	Use:   "detach",
	Short: "Detach from current session",
	Long:  "Save current session state and close session windows.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get session name from env
		sessionName := os.Getenv("KMUX_SESSION")
		if sessionName == "" {
			return fmt.Errorf("not in a kmux session (KMUX_SESSION not set)")
		}

		// Validate session name
		if err := store.ValidateSessionName(sessionName); err != nil {
			return fmt.Errorf("invalid session name: %w", err)
		}

		st := store.DefaultStore()
		k := kitty.NewClient()

		// Get current kitty state
		state, err := k.GetState()
		if err != nil {
			return fmt.Errorf("get kitty state: %w", err)
		}

		// Derive session from state (only windows belonging to this session)
		session := manager.DeriveSession(sessionName, state)

		// Save session
		if err := st.SaveSession(session); err != nil {
			return fmt.Errorf("save session: %w", err)
		}

		// Close only windows belonging to this session (identified by KMUX_SESSION env)
		if len(state) > 0 {
			for _, tab := range state[0].Tabs {
				for _, win := range tab.Windows {
					if win.Env["KMUX_SESSION"] == sessionName {
						// Close this window (zmx session stays alive)
						_ = k.CloseWindow(win.ID)
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
