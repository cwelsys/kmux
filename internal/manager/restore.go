package manager

import (
	"github.com/cwel/kmux/internal/kitty"
	"github.com/cwel/kmux/internal/model"
	"github.com/cwel/kmux/internal/zmx"
)

// isSimpleLayout returns true for kitty built-in layouts that don't need a SplitRoot tree.
func isSimpleLayout(layout string) bool {
	simple := map[string]bool{
		"tall":       true,
		"fat":        true,
		"grid":       true,
		"horizontal": true,
		"vertical":   true,
	}
	return simple[layout]
}

// WindowCreate holds info about a created window for mapping.
type WindowCreate struct {
	KittyWindowID int
	ZmxName       string
}

// SplitInfo holds split type and bias for window creation.
type SplitInfo struct {
	Type string // "tab", "hsplit", "vsplit"
	Bias int    // 0-100, percentage for new window (0 = default/equal)
}

// windowCreator encapsulates window creation state during restore.
type windowCreator struct {
	k           *kitty.Client
	session     *model.Session
	tabIdx      int
	tab         model.Tab
	windowIdx   int
	creations   []WindowCreate
	firstWinID  int
	tabLocation string // location for first tab creation (e.g., "before" for before pinned tabs)
}

// createWindow creates a single kitty window and records the creation.
// Returns the kitty window ID of the created window.
func (wc *windowCreator) createWindow(win model.Window, split SplitInfo) (int, error) {
	// Use saved ZmxName if available, otherwise generate
	zmxName := win.ZmxName
	if zmxName == "" {
		zmxName = wc.session.ZmxSessionName(wc.tabIdx, wc.windowIdx)
	}
	zmxCmd := zmx.AttachCmd(zmxName, win.Command)

	// Convert split type to kitty location
	location := ""
	launchType := split.Type
	if launchType == "hsplit" || launchType == "vsplit" {
		launchType = "window"
		location = split.Type
	} else if launchType == "tab" && wc.tabLocation != "" {
		// Use custom tab location (e.g., "before" for before pinned tabs)
		location = wc.tabLocation
	}

	opts := kitty.LaunchOpts{
		Type:     launchType,
		CWD:      win.CWD,
		Title:    wc.tab.Title,
		Location: location,
		Cmd:      zmxCmd,
		Env:      nil,
		Vars: map[string]string{
			"kmux_zmx":     zmxName,
			"kmux_session": wc.session.Name,
		},
		Bias: split.Bias,
	}

	id, err := wc.k.Launch(opts)
	if err != nil {
		return 0, err
	}

	// Record creation for mapping
	wc.creations = append(wc.creations, WindowCreate{
		KittyWindowID: id,
		ZmxName:       zmxName,
	})

	if wc.windowIdx == 0 {
		wc.firstWinID = id
	}
	wc.windowIdx++

	wc.session.ZmxSessions = append(wc.session.ZmxSessions, zmxName)
	return id, nil
}

// restoreSpine creates the "spine" of a subtree - following first-child path to a leaf.
// Returns the window ID of the created leaf.
func (wc *windowCreator) restoreSpine(node *model.SplitNode, parentSplit SplitInfo, windows []model.Window) (int, error) {
	if node == nil {
		return 0, nil
	}

	// Leaf node: create the window
	if node.IsLeaf() {
		idx := *node.WindowIdx
		if idx < 0 || idx >= len(windows) {
			return 0, nil
		}
		win := windows[idx]
		split := parentSplit
		if split.Type == "" {
			split.Type = "tab"
		}
		return wc.createWindow(win, split)
	}

	// Internal node: only follow first child path
	return wc.restoreSpine(node.Children[0], parentSplit, windows)
}

// fillSecondChildren creates all second children in a subtree.
// spineWinID is the window ID of the spine (first-child path) leaf in this subtree.
func (wc *windowCreator) fillSecondChildren(node *model.SplitNode, spineWinID int, windows []model.Window) error {
	if node == nil || node.IsLeaf() {
		return nil
	}

	// Determine split type for second child
	splitType := "vsplit"
	if !node.Horizontal {
		splitType = "hsplit"
	}

	// Calculate bias for second child
	bias := 0
	if node.Bias > 0 && node.Bias < 1 {
		bias = int((1 - node.Bias) * 100)
	}

	// Focus the spine window and create second child's spine
	if err := wc.k.FocusWindow(spineWinID); err != nil {
		return err
	}

	secondSpineID, err := wc.restoreSpine(node.Children[1], SplitInfo{Type: splitType, Bias: bias}, windows)
	if err != nil {
		return err
	}

	// Recursively fill second children in both subtrees
	// First child's spine is still spineWinID
	if err := wc.fillSecondChildren(node.Children[0], spineWinID, windows); err != nil {
		return err
	}
	// Second child's spine is secondSpineID
	if err := wc.fillSecondChildren(node.Children[1], secondSpineID, windows); err != nil {
		return err
	}

	return nil
}

// restoreSubtree restores a complete subtree using two-pass algorithm:
// 1. Create spines (first-child paths) - this establishes the split structure
// 2. Fill in second children
func (wc *windowCreator) restoreSubtree(node *model.SplitNode, parentSplit SplitInfo, windows []model.Window) (int, error) {
	if node == nil {
		return 0, nil
	}

	// Pass 1: Create the spine of this subtree
	spineWinID, err := wc.restoreSpine(node, parentSplit, windows)
	if err != nil {
		return 0, err
	}

	// Pass 2: Fill in all second children
	if err := wc.fillSecondChildren(node, spineWinID, windows); err != nil {
		return 0, err
	}

	return spineWinID, nil
}

// RestoreTabOpts holds options for RestoreTab.
type RestoreTabOpts struct {
	TabLocation string // location for tab creation (e.g., "before" for before pinned tabs)
}

// RestoreTab creates kitty windows for a tab with split layout.
// Returns the window creations for mapping and the first window ID for focusing.
func RestoreTab(
	k *kitty.Client,
	session *model.Session,
	tabIdx int,
	tab model.Tab,
	opts ...RestoreTabOpts,
) ([]WindowCreate, int, error) {
	var tabLocation string
	if len(opts) > 0 {
		tabLocation = opts[0].TabLocation
	}

	wc := &windowCreator{
		k:           k,
		session:     session,
		tabIdx:      tabIdx,
		tab:         tab,
		tabLocation: tabLocation,
	}

	// Handle simple kitty layouts (tall, fat, grid, horizontal, vertical)
	// These layouts don't need a SplitRoot tree - kitty arranges windows automatically
	if isSimpleLayout(tab.Layout) && tab.SplitRoot == nil {
		for i, win := range tab.Windows {
			if i == 0 {
				// Create first window as a new tab
				if _, err := wc.createWindow(win, SplitInfo{Type: "tab"}); err != nil {
					return nil, 0, err
				}
				// Set layout before creating additional windows
				if len(tab.Windows) > 1 {
					if err := k.GotoLayout(tab.Layout); err != nil {
						return nil, 0, err
					}
				}
			} else {
				// Subsequent windows - kitty places according to layout
				if _, err := wc.createWindow(win, SplitInfo{Type: "window"}); err != nil {
					return nil, 0, err
				}
			}
		}
		return wc.creations, wc.firstWinID, nil
	}

	// Handle single window (no splits)
	if tab.SplitRoot == nil || len(tab.Windows) <= 1 {
		for _, win := range tab.Windows {
			if _, err := wc.createWindow(win, SplitInfo{Type: "tab"}); err != nil {
				return nil, 0, err
			}
		}
		return wc.creations, wc.firstWinID, nil
	}

	// Restore split tree - this properly tracks subtree representatives
	// to ensure splits happen from the correct windows
	_, err := wc.restoreSubtree(tab.SplitRoot, SplitInfo{}, tab.Windows)
	if err != nil {
		return nil, 0, err
	}

	return wc.creations, wc.firstWinID, nil
}
