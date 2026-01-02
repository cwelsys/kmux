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
