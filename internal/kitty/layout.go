package kitty

import (
	"fmt"

	"github.com/cwel/kmux/internal/model"
)

// PairToSplitNode converts kitty's Pair tree to our SplitNode tree.
// windowIDToIdx maps kitty window IDs to indices in Tab.Windows.
func PairToSplitNode(pair *Pair, windowIDToIdx map[int]int) (*model.SplitNode, error) {
	if pair == nil {
		return nil, nil
	}

	// Leaf node
	if pair.WindowID != nil {
		idx, ok := windowIDToIdx[*pair.WindowID]
		if !ok {
			return nil, fmt.Errorf("unknown window ID: %d", *pair.WindowID)
		}
		return &model.SplitNode{WindowIdx: &idx}, nil
	}

	// Branch node
	one, err := PairToSplitNode(pair.One, windowIDToIdx)
	if err != nil {
		return nil, err
	}
	two, err := PairToSplitNode(pair.Two, windowIDToIdx)
	if err != nil {
		return nil, err
	}

	return &model.SplitNode{
		Horizontal: pair.Horizontal,
		Bias:       pair.Bias,
		Children:   [2]*model.SplitNode{one, two},
	}, nil
}
