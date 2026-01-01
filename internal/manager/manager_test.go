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
