package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/cwel/kmux/internal/kitty"
	"github.com/cwel/kmux/internal/model"
	"github.com/cwel/kmux/internal/store"
	"github.com/cwel/kmux/internal/zmx"
	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:   "attach <name>",
	Short: "Attach to a session",
	Long:  "Attach to an existing session or create a new one.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		st := store.DefaultStore()
		k := kitty.NewClient()

		// Try to load existing session
		session, err := st.LoadSession(name)
		if err != nil {
			// Create new session
			cwd, _ := os.Getwd()
			session = &model.Session{
				Name:    name,
				Host:    "local",
				SavedAt: time.Now(),
				Tabs: []model.Tab{
					{
						Title:  name,
						Layout: "splits",
						Windows: []model.Window{
							{CWD: cwd},
						},
					},
				},
			}
		}

		// Create windows in kitty
		for tabIdx, tab := range session.Tabs {
			for winIdx, win := range tab.Windows {
				zmxName := session.ZmxSessionName(tabIdx, winIdx)
				zmxCmd := zmx.AttachCmd(zmxName, win.Command)

				launchType := "tab"
				if winIdx > 0 {
					launchType = "window" // split within tab
				}

				opts := kitty.LaunchOpts{
					Type:  launchType,
					CWD:   win.CWD,
					Title: tab.Title,
					Cmd:   zmxCmd,
				}

				_, err := k.Launch(opts)
				if err != nil {
					return fmt.Errorf("launch window: %w", err)
				}

				// Track zmx session
				session.ZmxSessions = append(session.ZmxSessions, zmxName)
			}
		}

		// Save session state
		if err := st.SaveSession(session); err != nil {
			return fmt.Errorf("save session: %w", err)
		}

		fmt.Printf("Attached to session: %s\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(attachCmd)
}
