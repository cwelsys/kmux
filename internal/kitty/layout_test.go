package kitty

import (
	"testing"
)

func TestPairToSplitNode(t *testing.T) {
	// Simulates real kitty structure:
	// - Group 31 contains window 42
	// - Group 32 contains window 43
	// - Pairs references group IDs, not window IDs
	group31, group32 := 31, 32
	pair := &Pair{
		Horizontal: true,
		Bias:       0.7,
		One:        &Pair{GroupID: &group31},
		Two:        &Pair{GroupID: &group32},
	}

	// Group ID → Window ID (from AllWindows.window_groups)
	groupToWindowID := map[int]int{31: 42, 32: 43}

	// Window ID → index in Tab.Windows
	windowIDToIdx := map[int]int{42: 0, 43: 1}

	node, err := PairToSplitNode(pair, groupToWindowID, windowIDToIdx)
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
	// Simulates: vsplit(group31, hsplit(group41, group42))
	// Like real kitty output from kitty_with_nested_splits.json
	group31, group41, group42 := 31, 41, 42
	pair := &Pair{
		Horizontal: true, // kitty default for vsplit at root
		One:        &Pair{GroupID: &group31},
		Two: &Pair{
			Horizontal: false, // hsplit
			One:        &Pair{GroupID: &group41},
			Two:        &Pair{GroupID: &group42},
		},
	}

	// Group → Window mapping
	groupToWindowID := map[int]int{31: 33, 41: 45, 42: 46}

	// Window → index
	windowIDToIdx := map[int]int{33: 0, 45: 1, 46: 2}

	node, err := PairToSplitNode(pair, groupToWindowID, windowIDToIdx)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Verify tree structure
	if !node.Horizontal {
		t.Error("root should be horizontal (kitty's default)")
	}
	if !node.Children[0].IsLeaf() {
		t.Error("first child should be leaf")
	}
	if node.Children[1].IsLeaf() {
		t.Error("second child should be branch")
	}
	if node.Children[1].Horizontal {
		t.Error("nested split should be vertical (horizontal=false)")
	}
}

func TestPairToSplitNode_MissingGroup(t *testing.T) {
	group31 := 31
	pair := &Pair{GroupID: &group31}

	// Group 31 not in map
	groupToWindowID := map[int]int{}
	windowIDToIdx := map[int]int{}

	_, err := PairToSplitNode(pair, groupToWindowID, windowIDToIdx)
	if err == nil {
		t.Error("expected error for unknown group ID")
	}
}

func TestPairToSplitNode_MissingWindow(t *testing.T) {
	group31 := 31
	pair := &Pair{GroupID: &group31}

	// Group exists but window not in windowIDToIdx
	groupToWindowID := map[int]int{31: 42}
	windowIDToIdx := map[int]int{}

	_, err := PairToSplitNode(pair, groupToWindowID, windowIDToIdx)
	if err == nil {
		t.Error("expected error for unknown window ID")
	}
}
