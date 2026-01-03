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

	if cfg.WatchInterval != 5 {
		t.Errorf("WatchInterval = %d, want 5", cfg.WatchInterval)
	}
	if cfg.AutoSaveInterval != 900 {
		t.Errorf("AutoSaveInterval = %d, want 900", cfg.AutoSaveInterval)
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
