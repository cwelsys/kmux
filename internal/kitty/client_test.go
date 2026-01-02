package kitty

import (
	"testing"
)

func TestParseState(t *testing.T) {
	// Sample kitty @ ls output (simplified)
	jsonData := `[{
		"id": 1,
		"is_active": true,
		"tabs": [{
			"id": 1,
			"is_active": true,
			"title": "test",
			"layout": "splits",
			"windows": [{
				"id": 1,
				"is_active": true,
				"cwd": "/home/user",
				"pid": 1234,
				"cmdline": ["/bin/zsh"],
				"foreground_processes": [{
					"pid": 5678,
					"cwd": "/home/user",
					"cmdline": ["nvim", "."]
				}]
			}]
		}]
	}]`

	state, err := ParseState([]byte(jsonData))
	if err != nil {
		t.Fatalf("ParseState failed: %v", err)
	}

	if len(state) != 1 {
		t.Fatalf("expected 1 OS window, got %d", len(state))
	}
	if len(state[0].Tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(state[0].Tabs))
	}
	if len(state[0].Tabs[0].Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(state[0].Tabs[0].Windows))
	}

	win := state[0].Tabs[0].Windows[0]
	if win.CWD != "/home/user" {
		t.Errorf("CWD = %s, want /home/user", win.CWD)
	}
	if len(win.ForegroundProcesses) != 1 {
		t.Fatalf("expected 1 foreground process, got %d", len(win.ForegroundProcesses))
	}
	if win.ForegroundProcesses[0].Cmdline[0] != "nvim" {
		t.Errorf("foreground cmd = %s, want nvim", win.ForegroundProcesses[0].Cmdline[0])
	}
}

func TestParseState_WithSplits(t *testing.T) {
	jsonData := `[{
		"id": 1,
		"tabs": [{
			"id": 1,
			"title": "test",
			"layout": "splits",
			"layout_state": {
				"pairs": {
					"horizontal": true,
					"bias": 0.7,
					"one": 42,
					"two": {"one": 43, "two": 44}
				}
			},
			"windows": [
				{"id": 42, "cwd": "/", "env": {}},
				{"id": 43, "cwd": "/", "env": {}},
				{"id": 44, "cwd": "/", "env": {}}
			]
		}]
	}]`

	state, err := ParseState([]byte(jsonData))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	pairs := state[0].Tabs[0].LayoutState.Pairs
	if pairs == nil {
		t.Fatal("pairs should not be nil")
	}

	// Verify root is horizontal with bias
	if !pairs.Horizontal {
		t.Error("root should be horizontal")
	}
	if pairs.Bias != 0.7 {
		t.Errorf("bias = %v, want 0.7", pairs.Bias)
	}

	// Verify first child is window 42
	if pairs.One == nil || pairs.One.WindowID == nil || *pairs.One.WindowID != 42 {
		t.Error("first child should be window 42")
	}

	// Verify second child is nested pair
	if pairs.Two == nil || pairs.Two.WindowID != nil {
		t.Error("second child should be nested pair")
	}

	// Verify nested pair defaults (horizontal defaults to true when omitted in kitty)
	if pairs.Two.Horizontal != true {
		t.Errorf("nested horizontal = %v, want true (default)", pairs.Two.Horizontal)
	}
	if pairs.Two.Bias != 0.5 {
		t.Errorf("nested bias = %v, want 0.5 (default)", pairs.Two.Bias)
	}
}
