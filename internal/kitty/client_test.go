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
