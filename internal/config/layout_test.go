package config

import (
	"testing"
)

func TestParseLayout(t *testing.T) {
	yaml := `
name: ide
description: Editor with shell sidebar

tabs:
  - title: dev
    layout: tall
    bias: 70
    panes:
      - nvim .
      - ""
      - lazygit
`

	layout, err := ParseLayout([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseLayout() error = %v", err)
	}

	if layout.Name != "ide" {
		t.Errorf("Name = %q, want %q", layout.Name, "ide")
	}
	if len(layout.Tabs) != 1 {
		t.Fatalf("len(Tabs) = %d, want 1", len(layout.Tabs))
	}

	tab := layout.Tabs[0]
	if tab.Title != "dev" {
		t.Errorf("Tab.Title = %q, want %q", tab.Title, "dev")
	}
	if tab.Layout != "tall" {
		t.Errorf("Tab.Layout = %q, want %q", tab.Layout, "tall")
	}
	if tab.Bias != 70 {
		t.Errorf("Tab.Bias = %d, want 70", tab.Bias)
	}
	if len(tab.Panes) != 3 {
		t.Fatalf("len(Panes) = %d, want 3", len(tab.Panes))
	}
	if tab.Panes[0] != "nvim ." {
		t.Errorf("Panes[0] = %q, want %q", tab.Panes[0], "nvim .")
	}
}

func TestLayoutValidation(t *testing.T) {
	tests := []struct {
		name    string
		layout  string
		bias    int
		wantErr bool
	}{
		{"valid tall", "tall", 0, false},
		{"valid fat", "fat", 0, false},
		{"valid grid", "grid", 0, false},
		{"valid horizontal", "horizontal", 0, false},
		{"valid vertical", "vertical", 0, false},
		{"invalid layout", "invalid", 0, true},
		{"empty layout", "", 0, true},
		{"bias too low", "tall", 5, true},
		{"bias too high", "tall", 95, true},
		{"bias valid low", "tall", 10, false},
		{"bias valid high", "tall", 90, false},
		{"bias zero default", "tall", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tab := LayoutTab{Layout: tt.layout, Bias: tt.bias, Panes: []string{""}}
			err := tab.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLayoutFullValidation(t *testing.T) {
	tests := []struct {
		name    string
		layout  Layout
		wantErr bool
	}{
		{
			name:    "empty name",
			layout:  Layout{Name: "", Tabs: []LayoutTab{{Layout: "tall", Panes: []string{""}}}},
			wantErr: true,
		},
		{
			name:    "no tabs",
			layout:  Layout{Name: "test", Tabs: []LayoutTab{}},
			wantErr: true,
		},
		{
			name: "invalid tab",
			layout: Layout{
				Name: "test",
				Tabs: []LayoutTab{{Layout: "invalid", Panes: []string{""}}},
			},
			wantErr: true,
		},
		{
			name: "valid layout",
			layout: Layout{
				Name: "test",
				Tabs: []LayoutTab{{Layout: "tall", Panes: []string{""}}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.layout.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
