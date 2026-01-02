package cmd

import (
	"fmt"
	"os"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/client"
	"github.com/cwel/kmux/internal/store"
	"github.com/spf13/cobra"
)

var detachCmd = &cobra.Command{
	Use:   "detach",
	Short: "Detach from current session",
	Long:  "Save current session state and close session windows.",
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := os.Getenv("KMUX_SESSION")
		if sessionName == "" {
			return fmt.Errorf("not in a kmux session (KMUX_SESSION not set)")
		}

		if err := store.ValidateSessionName(sessionName); err != nil {
			return fmt.Errorf("invalid session name: %w", err)
		}

		c := client.New(config.SocketPath())

		if err := c.EnsureRunning(); err != nil {
			return fmt.Errorf("daemon: %w", err)
		}

		if err := c.Detach(sessionName); err != nil {
			return err
		}

		fmt.Printf("Detached from session: %s\n", sessionName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(detachCmd)
}
