package cmd

import (
	"fmt"

	"github.com/cwel/kmux/internal/state"
	"github.com/cwel/kmux/internal/store"
	"github.com/spf13/cobra"
)

var renameHost string

var renameCmd = &cobra.Command{
	Use:   "rename <old> <new>",
	Short: "Rename a session",
	Long: `Rename a session. Updates save files, ownership tracking, and kitty tab titles.

By default, renames the session across all hosts. Use --host to only rename on a specific host.`,
	Args:  cobra.ExactArgs(2),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// Only complete the first arg (old name), not the second (new name)
		if len(args) == 0 {
			return completeSessionNames(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		oldName := args[0]
		newName := args[1]

		if err := store.ValidateSessionName(oldName); err != nil {
			return fmt.Errorf("invalid old name: %w", err)
		}
		if err := store.ValidateSessionName(newName); err != nil {
			return fmt.Errorf("invalid new name: %w", err)
		}

		s := state.New()
		st := s.Store()

		// 1. Rename the save file (non-fatal: save file might not exist)
		st.RenameSession(oldName, newName)

		// 2. Update ownership mappings (zmx name -> session name)
		if err := store.RenameSessionOwnership(oldName, newName); err != nil {
			return fmt.Errorf("update ownership: %w", err)
		}

		// 3. Update kitty tab titles for active windows
		kc := s.KittyClient()
		kittyState, _ := kc.GetState()
		renamedTabs := 0
		for _, osWin := range kittyState {
			for _, tab := range osWin.Tabs {
				// Check if any window in this tab belongs to the old session
				for _, win := range tab.Windows {
					if win.UserVars["kmux_session"] != oldName {
						continue
					}
					// Filter by host if specified
					if renameHost != "" {
						winHost := win.UserVars["kmux_host"]
						if winHost == "" {
							winHost = "local"
						}
						if winHost != renameHost {
							continue
						}
					}
					kc.SetTabTitle(win.ID, newName)
					renamedTabs++
					break // Only rename once per tab
				}
			}
		}
		if renamedTabs > 0 {
			if renameHost != "" {
				fmt.Printf("Renamed session: %s -> %s on %s (tab titles updated, user_vars unchanged until detach/reattach)\n", oldName, newName, renameHost)
			} else {
				fmt.Printf("Renamed session: %s -> %s (tab titles updated, user_vars unchanged until detach/reattach)\n", oldName, newName)
			}
		} else {
			fmt.Printf("Renamed session: %s -> %s\n", oldName, newName)
		}

		return nil
	},
}

func init() {
	renameCmd.Flags().StringVarP(&renameHost, "host", "H", "", "only rename on specific host (default: all hosts)")
	rootCmd.AddCommand(renameCmd)
}
