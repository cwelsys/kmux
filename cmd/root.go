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
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI()
	},
}

func init() {
	rootCmd.SetHelpFunc(styledHelp)
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "help",
		Short:  "Print this help message",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			styledHelp(rootCmd, nil)
			return nil
		},
	})
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
