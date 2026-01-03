package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// DaemonConfig holds daemon-specific settings.
type DaemonConfig struct {
	WatchInterval    int `toml:"watch_interval"`
	AutoSaveInterval int `toml:"auto_save_interval"`
}

// KittyConfig holds kitty-specific settings.
type KittyConfig struct {
	Socket string `toml:"socket"`
}

// Config holds all kmux configuration.
type Config struct {
	Daemon DaemonConfig `toml:"daemon"`
	Kitty  KittyConfig  `toml:"kitty"`
}

// DefaultConfig returns configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Daemon: DaemonConfig{
			WatchInterval:    5,
			AutoSaveInterval: 900,
		},
	}
}

// LoadConfig loads configuration from the config file, using defaults for missing values.
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	configPath := filepath.Join(ConfigDir(), "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // No config file, use defaults
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

// SocketPath returns the daemon socket path.
func SocketPath() string {
	if path := os.Getenv("KMUX_SOCKET"); path != "" {
		return path
	}

	tmpDir := os.Getenv("KMUX_TMPDIR")
	if tmpDir == "" {
		tmpDir = "/tmp"
	}

	uid := os.Getuid()
	return filepath.Join(tmpDir, fmt.Sprintf("kmux-%d", uid), "default")
}

// DataDir returns the data directory for session storage.
func DataDir() string {
	if dir := os.Getenv("KMUX_DATA_DIR"); dir != "" {
		return dir
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, _ := os.UserHomeDir()
		dataHome = filepath.Join(home, ".local", "share")
	}

	return filepath.Join(dataHome, "kmux")
}

// ConfigDir returns the config directory for user settings and layouts.
func ConfigDir() string {
	if dir := os.Getenv("KMUX_CONFIG_DIR"); dir != "" {
		return dir
	}

	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, _ := os.UserHomeDir()
		configHome = filepath.Join(home, ".config")
	}

	return filepath.Join(configHome, "kmux")
}
