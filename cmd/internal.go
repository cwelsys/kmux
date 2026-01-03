package cmd

import (
	"strconv"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/client"
	"github.com/spf13/cobra"
)

var internalCmd = &cobra.Command{
	Use:    "internal",
	Short:  "Internal commands (not for direct use)",
	Hidden: true,
}

var notifyCloseCmd = &cobra.Command{
	Use:   "notify-close <window_id> <zmx_name> <session>",
	Short: "Notify daemon of window close",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		windowID, err := strconv.Atoi(args[0])
		if err != nil {
			return err
		}
		zmxName := args[1]
		session := args[2]

		c := client.New(config.SocketPath())
		return c.NotifyWindowClosed(windowID, zmxName, session)
	},
}

func init() {
	internalCmd.AddCommand(notifyCloseCmd)
	rootCmd.AddCommand(internalCmd)
}
