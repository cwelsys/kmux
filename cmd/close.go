package cmd

import (
	"fmt"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/client"
	"github.com/spf13/cobra"
)

var closeCmd = &cobra.Command{
	Use:   "close",
	Short: "Close the current window",
	Long: `Close the currently focused kitty window.

If the window is a kmux session window, kills the zmx session first.
Works for both kmux and non-kmux windows - the daemon determines which.

Designed to be mapped in kitty.conf:

  map ctrl+space>x launch --type=background kmux close`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(config.SocketPath())

		if err := c.EnsureRunning(); err != nil {
			return fmt.Errorf("daemon: %w", err)
		}

		return c.CloseFocused()
	},
}

var closeTabCmd = &cobra.Command{
	Use:   "close-tab",
	Short: "Close the current tab",
	Long: `Close the currently focused kitty tab.

Kills all zmx sessions in the tab if it contains kmux windows.
Works for both kmux and non-kmux tabs - the daemon determines which.

Designed to be mapped in kitty.conf:

  map cmd+w launch --type=background kmux close-tab`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(config.SocketPath())

		if err := c.EnsureRunning(); err != nil {
			return fmt.Errorf("daemon: %w", err)
		}

		return c.CloseTab()
	},
}

func init() {
	rootCmd.AddCommand(closeCmd)
	rootCmd.AddCommand(closeTabCmd)
}
