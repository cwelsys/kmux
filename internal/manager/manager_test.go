package manager

import (
	"testing"

	"github.com/cwel/kmux/internal/kitty"
)

func TestDeriveSession(t *testing.T) {
	// Simulated kitty state
	state := kitty.KittyState{
		{
			ID:       1,
			IsActive: true,
			Tabs: []kitty.Tab{
				{
					ID:       1,
					IsActive: true,
					Title:    "editor",
					Layout:   "splits",
					Windows: []kitty.Window{
						{
							ID:       1,
							IsActive: true,
							CWD:      "/home/user/project",
							Env:      map[string]string{"KMUX_SESSION": "myproject"},
							ForegroundProcesses: []kitty.ForegroundProcess{
								{Cmdline: []string{"nvim", "."}},
							},
						},
						{
							ID:  2,
							CWD: "/home/user/project",
							Env: map[string]string{"KMUX_SESSION": "myproject"},
							ForegroundProcesses: []kitty.ForegroundProcess{
								{Cmdline: []string{"/bin/zsh"}},
							},
						},
					},
				},
			},
		},
	}

	session := DeriveSession("myproject", state)

	if session.Name != "myproject" {
		t.Errorf("Name = %s, want myproject", session.Name)
	}
	if len(session.Tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(session.Tabs))
	}
	if len(session.Tabs[0].Windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(session.Tabs[0].Windows))
	}

	// Check that nvim is captured as command
	if session.Tabs[0].Windows[0].Command != "nvim ." {
		t.Errorf("Window 0 command = %s, want 'nvim .'", session.Tabs[0].Windows[0].Command)
	}
}

func TestDeriveSession_WithSplits(t *testing.T) {
	// Build kitty state with splits layout using real structure
	// Groups 31 and 32 contain windows 42 and 43
	group31, group32 := 31, 32
	state := kitty.KittyState{
		{
			ID: 1,
			Tabs: []kitty.Tab{
				{
					ID:     1,
					Title:  "dev",
					Layout: "splits",
					LayoutState: kitty.LayoutState{
						AllWindows: &kitty.AllWindows{
							WindowGroups: []kitty.WindowGroup{
								{ID: 31, WindowIDs: []int{42}},
								{ID: 32, WindowIDs: []int{43}},
							},
						},
						Pairs: &kitty.Pair{
							Horizontal: true,
							Bias:       0.7,
							One:        &kitty.Pair{GroupID: &group31},
							Two:        &kitty.Pair{GroupID: &group32},
						},
					},
					Windows: []kitty.Window{
						{ID: 42, CWD: "/project", Env: map[string]string{"KMUX_SESSION": "test"}},
						{ID: 43, CWD: "/project/src", Env: map[string]string{"KMUX_SESSION": "test"}},
					},
				},
			},
		},
	}

	session := DeriveSession("test", state)

	if len(session.Tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(session.Tabs))
	}

	tab := session.Tabs[0]
	if len(tab.Windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(tab.Windows))
	}

	if tab.SplitRoot == nil {
		t.Fatal("expected SplitRoot to be set")
	}

	if !tab.SplitRoot.Horizontal {
		t.Error("split should be horizontal")
	}
	if tab.SplitRoot.Bias != 0.7 {
		t.Errorf("bias = %v, want 0.7", tab.SplitRoot.Bias)
	}
}
