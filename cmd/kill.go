package cmd

import (
	"fmt"

	"github.com/cwel/kmux/internal/state"
	"github.com/cwel/kmux/internal/store"
	"github.com/spf13/cobra"
)

var (
	killAll  bool
	killHost string
)

var killCmd = &cobra.Command{
	Use:               "kill <name>... | --all",
	Aliases:           []string{"k", "rm"},
	Short:             "Kill sessions",
	Long:              "Terminate zmx sessions and delete saved state. Use --all or * to kill all sessions including restore points.\n\nUse --host to specify which host's session to kill (default: local).",
	Args:              cobra.ArbitraryArgs,
	ValidArgsFunction: completeSessionNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := state.New()

		// Handle --all or * argument
		if killAll || (len(args) == 1 && args[0] == "*") {
			host := killHost
			if host == "" {
				host = "local"
			}
			sessions, err := s.Sessions(true) // include restore points
			if err != nil {
				return fmt.Errorf("list sessions: %w", err)
			}
			var names []string
			for _, sess := range sessions {
				// Only include sessions for the specified host
				if sess.Host == host {
					names = append(names, sess.Name)
				}
			}
			if len(names) == 0 {
				fmt.Println("No sessions to kill")
				return nil
			}

			var killed int
			for _, name := range names {
				if err := killSessionWithHost(s, name, host); err != nil {
					fmt.Printf("Failed to kill %s: %v\n", name, err)
					continue
				}
				killed++
			}
			if len(names) > 1 {
				fmt.Printf("Killed %d sessions\n", killed)
			}
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("session name required (or use --all)")
		}

		// Validate all names first
		for _, name := range args {
			if err := store.ValidateSessionName(name); err != nil {
				return err
			}
		}

		// Kill each session, auto-detecting host if not specified
		var killed int
		for _, name := range args {
			host := killHost
			if host == "" {
				// Auto-detect which host has this session
				host = autoDetectSessionHost(s, name)
			}

			if err := killSessionWithHost(s, name, host); err != nil {
				fmt.Printf("Failed to kill %s: %v\n", name, err)
				continue
			}
			killed++
		}

		if len(args) > 1 {
			fmt.Printf("Killed %d sessions\n", killed)
		}
		return nil
	},
}

func init() {
	killCmd.Flags().BoolVarP(&killAll, "all", "a", false, "Kill all sessions including restore points")
	killCmd.Flags().StringVarP(&killHost, "host", "H", "", "remote host (SSH alias, default: local)")
	rootCmd.AddCommand(killCmd)
}
