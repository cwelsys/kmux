package cmd

import (
	"fmt"
	"os"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/client"
	"github.com/spf13/cobra"
)

var splitCmd = &cobra.Command{
	Use:   "split <direction>",
	Short: "Create a split in the current session",
	Long: `Create a new split window in the current kmux session.

Direction must be 'vertical' (or 'v') for side-by-side, or 'horizontal' (or 'h') for stacked.

Must be run from within a kmux session (KMUX_SESSION must be set).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := os.Getenv("KMUX_SESSION")
		if sessionName == "" {
			return fmt.Errorf("not in a kmux session (KMUX_SESSION not set)")
		}

		direction := args[0]
		// Validate direction
		switch direction {
		case "vertical", "v", "horizontal", "h":
			// valid
		default:
			return fmt.Errorf("invalid direction: %s (use 'vertical', 'v', 'horizontal', or 'h')", direction)
		}

		c := client.New(config.SocketPath())

		if err := c.EnsureRunning(); err != nil {
			return fmt.Errorf("daemon: %w", err)
		}

		cwd, _ := os.Getwd()

		windowID, err := c.Split(sessionName, direction, cwd)
		if err != nil {
			return err
		}

		fmt.Printf("Created %s split (window %d)\n", direction, windowID)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(splitCmd)
}
