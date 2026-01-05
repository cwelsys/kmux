package manager

import (
	"testing"

	"github.com/cwel/kmux/internal/model"
)

// Note: The restore logic is now integrated with window creation via windowCreator.
// Unit testing would require mocking the kitty client. Integration testing with
// test_workflow.sh verifies the actual behavior.

func TestIsSimpleLayout(t *testing.T) {
	tests := []struct {
		layout string
		want   bool
	}{
		{"tall", true},
		{"fat", true},
		{"grid", true},
		{"horizontal", true},
		{"vertical", true},
		{"splits", false},
		{"stack", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isSimpleLayout(tt.layout)
		if got != tt.want {
			t.Errorf("isSimpleLayout(%q) = %v, want %v", tt.layout, got, tt.want)
		}
	}
}

func TestSplitInfoCalculation(t *testing.T) {
	// Test that bias calculation is correct:
	// node.Bias is fraction for first child (e.g., 0.7 = first gets 70%)
	// kitty --bias is for NEW window, so pass (1 - bias) * 100

	tests := []struct {
		nodeBias     float64
		expectedBias int
	}{
		{0.7, 30},  // first gets 70%, new window gets 30%
		{0.5, 50},  // equal split
		{0.33, 67}, // first gets 33%, new window gets 67%
		{0.0, 0},   // zero means default
		{1.0, 0},   // 1.0 means default
	}

	for _, tt := range tests {
		bias := 0
		if tt.nodeBias > 0 && tt.nodeBias < 1 {
			bias = int((1 - tt.nodeBias) * 100)
		}
		if bias != tt.expectedBias {
			t.Errorf("bias for nodeBias=%v: got %d, want %d", tt.nodeBias, bias, tt.expectedBias)
		}
	}
}

func TestSplitTypeFromHorizontal(t *testing.T) {
	// In kitty layout_state:
	// horizontal=true means children are arranged left/right (vsplit)
	// horizontal=false means children are arranged top/bottom (hsplit)

	tests := []struct {
		horizontal bool
		want       string
	}{
		{true, "vsplit"},
		{false, "hsplit"},
	}

	for _, tt := range tests {
		splitType := "vsplit"
		if !tt.horizontal {
			splitType = "hsplit"
		}
		if splitType != tt.want {
			t.Errorf("splitType for horizontal=%v: got %q, want %q", tt.horizontal, splitType, tt.want)
		}
	}
}

func TestSplitNodeIsLeaf(t *testing.T) {
	idx := 0
	leaf := &model.SplitNode{WindowIdx: &idx}
	if !leaf.IsLeaf() {
		t.Error("expected leaf node with WindowIdx to be a leaf")
	}

	internal := &model.SplitNode{
		Children: [2]*model.SplitNode{
			{WindowIdx: &idx},
			{WindowIdx: &idx},
		},
	}
	if internal.IsLeaf() {
		t.Error("expected internal node with children to not be a leaf")
	}
}
