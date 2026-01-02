package model

import (
	"strconv"
	"time"
)

// Session represents a kmux session with its layout and state.
type Session struct {
	Name        string    `json:"name"`
	Host        string    `json:"host"`
	SavedAt     time.Time `json:"saved_at"`
	Tabs        []Tab     `json:"tabs"`
	ZmxSessions []string  `json:"zmx_sessions"`
}

// Tab represents a kitty tab containing windows.
type Tab struct {
	Title     string     `json:"title"`
	Layout    string     `json:"layout"`
	Windows   []Window   `json:"windows"`
	SplitRoot *SplitNode `json:"split_root,omitempty"` // nil for single-window tabs
}

// Window represents a single pane in a tab.
type Window struct {
	CWD       string `json:"cwd"`
	Command   string `json:"command,omitempty"`
	Ephemeral bool   `json:"ephemeral,omitempty"`
}

// SplitNode represents a node in the split tree.
// Leaf nodes have WindowIdx set. Branch nodes have Children set.
type SplitNode struct {
	// Leaf node: index into Tab.Windows
	WindowIdx *int `json:"window_idx,omitempty"`

	// Branch node fields
	Horizontal bool          `json:"horizontal,omitempty"` // true=left/right, false=top/bottom
	Bias       float64       `json:"bias,omitempty"`       // space ratio (default 0.5)
	Children   [2]*SplitNode `json:"children,omitempty"`   // [first, second]
}

// IsLeaf returns true if this is a leaf node (has window, no children).
func (n *SplitNode) IsLeaf() bool {
	return n.WindowIdx != nil
}

// ZmxSessionName returns the zmx session name for a window at the given position.
func (s *Session) ZmxSessionName(tabIdx, winIdx int) string {
	return s.Name + "." + strconv.Itoa(tabIdx) + "." + strconv.Itoa(winIdx)
}
