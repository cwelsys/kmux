package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/client"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List sessions",
	Long:    "List saved sessions and their status.",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(config.SocketPath())

		if err := c.EnsureRunning(); err != nil {
			return fmt.Errorf("daemon: %w", err)
		}

		sessions, err := c.Sessions()
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SESSION\tSTATUS\tPANES")

		for _, s := range sessions {
			fmt.Fprintf(w, "%s\t%s\t%d\n", s.Name, s.Status, s.Panes)
		}

		w.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(lsCmd)
}
