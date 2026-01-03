package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Layout defines a session template.
type Layout struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Tabs        []LayoutTab `yaml:"tabs"`
}

// LayoutTab defines a tab within a layout.
type LayoutTab struct {
	Title    string   `yaml:"title"`
	Layout   string   `yaml:"layout"`   // tall, fat, grid, horizontal, vertical
	Bias     int      `yaml:"bias"`     // percentage for main pane (default 50)
	FullSize int      `yaml:"full_size"` // number of "main" panes (default 1)
	Panes    []string `yaml:"panes"`    // commands for each pane
}

// ValidLayouts lists supported kitty layouts.
var ValidLayouts = map[string]bool{
	"tall":       true,
	"fat":        true,
	"grid":       true,
	"horizontal": true,
	"vertical":   true,
}

// ParseLayout parses a YAML layout definition.
func ParseLayout(data []byte) (*Layout, error) {
	var layout Layout
	if err := yaml.Unmarshal(data, &layout); err != nil {
		return nil, fmt.Errorf("parse layout: %w", err)
	}
	return &layout, nil
}

// Validate checks that the layout tab has valid settings.
func (t *LayoutTab) Validate() error {
	if t.Layout == "" {
		return fmt.Errorf("layout type required")
	}
	if !ValidLayouts[t.Layout] {
		return fmt.Errorf("invalid layout type: %q (valid: tall, fat, grid, horizontal, vertical)", t.Layout)
	}
	if len(t.Panes) == 0 {
		return fmt.Errorf("at least one pane required")
	}
	// Only validate bias if it's set (0 means use default)
	if t.Bias != 0 && (t.Bias < 10 || t.Bias > 90) {
		return fmt.Errorf("bias must be between 10 and 90 (got %d)", t.Bias)
	}
	return nil
}

// Validate checks the entire layout.
func (l *Layout) Validate() error {
	if l.Name == "" {
		return fmt.Errorf("layout name required")
	}
	if len(l.Tabs) == 0 {
		return fmt.Errorf("at least one tab required")
	}
	for i, tab := range l.Tabs {
		if err := tab.Validate(); err != nil {
			return fmt.Errorf("tab %d: %w", i, err)
		}
	}
	return nil
}
