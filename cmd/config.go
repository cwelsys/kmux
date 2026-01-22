package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/store"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage kmux configuration",
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print config file location",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(filepath.Join(config.ConfigDir(), "config.toml"))
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create default config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := config.ConfigDir()
		configPath := filepath.Join(configDir, "config.toml")

		// Create directory if needed
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("create config dir: %w", err)
		}

		// Back up existing config
		if _, err := os.Stat(configPath); err == nil {
			backupPath := configPath + ".bak"
			if err := os.Rename(configPath, backupPath); err != nil {
				return fmt.Errorf("backup config: %w", err)
			}
			fmt.Printf("Backed up existing config to %s\n", backupPath)
		}

		// Write default config
		defaultConfig := `[kitty]
# Socket path for kitty remote control (required if running kmux outside kitty)
# socket = "/tmp/mykitty"

[projects]
# Directories to scan for projects (shown in TUI)
# directories = ["~/src", "~/projects"]
# max_depth = 2
# git_only = true  # only show git repos (set false to show all directories)
# ignore = ["node_modules", "vendor", "~/src/old-stuff"]
`
		if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
			return fmt.Errorf("write config: %w", err)
		}

		fmt.Printf("Created config at %s\n", configPath)

		// Install bundled layouts
		if err := store.InstallBundledLayouts(); err != nil {
			return fmt.Errorf("install bundled layouts: %w", err)
		}
		fmt.Printf("Installed bundled layouts to %s\n", filepath.Join(config.DataDir(), "layouts"))

		return nil
	},
}

func init() {
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configInitCmd)
	rootCmd.AddCommand(configCmd)
}
