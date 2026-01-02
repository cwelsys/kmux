package cmd

import (
	"fmt"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/client"
	"github.com/cwel/kmux/internal/store"
	"github.com/spf13/cobra"
)

var killCmd = &cobra.Command{
	Use:   "kill <name>",
	Short: "Kill a session",
	Long:  "Terminate zmx sessions and delete saved state.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if err := store.ValidateSessionName(name); err != nil {
			return err
		}

		c := client.New(config.SocketPath())

		if err := c.EnsureRunning(); err != nil {
			return fmt.Errorf("daemon: %w", err)
		}

		if err := c.Kill(name); err != nil {
			return err
		}

		fmt.Printf("Killed session: %s\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(killCmd)
}
