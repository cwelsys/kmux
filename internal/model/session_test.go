package model

import (
	"testing"
	"time"
)

func TestSessionBasics(t *testing.T) {
	s := Session{
		Name:    "myproject",
		Host:    "local",
		SavedAt: time.Now(),
		Tabs: []Tab{
			{
				Title:  "editor",
				Layout: "splits",
				Windows: []Window{
					{CWD: "/home/user/src", Command: "nvim ."},
				},
			},
		},
	}

	if s.Name != "myproject" {
		t.Errorf("expected name 'myproject', got '%s'", s.Name)
	}
	if len(s.Tabs) != 1 {
		t.Errorf("expected 1 tab, got %d", len(s.Tabs))
	}
	if len(s.Tabs[0].Windows) != 1 {
		t.Errorf("expected 1 window, got %d", len(s.Tabs[0].Windows))
	}
}

func TestZmxSessionName(t *testing.T) {
	s := Session{Name: "myproject"}

	tests := []struct {
		tabIdx, winIdx int
		expected       string
	}{
		{0, 0, "myproject.0.0"},
		{0, 1, "myproject.0.1"},
		{1, 0, "myproject.1.0"},
		{2, 5, "myproject.2.5"},
	}

	for _, tt := range tests {
		got := s.ZmxSessionName(tt.tabIdx, tt.winIdx)
		if got != tt.expected {
			t.Errorf("ZmxSessionName(%d, %d) = %s, want %s", tt.tabIdx, tt.winIdx, got, tt.expected)
		}
	}
}

func TestSplitNode_IsLeaf(t *testing.T) {
	idx := 0
	leaf := &SplitNode{WindowIdx: &idx}
	if !leaf.IsLeaf() {
		t.Error("expected leaf node")
	}

	branch := &SplitNode{
		Horizontal: true,
		Children:   [2]*SplitNode{{WindowIdx: &idx}, {WindowIdx: &idx}},
	}
	if branch.IsLeaf() {
		t.Error("expected branch node")
	}
}
