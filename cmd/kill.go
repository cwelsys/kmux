package cmd

import (
	"fmt"
	"strings"

	"github.com/cwel/kmux/internal/store"
	"github.com/cwel/kmux/internal/zmx"
	"github.com/spf13/cobra"
)

var killCmd = &cobra.Command{
	Use:   "kill <name>",
	Short: "Kill a session",
	Long:  "Kill all zmx sessions and delete saved state for a session.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Validate session name early
		if err := store.ValidateSessionName(name); err != nil {
			return err
		}

		st := store.DefaultStore()
		z := zmx.NewClient()

		// Load session to get zmx session names
		session, err := st.LoadSession(name)
		if err == nil && len(session.ZmxSessions) > 0 {
			// Kill each zmx session
			for _, zmxName := range session.ZmxSessions {
				if err := z.Kill(zmxName); err != nil {
					fmt.Printf("warning: failed to kill zmx session %s: %v\n", zmxName, err)
				}
			}
		}

		// Also kill any running zmx sessions matching the pattern
		running, err := z.List()
		if err != nil {
			fmt.Printf("warning: failed to list running zmx sessions: %v\n", err)
		}
		prefix := name + "."
		for _, r := range running {
			if strings.HasPrefix(r, prefix) {
				if err := z.Kill(r); err != nil {
					fmt.Printf("warning: failed to kill zmx session %s: %v\n", r, err)
				}
			}
		}

		// Delete saved session
		if err := st.DeleteSession(name); err != nil {
			return fmt.Errorf("delete session: %w", err)
		}

		fmt.Printf("Killed session: %s\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(killCmd)
}
