package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/client"
	"github.com/spf13/cobra"
)

var closeCmd = &cobra.Command{
	Use:   "close",
	Short: "Close the current kmux window",
	Long: `Close the current kmux window, properly cleaning up the zmx session.

Use this instead of kitty's close_window to ensure zmx sessions are killed
and the daemon is notified. Designed to be mapped in kitty.conf:

  map ctrl+w launch --type=background --copy-env kmux close`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get window ID from environment
		windowIDStr := os.Getenv("KITTY_WINDOW_ID")
		if windowIDStr == "" {
			return fmt.Errorf("KITTY_WINDOW_ID not set - must run from within kitty")
		}
		windowID, err := strconv.Atoi(windowIDStr)
		if err != nil {
			return fmt.Errorf("invalid KITTY_WINDOW_ID: %v", err)
		}

		// Get kitty socket
		kittySocket := os.Getenv("KITTY_LISTEN_ON")
		if kittySocket == "" {
			return fmt.Errorf("KITTY_LISTEN_ON not set")
		}

		// Connect to daemon and resolve the window
		c := client.New(config.SocketPath())
		if err := c.EnsureRunning(); err != nil {
			// Daemon not running - just close the window
			return closeKittyWindow(kittySocket, windowID)
		}

		// Get session info for this window
		session, zmxName, err := c.ResolveWindow(windowID)
		if err != nil || session == "" {
			// Not a kmux window - just close it
			return closeKittyWindow(kittySocket, windowID)
		}

		// Kill the zmx session
		if zmxName != "" {
			exec.Command("zmx", "kill", zmxName).Run()
		}

		// Notify daemon
		c.NotifyWindowClosed(windowID, zmxName, session)

		// Close the kitty window
		return closeKittyWindow(kittySocket, windowID)
	},
}

func closeKittyWindow(socket string, windowID int) error {
	// Extract socket path from "unix:/path" format
	socketPath := socket
	if len(socket) > 5 && socket[:5] == "unix:" {
		socketPath = socket[5:]
	}

	cmd := exec.Command("kitty", "@", "--to", "unix:"+socketPath,
		"close-window", "--match", fmt.Sprintf("id:%d", windowID))
	// Ignore errors - window may already be closed after zmx kill
	cmd.Run()
	return nil
}

func init() {
	rootCmd.AddCommand(closeCmd)
}
