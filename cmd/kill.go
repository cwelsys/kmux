package cmd

import (
	"fmt"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/client"
	"github.com/cwel/kmux/internal/store"
	"github.com/spf13/cobra"
)

var killAll bool

var killCmd = &cobra.Command{
	Use:     "kill <name>... | --all",
	Aliases: []string{"k", "rm"},
	Short:   "Kill sessions",
	Long:  "Terminate zmx sessions and delete saved state. Use --all or * to kill all sessions including restore points.",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(config.SocketPath())

		if err := c.EnsureRunning(); err != nil {
			return fmt.Errorf("daemon: %w", err)
		}

		var names []string

		// Handle --all or * argument
		if killAll || (len(args) == 1 && args[0] == "*") {
			sessions, err := c.Sessions(true) // include restore points
			if err != nil {
				return fmt.Errorf("list sessions: %w", err)
			}
			for _, s := range sessions {
				names = append(names, s.Name)
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
			if err := c.Kill(name); err != nil {
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

func init() {
	killCmd.Flags().BoolVarP(&killAll, "all", "a", false, "Kill all sessions including restore points")
	rootCmd.AddCommand(killCmd)
}
