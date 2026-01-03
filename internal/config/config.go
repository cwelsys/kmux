package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds daemon configuration.
type Config struct {
	WatchInterval    int `json:"watch_interval"`     // seconds
	AutoSaveInterval int `json:"auto_save_interval"` // seconds
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		WatchInterval:    5,
		AutoSaveInterval: 900, // 15 minutes
	}
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
