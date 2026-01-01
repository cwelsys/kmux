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
	Long:  "Save current session state and close all windows.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get session name from env
		sessionName := os.Getenv("KMUX_SESSION")
		if sessionName == "" {
			return fmt.Errorf("not in a kmux session (KMUX_SESSION not set)")
		}

		st := store.DefaultStore()
		k := kitty.NewClient()

		// Get current kitty state
		state, err := k.GetState()
		if err != nil {
			return fmt.Errorf("get kitty state: %w", err)
		}

		// Derive session from state
		session := manager.DeriveSession(sessionName, state)

		// Save session
		if err := st.SaveSession(session); err != nil {
			return fmt.Errorf("save session: %w", err)
		}

		// Close all tabs in current OS window (zmx sessions stay alive)
		if len(state) > 0 {
			for _, tab := range state[0].Tabs {
				if err := k.CloseTab(tab.ID); err != nil {
					continue // Ignore errors (might be closing self)
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
