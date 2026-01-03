package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/client"
	"github.com/spf13/cobra"
)

var lsAll bool

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"l", "list"},
	Short:   "List sessions",
	Long:    "List running sessions. Use --all to include restore points.",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(config.SocketPath())

		if err := c.EnsureRunning(); err != nil {
			return fmt.Errorf("daemon: %w", err)
		}

		sessions, err := c.Sessions(lsAll)
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
	lsCmd.Flags().BoolVarP(&lsAll, "all", "a", false, "Include restore points (saved sessions without running zmx)")
	rootCmd.AddCommand(lsCmd)
}
