package store

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cwel/kmux/internal/config"
)

// LoadLayout loads a layout by name, searching user layouts first, then bundled.
func LoadLayout(name string) (*config.Layout, error) {
	// Search order: user layouts → bundled layouts
	paths := []string{
		filepath.Join(config.ConfigDir(), "layouts", name+".yaml"),
		filepath.Join(config.DataDir(), "layouts", name+".yaml"),
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read layout %s: %w", path, err)
		}

		layout, err := config.ParseLayout(data)
		if err != nil {
			return nil, fmt.Errorf("parse layout %s: %w", path, err)
		}

		if err := layout.Validate(); err != nil {
			return nil, fmt.Errorf("validate layout %s: %w", path, err)
		}

		return layout, nil
	}

	return nil, fmt.Errorf("layout not found: %s", name)
}

// ListLayouts returns available layout names.
func ListLayouts() ([]string, error) {
	seen := make(map[string]bool)
	var layouts []string

	dirs := []string{
		filepath.Join(config.ConfigDir(), "layouts"),
		filepath.Join(config.DataDir(), "layouts"),
	}

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if filepath.Ext(name) != ".yaml" {
				continue
			}
			baseName := name[:len(name)-5] // remove .yaml
			if !seen[baseName] {
				seen[baseName] = true
				layouts = append(layouts, baseName)
			}
		}
	}

	return layouts, nil
}
