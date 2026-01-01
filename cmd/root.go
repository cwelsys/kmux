package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "kmux",
	Short: "Session management for kitty + zmx",
	Long:  "kmux provides tmux-like session persistence using kitty for window management and zmx for process persistence.",
	Run: func(cmd *cobra.Command, args []string) {
		// Default action: show help (TUI comes later)
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
