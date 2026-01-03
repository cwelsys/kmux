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
	Use:     "detach [session]",
	Aliases: []string{"d"},
	Short:   "Detach from a session",
	Long: `Save session state and close session windows.

If session name is provided, detaches that session.
Otherwise uses $KMUX_SESSION from the environment.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var sessionName string
		if len(args) > 0 {
			sessionName = args[0]
		} else {
			// Try to resolve session from current window
			windowIDStr := os.Getenv("KITTY_WINDOW_ID")
			if windowIDStr != "" {
				var windowID int
				if _, err := fmt.Sscanf(windowIDStr, "%d", &windowID); err == nil {
					c := client.New(config.SocketPath())
					if err := c.EnsureRunning(); err == nil {
						sessionName, _ = c.Resolve(windowID)
					}
				}
			}
		}

		if sessionName == "" {
			return fmt.Errorf("session name required (provide as argument or run from within a session)")
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
