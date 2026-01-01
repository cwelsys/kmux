package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/cwel/kmux/internal/store"
	"github.com/cwel/kmux/internal/zmx"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List sessions",
	Long:    "List saved sessions and active zmx sessions.",
	RunE: func(cmd *cobra.Command, args []string) error {
		st := store.DefaultStore()
		z := zmx.NewClient()

		// Get saved sessions
		saved, err := st.ListSessions()
		if err != nil {
			return fmt.Errorf("list saved sessions: %w", err)
		}

		// Get running zmx sessions
		running, err := z.List()
		if err != nil {
			return fmt.Errorf("list zmx sessions: %w", err)
		}

		// Build set of running zmx session prefixes (session names)
		runningSet := make(map[string]bool)
		for _, r := range running {
			// Extract session name from "sessionname.tab.window"
			parts := splitSessionName(r)
			if len(parts) > 0 {
				runningSet[parts[0]] = true
			}
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SESSION\tSTATUS\tPANES")

		for _, name := range saved {
			status := "saved"
			if runningSet[name] {
				status = "running"
			}

			sess, err := st.LoadSession(name)
			panes := 0
			if err == nil {
				for _, tab := range sess.Tabs {
					panes += len(tab.Windows)
				}
			}

			fmt.Fprintf(w, "%s\t%s\t%d\n", name, status, panes)
		}

		w.Flush()
		return nil
	},
}

func splitSessionName(zmxName string) []string {
	// Split "myproject.0.0" into ["myproject", "0", "0"]
	var parts []string
	var current string
	for _, c := range zmxName {
		if c == '.' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func init() {
	rootCmd.AddCommand(lsCmd)
}
