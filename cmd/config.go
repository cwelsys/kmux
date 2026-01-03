package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cwel/kmux/internal/config"
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
		defaultConfig := `[daemon]
watch_interval = 5
auto_save_interval = 900

[kitty]
# socket = "/tmp/mykitty"
`
		if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
			return fmt.Errorf("write config: %w", err)
		}

		fmt.Printf("Created config at %s\n", configPath)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configInitCmd)
	rootCmd.AddCommand(configCmd)
}
