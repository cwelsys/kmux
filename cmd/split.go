package cmd

import (
	"fmt"
	"os"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/client"
	"github.com/cwel/kmux/internal/kitty"
	"github.com/spf13/cobra"
)

var splitCmd = &cobra.Command{
	Use:   "split <direction>",
	Short: "Create a split window",
	Long: `Create a new split window.

Direction must be 'vertical' (or 'v') for side-by-side, or 'horizontal' (or 'h') for stacked.

If run from within a kmux session, creates a zmx-backed persistent split.
If run outside a kmux session, creates a native kitty split.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		direction := args[0]
		// Validate and normalize direction
		var location string
		switch direction {
		case "vertical", "v":
			location = "vsplit"
		case "horizontal", "h":
			location = "hsplit"
		default:
			return fmt.Errorf("invalid direction: %s (use 'vertical', 'v', 'horizontal', or 'h')", direction)
		}

		cwd, _ := os.Getwd()
		sessionName := os.Getenv("KMUX_SESSION")

		// If in a kmux session, create zmx-backed split via daemon
		if sessionName != "" {
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
		}

		// Not in a kmux session - create native kitty split
		k := kitty.NewClient()
		opts := kitty.LaunchOpts{
			Type:     "window",
			Location: location,
			CWD:      cwd,
		}

		windowID, err := k.Launch(opts)
		if err != nil {
			return fmt.Errorf("kitty split: %w", err)
		}

		fmt.Printf("Created %s split (window %d)\n", direction, windowID)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(splitCmd)
}
