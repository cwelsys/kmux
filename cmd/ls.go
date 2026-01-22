package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/cwel/kmux/internal/state"
	"github.com/spf13/cobra"
)

var (
	lsAll   bool
	lsLocal bool
)

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"l", "list"},
	Short:   "List sessions",
	Long:    "List running sessions. Use --all to include restore points.",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := state.New()

		var sessions []state.SessionInfo
		var err error

		if lsLocal {
			// Local only mode
			sessions, err = s.Sessions(lsAll)
		} else {
			// Query all configured hosts
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			sessions, err = s.AllSessions(ctx, lsAll)
		}

		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

		// Check if we have any remote sessions
		hasRemote := false
		for _, sess := range sessions {
			if sess.Host != "local" {
				hasRemote = true
				break
			}
		}

		if hasRemote {
			fmt.Fprintln(w, "SESSION\tHOST\tSTATUS\tPANES")
			for _, sess := range sessions {
				host := sess.Host
				if host == "" {
					host = "local"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", sess.Name, host, sess.Status, sess.Panes)
			}
		} else {
			fmt.Fprintln(w, "SESSION\tSTATUS\tPANES")
			for _, sess := range sessions {
				fmt.Fprintf(w, "%s\t%s\t%d\n", sess.Name, sess.Status, sess.Panes)
			}
		}

		w.Flush()
		return nil
	},
}

func init() {
	lsCmd.Flags().BoolVarP(&lsAll, "all", "a", false, "Include restore points (saved sessions without running zmx)")
	lsCmd.Flags().BoolVarP(&lsLocal, "local", "L", false, "Only show local sessions (skip remote hosts)")
	rootCmd.AddCommand(lsCmd)
}
