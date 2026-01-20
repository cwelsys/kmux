package cmd

import (
	"fmt"

	"github.com/cwel/kmux/internal/model"
	"github.com/cwel/kmux/internal/state"
	"github.com/cwel/kmux/internal/store"
	"github.com/spf13/cobra"
)

var killAll bool

var killCmd = &cobra.Command{
	Use:               "kill <name>... | --all",
	Aliases:           []string{"k", "rm"},
	Short:             "Kill sessions",
	Long:              "Terminate zmx sessions and delete saved state. Use --all or * to kill all sessions including restore points.",
	Args:              cobra.ArbitraryArgs,
	ValidArgsFunction: completeSessionNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := state.New()

		var names []string

		// Handle --all or * argument
		if killAll || (len(args) == 1 && args[0] == "*") {
			sessions, err := s.Sessions(true) // include restore points
			if err != nil {
				return fmt.Errorf("list sessions: %w", err)
			}
			for _, sess := range sessions {
				names = append(names, sess.Name)
			}
			if len(names) == 0 {
				fmt.Println("No sessions to kill")
				return nil
			}
		} else if len(args) == 0 {
			return fmt.Errorf("session name required (or use --all)")
		} else {
			// Validate all names first
			for _, name := range args {
				if err := store.ValidateSessionName(name); err != nil {
					return err
				}
			}
			names = args
		}

		// Kill each session
		var killed int
		for _, name := range names {
			if err := killSession(s, name); err != nil {
				fmt.Printf("Failed to kill %s: %v\n", name, err)
				continue
			}
			fmt.Printf("Killed: %s\n", name)
			killed++
		}

		if len(names) > 1 {
			fmt.Printf("Killed %d sessions\n", killed)
		}
		return nil
	},
}

func killSession(s *state.State, name string) error {
	k := s.KittyClient()
	z := s.ZmxClient()
	st := s.Store()

	// Collect zmx sessions to kill from save file and naming convention
	zmxToKill := make(map[string]bool)

	// Check save file first
	if sess, err := st.LoadSession(name); err == nil {
		for _, zmxName := range sess.ZmxSessions {
			zmxToKill[zmxName] = true
		}
		for _, tab := range sess.Tabs {
			for _, win := range tab.Windows {
				if win.ZmxName != "" {
					zmxToKill[win.ZmxName] = true
				}
			}
		}
	}

	// Query zmx and find sessions matching naming convention
	zmxSessions, _ := z.List()
	for _, zmxName := range zmxSessions {
		if model.ParseZmxSessionName(zmxName) == name {
			zmxToKill[zmxName] = true
		}
	}

	// Get kitty state to find windows for this session
	kittyState, _ := k.GetState()

	// Close windows and collect any zmx names from user_vars
	for _, osWin := range kittyState {
		for _, tab := range osWin.Tabs {
			for _, win := range tab.Windows {
				if win.UserVars["kmux_session"] == name {
					// Add zmx name if present
					if zmxName := win.UserVars["kmux_zmx"]; zmxName != "" {
						zmxToKill[zmxName] = true
					}
					// Close the kitty window
					k.CloseWindow(win.ID)
				}
			}
		}
	}

	// Kill all zmx sessions for this session
	for zmxName := range zmxToKill {
		z.Kill(zmxName)
	}

	// Delete saved session
	st.DeleteSession(name)

	return nil
}

func init() {
	killCmd.Flags().BoolVarP(&killAll, "all", "a", false, "Kill all sessions including restore points")
	rootCmd.AddCommand(killCmd)
}
