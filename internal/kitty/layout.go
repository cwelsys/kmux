package kitty

import (
	"fmt"

	"github.com/cwel/kmux/internal/model"
)

// PairToSplitNode converts kitty's Pair tree to our SplitNode tree.
// groupToWindowID maps group IDs (from pairs) to window IDs (from AllWindows).
// windowIDToIdx maps window IDs to indices in Tab.Windows.
func PairToSplitNode(pair *Pair, groupToWindowID, windowIDToIdx map[int]int) (*model.SplitNode, error) {
	if pair == nil {
		return nil, nil
	}

	// Leaf node - GroupID references a window group
	if pair.GroupID != nil {
		groupID := *pair.GroupID

		// Dereference group ID to window ID
		windowID, ok := groupToWindowID[groupID]
		if !ok {
			return nil, fmt.Errorf("unknown group ID: %d", groupID)
		}

		// Map window ID to index
		idx, ok := windowIDToIdx[windowID]
		if !ok {
			return nil, fmt.Errorf("unknown window ID: %d (from group %d)", windowID, groupID)
		}
		return &model.SplitNode{WindowIdx: &idx}, nil
	}

	// Branch node
	one, err := PairToSplitNode(pair.One, groupToWindowID, windowIDToIdx)
	if err != nil {
		return nil, err
	}
	two, err := PairToSplitNode(pair.Two, groupToWindowID, windowIDToIdx)
	if err != nil {
		return nil, err
	}

	return &model.SplitNode{
		Horizontal: pair.Horizontal,
		Bias:       pair.Bias,
		Children:   [2]*model.SplitNode{one, two},
	}, nil
}
