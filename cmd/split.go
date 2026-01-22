package cmd

import (
	"fmt"

	"github.com/cwel/kmux/internal/kitty"
	"github.com/cwel/kmux/internal/state"
	"github.com/spf13/cobra"
)

var (
	splitSession string
	splitCwd     string
)

var splitCmd = &cobra.Command{
	Use:   "split <direction>",
	Short: "Create a split window",
	Long: `Create a new split window.

Direction must be 'vertical' (or 'v') for side-by-side, or 'horizontal' (or 'h') for stacked.

If run from within a kmux session, creates a zmx-backed persistent split.
If run outside a kmux session, creates a native kitty split.

Use --session to specify which session to split (for scripting outside sessions).

The --cwd flag controls the working directory. Special values:
  current        Use cwd of the current window (default, preserves SSH context)
  last_reported  Use the last cwd reported by shell integration
  oldest         Use cwd of the oldest foreground process
  root           Use cwd of the original process
  <path>         Use an explicit directory path`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		direction := args[0]
		// Validate direction
		var location string
		switch direction {
		case "vertical", "v":
			location = "vsplit"
		case "horizontal", "h":
			location = "hsplit"
		default:
			return fmt.Errorf("invalid direction: %s (use 'vertical', 'v', 'horizontal', or 'h')", direction)
		}

		sessionName := splitSession

		s := state.New()
		k := s.KittyClient()

		// Find session/host from focused window's user_vars
		// Note: We query kitty state directly instead of using KITTY_WINDOW_ID env
		// because --copy-env doesn't work on macOS (KERN_PROCARGS2 is empty for shells)
		var host string
		if sessionName == "" {
			kittyState, err := k.GetState()
			if err == nil {
				// Find the active window and read its user_vars
				for _, osWin := range kittyState {
					if !osWin.IsActive {
						continue
					}
					for _, tab := range osWin.Tabs {
						if !tab.IsActive {
							continue
						}
						for _, win := range tab.Windows {
							if !win.IsActive {
								continue
							}
							sessionName = win.UserVars["kmux_session"]
							host = win.UserVars["kmux_host"]
							break
						}
						break
					}
					break
				}
			}
		}
		if host == "" {
			host = "local"
		}

		// If no session, create a native kitty split (no zmx)
		if sessionName == "" {
			opts := kitty.LaunchOpts{
				Type:     "window",
				Location: location,
				CWD:      splitCwd,
			}

			windowID, err := k.Launch(opts)
			if err != nil {
				return fmt.Errorf("launch split: %w", err)
			}

			fmt.Printf("Created native %s split (window %d)\n", direction, windowID)
			return nil
		}

		// In a kmux session - create zmx-backed split
		kittyState, err := k.GetState()
		if err != nil {
			return fmt.Errorf("get kitty state: %w", err)
		}

		if len(kittyState) == 0 {
			return fmt.Errorf("no kitty windows found")
		}

		// Find windows for this session by reading user_vars (source of truth)
		var windowCount int
		for _, osWin := range kittyState {
			for _, tab := range osWin.Tabs {
				for _, win := range tab.Windows {
					if win.UserVars["kmux_session"] == sessionName {
						windowCount++
					}
				}
			}
		}

		if windowCount == 0 {
			return fmt.Errorf("no windows found for session: %s", sessionName)
		}

		// Build zmx session name: {session}.0.{window_idx}
		// For now, assume single-tab sessions (tab index = 0)
		zmxName := fmt.Sprintf("%s.0.%d", sessionName, windowCount)

		// Get the zmx client for this host and build attach command
		zmxClient := s.ZmxClientForHost(host)
		zmxCmd := zmxClient.AttachCmd(zmxName)

		// Launch the split window with zmx and user_vars
		vars := map[string]string{
			"kmux_zmx":     zmxName,
			"kmux_session": sessionName,
		}
		if host != "local" {
			vars["kmux_host"] = host
		}

		opts := kitty.LaunchOpts{
			Type:     "window",
			Location: location,
			CWD:      splitCwd,
			Cmd:      zmxCmd,
			Vars:     vars,
		}

		windowID, err := k.Launch(opts)
		if err != nil {
			return fmt.Errorf("launch split: %w", err)
		}

		fmt.Printf("Created %s split (window %d)\n", direction, windowID)
		return nil
	},
}

func init() {
	splitCmd.Flags().StringVarP(&splitSession, "session", "s", "", "Session to create split in (default: $KMUX_SESSION)")
	splitCmd.Flags().StringVar(&splitCwd, "cwd", "current", "Working directory (current, last_reported, oldest, root, or path)")
	rootCmd.AddCommand(splitCmd)
}
