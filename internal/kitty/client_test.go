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
	// Real kitty structure: pairs contain GROUP IDs that reference all_windows.window_groups
	jsonData := `[{
		"id": 1,
		"tabs": [{
			"id": 1,
			"title": "test",
			"layout": "splits",
			"layout_state": {
				"all_windows": {
					"window_groups": [
						{"id": 31, "window_ids": [33]},
						{"id": 41, "window_ids": [45]},
						{"id": 42, "window_ids": [46]}
					]
				},
				"pairs": {
					"one": 31,
					"two": {"horizontal": false, "one": 41, "two": 42}
				}
			},
			"windows": [
				{"id": 33, "cwd": "/", "env": {}},
				{"id": 45, "cwd": "/", "env": {}},
				{"id": 46, "cwd": "/", "env": {}}
			]
		}]
	}]`

	state, err := ParseState([]byte(jsonData))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	tab := state[0].Tabs[0]

	// Verify all_windows is parsed
	if tab.LayoutState.AllWindows == nil {
		t.Fatal("all_windows should not be nil")
	}
	if len(tab.LayoutState.AllWindows.WindowGroups) != 3 {
		t.Errorf("expected 3 window groups, got %d", len(tab.LayoutState.AllWindows.WindowGroups))
	}

	// Verify GroupToWindowID mapping
	groupToWindow := tab.LayoutState.AllWindows.GroupToWindowID()
	if groupToWindow[31] != 33 {
		t.Errorf("group 31 should map to window 33, got %d", groupToWindow[31])
	}
	if groupToWindow[41] != 45 {
		t.Errorf("group 41 should map to window 45, got %d", groupToWindow[41])
	}

	pairs := tab.LayoutState.Pairs
	if pairs == nil {
		t.Fatal("pairs should not be nil")
	}

	// Verify root structure
	if pairs.Horizontal != true {
		t.Error("root should be horizontal (default)")
	}

	// Verify first child is group 31
	if pairs.One == nil || pairs.One.GroupID == nil || *pairs.One.GroupID != 31 {
		t.Error("first child should be group 31")
	}

	// Verify second child is nested pair (hsplit)
	if pairs.Two == nil || pairs.Two.GroupID != nil {
		t.Error("second child should be nested pair")
	}
	if pairs.Two.Horizontal != false {
		t.Error("nested split should be horizontal=false (hsplit)")
	}
}
