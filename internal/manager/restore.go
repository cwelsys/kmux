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

// RestoreCallback is called for each window to be created.
// launchType is "tab" for first window, "hsplit" or "vsplit" for splits.
type RestoreCallback func(win model.Window, launchType string)

// traverseForRestore does DFS traversal of split tree, calling callback for each leaf.
// parentSplit is the split direction from parent (empty for first window).
func traverseForRestore(node *model.SplitNode, parentSplit string, windows []model.Window, callback RestoreCallback) {
	if node == nil {
		return
	}

	if node.IsLeaf() {
		idx := *node.WindowIdx
		if idx < 0 || idx >= len(windows) {
			return // silently skip invalid indices
		}
		win := windows[idx]
		launchType := parentSplit
		if launchType == "" {
			launchType = "tab" // first window creates the tab
		}
		callback(win, launchType)
		return
	}

	// Determine split type for second child
	// In kitty layout_state: horizontal=true means left/right (vsplit), false means top/bottom (hsplit)
	splitType := "vsplit"
	if !node.Horizontal {
		splitType = "hsplit"
	}

	// First child inherits parent's split type
	traverseForRestore(node.Children[0], parentSplit, windows, callback)

	// Second child uses this node's split type
	traverseForRestore(node.Children[1], splitType, windows, callback)
}

// RestoreTab creates kitty windows for a tab with split layout.
// Returns the window creations for mapping and the first window ID for focusing.
func RestoreTab(
	k *kitty.Client,
	session *model.Session,
	tabIdx int,
	tab model.Tab,
) ([]WindowCreate, int, error) {
	var creations []WindowCreate
	var firstWindowID int
	var lastWindowID int
	windowIdx := 0

	createWindow := func(win model.Window, launchType string) error {
		// Use saved ZmxName if available, otherwise generate
		zmxName := win.ZmxName
		if zmxName == "" {
			zmxName = session.ZmxSessionName(tabIdx, windowIdx)
		}
		// Pass session name for cleanup callback
		zmxCmd := zmx.AttachCmd(zmxName, session.Name, win.Command)

		// Convert launchType to kitty location
		location := ""
		launchTypeKitty := launchType
		if launchType == "hsplit" || launchType == "vsplit" {
			launchTypeKitty = "window"
			location = launchType
		}

		opts := kitty.LaunchOpts{
			Type:     launchTypeKitty,
			CWD:      win.CWD,
			Title:    tab.Title,
			Location: location,
			Cmd:      zmxCmd,
			Env:      nil,
		}

		id, err := k.Launch(opts)
		if err != nil {
			return err
		}

		// Record creation for mapping
		creations = append(creations, WindowCreate{
			KittyWindowID: id,
			ZmxName:       zmxName,
		})

		if windowIdx == 0 {
			firstWindowID = id
		}
		lastWindowID = id
		windowIdx++

		session.ZmxSessions = append(session.ZmxSessions, zmxName)
		return nil
	}

	// Handle simple kitty layouts (tall, fat, grid, horizontal, vertical)
	// These layouts don't need a SplitRoot tree - kitty arranges windows automatically
	if isSimpleLayout(tab.Layout) && tab.SplitRoot == nil {
		for i, win := range tab.Windows {
			if i == 0 {
				// Create first window as a new tab
				if err := createWindow(win, "tab"); err != nil {
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
				if err := createWindow(win, "window"); err != nil {
					return nil, 0, err
				}
			}
		}
		return creations, firstWindowID, nil
	}

	// Handle single window (no splits)
	if tab.SplitRoot == nil || len(tab.Windows) <= 1 {
		for _, win := range tab.Windows {
			if err := createWindow(win, "tab"); err != nil {
				return nil, 0, err
			}
		}
		return creations, firstWindowID, nil
	}

	// Traverse split tree
	var restoreErr error
	traverseForRestore(tab.SplitRoot, "", tab.Windows, func(win model.Window, launchType string) {
		if restoreErr != nil {
			return
		}
		// Focus last window before creating split
		if launchType != "tab" && lastWindowID > 0 {
			if err := k.FocusWindow(lastWindowID); err != nil {
				restoreErr = err
				return
			}
		}
		restoreErr = createWindow(win, launchType)
	})

	return creations, firstWindowID, restoreErr
}
