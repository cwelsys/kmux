package cmd

import (
	"context"
	"encoding/json"
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
	lsJSON  bool
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
			sessions, err = s.Sessions(lsAll)
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			sessions, err = s.AllSessions(ctx, lsAll)
		}

		if err != nil {
			return err
		}

		if lsJSON {
			return printSessionsJSON(sessions)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SESSION\tHOST\tSTATUS\tPANES")
		for _, sess := range sessions {
			host := sess.Host
			if host == "" {
				host = "local"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", sess.Name, host, sess.Status, sess.Panes)
		}
		w.Flush()
		return nil
	},
}

type sessionJSON struct {
	Name   string `json:"name"`
	Host   string `json:"host"`
	Status string `json:"status"`
	Panes  int    `json:"panes"`
}

func printSessionsJSON(sessions []state.SessionInfo) error {
	out := make([]sessionJSON, len(sessions))
	for i, s := range sessions {
		host := s.Host
		if host == "" {
			host = "local"
		}
		out[i] = sessionJSON{
			Name:   s.Name,
			Host:   host,
			Status: s.Status,
			Panes:  s.Panes,
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func init() {
	lsCmd.Flags().BoolVarP(&lsAll, "all", "a", false, "Include restore points (saved sessions without running zmx)")
	lsCmd.Flags().BoolVarP(&lsLocal, "local", "L", false, "Only show local sessions (skip remote hosts)")
	lsCmd.Flags().BoolVar(&lsJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(lsCmd)
}
