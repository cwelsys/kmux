package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLayout(t *testing.T) {
	// Create temp layout directory
	configDir := t.TempDir()
	layoutDir := filepath.Join(configDir, "layouts")
	os.MkdirAll(layoutDir, 0755)

	// Write test layout
	layoutContent := `
name: test
description: Test layout
tabs:
  - title: main
    layout: tall
    panes:
      - ""
`
	os.WriteFile(filepath.Join(layoutDir, "test.yaml"), []byte(layoutContent), 0644)

	os.Setenv("KMUX_CONFIG_DIR", configDir)
	defer os.Unsetenv("KMUX_CONFIG_DIR")

	layout, err := LoadLayout("test")
	if err != nil {
		t.Fatalf("LoadLayout() error = %v", err)
	}

	if layout.Name != "test" {
		t.Errorf("Name = %q, want %q", layout.Name, "test")
	}
}

func TestLoadLayoutNotFound(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("KMUX_CONFIG_DIR", dir)
	os.Setenv("KMUX_DATA_DIR", dir)
	defer os.Unsetenv("KMUX_CONFIG_DIR")
	defer os.Unsetenv("KMUX_DATA_DIR")

	_, err := LoadLayout("nonexistent")
	if err == nil {
		t.Error("LoadLayout() expected error for nonexistent layout")
	}
}

func TestLoadLayoutResolutionOrder(t *testing.T) {
	// User layouts should take precedence over bundled
	configDir := t.TempDir()
	dataDir := t.TempDir()

	userLayoutDir := filepath.Join(configDir, "layouts")
	bundledLayoutDir := filepath.Join(dataDir, "layouts")
	os.MkdirAll(userLayoutDir, 0755)
	os.MkdirAll(bundledLayoutDir, 0755)

	// Write user layout
	userLayout := `
name: custom
description: User version
tabs:
  - title: user
    layout: tall
    panes:
      - ""
`
	os.WriteFile(filepath.Join(userLayoutDir, "custom.yaml"), []byte(userLayout), 0644)

	// Write bundled layout with same name
	bundledLayout := `
name: custom
description: Bundled version
tabs:
  - title: bundled
    layout: fat
    panes:
      - ""
`
	os.WriteFile(filepath.Join(bundledLayoutDir, "custom.yaml"), []byte(bundledLayout), 0644)

	os.Setenv("KMUX_CONFIG_DIR", configDir)
	os.Setenv("KMUX_DATA_DIR", dataDir)
	defer os.Unsetenv("KMUX_CONFIG_DIR")
	defer os.Unsetenv("KMUX_DATA_DIR")

	layout, err := LoadLayout("custom")
	if err != nil {
		t.Fatalf("LoadLayout() error = %v", err)
	}

	// Should load user version
	if layout.Description != "User version" {
		t.Errorf("Description = %q, want %q", layout.Description, "User version")
	}
}
