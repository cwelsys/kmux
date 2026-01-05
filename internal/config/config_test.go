package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSocketPath_Default(t *testing.T) {
	os.Unsetenv("KMUX_SOCKET")
	os.Unsetenv("KMUX_TMPDIR")

	path := SocketPath()

	// Should be /tmp/kmux-{UID}/default
	if filepath.Base(path) != "default" {
		t.Errorf("SocketPath() = %q, want basename 'default'", path)
	}
	dir := filepath.Dir(path)
	if filepath.Dir(dir) != "/tmp" {
		t.Errorf("SocketPath() parent should be under /tmp, got %q", dir)
	}
}

func TestSocketPath_Override(t *testing.T) {
	os.Setenv("KMUX_SOCKET", "/custom/path/sock")
	defer os.Unsetenv("KMUX_SOCKET")

	path := SocketPath()
	if path != "/custom/path/sock" {
		t.Errorf("SocketPath() = %q, want /custom/path/sock", path)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Daemon.AutoSaveInterval != 900 {
		t.Errorf("AutoSaveInterval = %d, want 900", cfg.Daemon.AutoSaveInterval)
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
[daemon]
auto_save_interval = 600

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

	if cfg.Daemon.AutoSaveInterval != 600 {
		t.Errorf("AutoSaveInterval = %d, want 600", cfg.Daemon.AutoSaveInterval)
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
	if cfg.Daemon.AutoSaveInterval != 900 {
		t.Errorf("AutoSaveInterval = %d, want 900", cfg.Daemon.AutoSaveInterval)
	}
}

func TestLoadConfigPartial(t *testing.T) {
	// Config with only daemon section, kitty section omitted
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	content := `
[daemon]
auto_save_interval = 300
`
	os.WriteFile(configPath, []byte(content), 0644)

	os.Setenv("KMUX_CONFIG_DIR", dir)
	defer os.Unsetenv("KMUX_CONFIG_DIR")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Should use custom daemon value
	if cfg.Daemon.AutoSaveInterval != 300 {
		t.Errorf("AutoSaveInterval = %d, want 300", cfg.Daemon.AutoSaveInterval)
	}
	// Kitty.Socket should remain empty (default)
	if cfg.Kitty.Socket != "" {
		t.Errorf("Kitty.Socket = %q, want empty (default)", cfg.Kitty.Socket)
	}
}
