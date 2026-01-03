package cmd

import (
	"fmt"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/client"
	"github.com/cwel/kmux/internal/store"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename <old> <new>",
	Short: "Rename a session",
	Long:  `Rename a session. Works whether the session is attached or detached.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldName := args[0]
		newName := args[1]

		if err := store.ValidateSessionName(oldName); err != nil {
			return fmt.Errorf("invalid old name: %w", err)
		}
		if err := store.ValidateSessionName(newName); err != nil {
			return fmt.Errorf("invalid new name: %w", err)
		}

		c := client.New(config.SocketPath())

		if err := c.EnsureRunning(); err != nil {
			return fmt.Errorf("daemon: %w", err)
		}

		if err := c.Rename(oldName, newName); err != nil {
			return err
		}

		fmt.Printf("Renamed session: %s -> %s\n", oldName, newName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(renameCmd)
}
