package cmd

import (
	"fmt"
	"os"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/client"
	"github.com/spf13/cobra"
)

var splitSession string

var splitCmd = &cobra.Command{
	Use:   "split <direction>",
	Short: "Create a split window",
	Long: `Create a new split window.

Direction must be 'vertical' (or 'v') for side-by-side, or 'horizontal' (or 'h') for stacked.

If run from within a kmux session, creates a zmx-backed persistent split.
If run outside a kmux session, creates a native kitty split.

Use --session to specify which session to split (for scripting outside sessions).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		direction := args[0]
		// Validate direction
		switch direction {
		case "vertical", "v", "horizontal", "h":
			// valid
		default:
			return fmt.Errorf("invalid direction: %s (use 'vertical', 'v', 'horizontal', or 'h')", direction)
		}

		cwd, _ := os.Getwd()
		sessionName := splitSession
		if sessionName == "" {
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

		// All splits go through daemon - it discovers the kitty socket
		c := client.New(config.SocketPath())

		if err := c.EnsureRunning(); err != nil {
			return fmt.Errorf("daemon: %w", err)
		}

		windowID, err := c.Split(sessionName, direction, cwd)
		if err != nil {
			return err
		}

		fmt.Printf("Created %s split (window %d)\n", direction, windowID)
		return nil
	},
}

func init() {
	splitCmd.Flags().StringVarP(&splitSession, "session", "s", "", "Session to create split in (default: $KMUX_SESSION)")
	rootCmd.AddCommand(splitCmd)
}
