package kitty

import (
	"testing"
)

func TestPairToSplitNode(t *testing.T) {
	// Build a simple Pair tree: horizontal split with windows 42 and 43
	id42, id43 := 42, 43
	pair := &Pair{
		Horizontal: true,
		Bias:       0.7,
		One:        &Pair{WindowID: &id42},
		Two:        &Pair{WindowID: &id43},
	}

	// Map window IDs to indices
	windowIDToIdx := map[int]int{42: 0, 43: 1}

	node, err := PairToSplitNode(pair, windowIDToIdx)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if node.IsLeaf() {
		t.Error("root should be branch")
	}
	if !node.Horizontal {
		t.Error("should be horizontal")
	}
	if node.Bias != 0.7 {
		t.Errorf("bias = %v, want 0.7", node.Bias)
	}

	// Check children
	if !node.Children[0].IsLeaf() || *node.Children[0].WindowIdx != 0 {
		t.Error("first child should be leaf with idx 0")
	}
	if !node.Children[1].IsLeaf() || *node.Children[1].WindowIdx != 1 {
		t.Error("second child should be leaf with idx 1")
	}
}

func TestPairToSplitNode_Nested(t *testing.T) {
	// Tree: vsplit(win0, hsplit(win1, win2))
	id0, id1, id2 := 10, 20, 30
	pair := &Pair{
		Horizontal: false, // vertical split
		One:        &Pair{WindowID: &id0},
		Two: &Pair{
			Horizontal: true,
			One:        &Pair{WindowID: &id1},
			Two:        &Pair{WindowID: &id2},
		},
	}

	windowIDToIdx := map[int]int{10: 0, 20: 1, 30: 2}

	node, err := PairToSplitNode(pair, windowIDToIdx)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Verify tree structure
	if node.Horizontal {
		t.Error("root should be vertical")
	}
	if !node.Children[0].IsLeaf() {
		t.Error("first child should be leaf")
	}
	if node.Children[1].IsLeaf() {
		t.Error("second child should be branch")
	}
	if !node.Children[1].Horizontal {
		t.Error("nested split should be horizontal")
	}
}
