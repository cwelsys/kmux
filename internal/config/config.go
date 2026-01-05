package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// DaemonConfig holds daemon-specific settings.
type DaemonConfig struct {
	AutoSaveInterval int `toml:"auto_save_interval"`
}

// KittyConfig holds kitty-specific settings.
type KittyConfig struct {
	Socket string `toml:"socket"`
}

// ProjectsConfig holds project discovery settings.
type ProjectsConfig struct {
	Directories []string `toml:"directories"`
	MaxDepth    int      `toml:"max_depth"`
	Ignore      []string `toml:"ignore"`   // patterns to ignore (glob-style)
	GitOnly     bool     `toml:"git_only"` // only show git repos (default true)
}

// BrowserConfig holds file browser settings.
type BrowserConfig struct {
	StartPath string `toml:"start_path"` // "~", "cwd", or absolute path
}

// Config holds all kmux configuration.
type Config struct {
	Daemon   DaemonConfig   `toml:"daemon"`
	Kitty    KittyConfig    `toml:"kitty"`
	Projects ProjectsConfig `toml:"projects"`
	Browser  BrowserConfig  `toml:"browser"`
}

// DefaultConfig returns configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Daemon: DaemonConfig{
			AutoSaveInterval: 900,
		},
		Projects: ProjectsConfig{
			Directories: nil, // User must configure - no defaults
			MaxDepth:    2,
			Ignore:      nil,
			GitOnly:     true, // Only show git repos by default
		},
		Browser: BrowserConfig{
			StartPath: "~", // Start at home directory
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

	// Validate and fix invalid values
	if cfg.Daemon.AutoSaveInterval < 1 {
		cfg.Daemon.AutoSaveInterval = 900 // default
	}
	if cfg.Projects.MaxDepth < 1 {
		cfg.Projects.MaxDepth = 2 // default
	}

	return cfg, nil
}

// ExpandPath expands ~ to the user's home directory.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
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

// SaveConfig writes the config to the config file.
func SaveConfig(cfg *Config) error {
	configPath := filepath.Join(ConfigDir(), "config.toml")

	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// AddIgnorePattern adds a pattern to the ignore list and saves the config.
func AddIgnorePattern(pattern string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	// Check if already ignored
	for _, p := range cfg.Projects.Ignore {
		if p == pattern {
			return nil // Already ignored
		}
	}

	cfg.Projects.Ignore = append(cfg.Projects.Ignore, pattern)
	return SaveConfig(cfg)
}

// AddProjectDirectory adds a directory to the projects list and saves the config.
func AddProjectDirectory(dir string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	// Normalize path - use ~ for home directory prefix
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(dir, home) {
		dir = "~" + dir[len(home):]
	}

	// Check if already in list
	for _, d := range cfg.Projects.Directories {
		if d == dir {
			return nil // Already in list
		}
	}

	cfg.Projects.Directories = append(cfg.Projects.Directories, dir)
	return SaveConfig(cfg)
}

// BrowserStartPath returns the resolved starting path for the file browser.
func (c *Config) BrowserStartPath() string {
	path := c.Browser.StartPath
	if path == "" || path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if path == "cwd" {
		cwd, _ := os.Getwd()
		return cwd
	}
	return ExpandPath(path)
}
