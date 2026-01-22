package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Kitty.Socket != "" {
		t.Errorf("Kitty.Socket = %q, want empty string", cfg.Kitty.Socket)
	}
	if cfg.Projects.MaxDepth != 2 {
		t.Errorf("Projects.MaxDepth = %d, want 2", cfg.Projects.MaxDepth)
	}
}

func TestConfigDir(t *testing.T) {
	// Clear env for clean test
	os.Unsetenv("KMUX_CONFIG_DIR")
	os.Unsetenv("XDG_CONFIG_HOME")

	dir := ConfigDir()

	// Should default to ~/.config/kmux
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "kmux")
	if dir != expected {
		t.Errorf("ConfigDir() = %q, want %q", dir, expected)
	}
}

func TestConfigDirWithEnv(t *testing.T) {
	os.Setenv("KMUX_CONFIG_DIR", "/custom/config")
	defer os.Unsetenv("KMUX_CONFIG_DIR")

	dir := ConfigDir()
	if dir != "/custom/config" {
		t.Errorf("ConfigDir() = %q, want %q", dir, "/custom/config")
	}
}

func TestConfigDirWithXDG(t *testing.T) {
	os.Unsetenv("KMUX_CONFIG_DIR")
	os.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	defer os.Unsetenv("XDG_CONFIG_HOME")

	dir := ConfigDir()
	if dir != "/xdg/config/kmux" {
		t.Errorf("ConfigDir() = %q, want %q", dir, "/xdg/config/kmux")
	}
}

func TestLoadConfig(t *testing.T) {
	// Create temp config file
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	content := `
[kitty]
socket = "/tmp/custom-kitty"
`
	os.WriteFile(configPath, []byte(content), 0644)

	os.Setenv("KMUX_CONFIG_DIR", dir)
	defer os.Unsetenv("KMUX_CONFIG_DIR")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Kitty.Socket != "/tmp/custom-kitty" {
		t.Errorf("Kitty.Socket = %q, want %q", cfg.Kitty.Socket, "/tmp/custom-kitty")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	// Empty dir - no config file
	dir := t.TempDir()
	os.Setenv("KMUX_CONFIG_DIR", dir)
	defer os.Unsetenv("KMUX_CONFIG_DIR")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Should use defaults
	if cfg.Kitty.Socket != "" {
		t.Errorf("Kitty.Socket = %q, want empty string", cfg.Kitty.Socket)
	}
	if cfg.Projects.MaxDepth != 2 {
		t.Errorf("Projects.MaxDepth = %d, want 2", cfg.Projects.MaxDepth)
	}
}

func TestLoadConfigPartial(t *testing.T) {
	// Config with only projects section, kitty section omitted
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	content := `
[projects]
max_depth = 4
`
	os.WriteFile(configPath, []byte(content), 0644)

	os.Setenv("KMUX_CONFIG_DIR", dir)
	defer os.Unsetenv("KMUX_CONFIG_DIR")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Should use custom projects value
	if cfg.Projects.MaxDepth != 4 {
		t.Errorf("Projects.MaxDepth = %d, want 4", cfg.Projects.MaxDepth)
	}
	// Kitty.Socket should use default (empty)
	if cfg.Kitty.Socket != "" {
		t.Errorf("Kitty.Socket = %q, want empty string (default)", cfg.Kitty.Socket)
	}
}
