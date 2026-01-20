package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/cwel/kmux/internal/kitty"
	"github.com/cwel/kmux/internal/state"
	"github.com/cwel/kmux/internal/zmx"
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
		sessionName := splitSession

		s := state.New()
		k := s.KittyClient()

		// Try to resolve session from current window if not specified
		if sessionName == "" {
			windowIDStr := os.Getenv("KITTY_WINDOW_ID")
			if windowIDStr != "" {
				windowID, err := strconv.Atoi(windowIDStr)
				if err == nil {
					sessInfo, _, err := s.FindWindowSession(windowID)
					if err == nil && sessInfo != nil {
						sessionName = sessInfo.Name
					}
				}
			}
		}

		// If no session, create a native kitty split (no zmx)
		if sessionName == "" {
			opts := kitty.LaunchOpts{
				Type:     "window",
				Location: location,
				CWD:      cwd,
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
		zmxCmd := zmx.AttachCmd(zmxName)

		// Launch the split window with zmx and user_vars
		opts := kitty.LaunchOpts{
			Type:     "window",
			Location: location,
			CWD:      cwd,
			Cmd:      zmxCmd,
			Vars: map[string]string{
				"kmux_zmx":     zmxName,
				"kmux_session": sessionName,
			},
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
	rootCmd.AddCommand(splitCmd)
}
