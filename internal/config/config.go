package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

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

// HostConfig holds configuration for a remote host.
// Hosts are referenced by their SSH config alias - all auth/proxy is handled by SSH.
type HostConfig struct {
	ZmxPath string `toml:"zmx_path"` // optional path to zmx on remote (default: "zmx")
}

// Config holds all kmux configuration.
type Config struct {
	Kitty    KittyConfig           `toml:"kitty"`
	Projects ProjectsConfig        `toml:"projects"`
	Browser  BrowserConfig         `toml:"browser"`
	Hosts    map[string]HostConfig `toml:"hosts"` // SSH alias -> host config
}

// DefaultConfig returns configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
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

// HostNames returns a sorted list of configured host aliases.
func (c *Config) HostNames() []string {
	if c.Hosts == nil {
		return nil
	}
	names := make([]string, 0, len(c.Hosts))
	for name := range c.Hosts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetHost returns the config for a host, or nil if not configured.
func (c *Config) GetHost(name string) *HostConfig {
	if c.Hosts == nil {
		return nil
	}
	if cfg, ok := c.Hosts[name]; ok {
		return &cfg
	}
	return nil
}
