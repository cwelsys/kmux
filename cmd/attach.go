package cmd

import (
	"fmt"
	"os"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/client"
	"github.com/cwel/kmux/internal/store"
	"github.com/spf13/cobra"
)

var (
	attachLayout string
	attachCWD    string
)

var attachCmd = &cobra.Command{
	Use:     "attach <name>",
	Aliases: []string{"a"},
	Short:   "Attach to a session",
	Long:  "Attach to an existing session or create a new one.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if err := store.ValidateSessionName(name); err != nil {
			return err
		}

		c := client.New(config.SocketPath())

		if err := c.EnsureRunning(); err != nil {
			return fmt.Errorf("daemon: %w", err)
		}

		cwd := attachCWD
		if cwd == "" {
			cwd, _ = os.Getwd()
		}

		if err := c.Attach(name, cwd, attachLayout); err != nil {
			return err
		}

		fmt.Printf("Attached to session: %s\n", name)
		return nil
	},
}

func init() {
	attachCmd.Flags().StringVarP(&attachLayout, "layout", "l", "", "create session from layout template")
	attachCmd.Flags().StringVarP(&attachCWD, "cwd", "C", "", "working directory for panes")
	rootCmd.AddCommand(attachCmd)
}
