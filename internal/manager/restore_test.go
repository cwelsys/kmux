package manager

import (
	"testing"

	"github.com/cwel/kmux/internal/model"
)

func TestRestoreTraversalOrder(t *testing.T) {
	// Tree structure:
	//   root (Horizontal=true, left/right split)
	//   ├── win0 (first child)
	//   └── nested (Horizontal=false, top/bottom split)
	//       ├── win1
	//       └── win2
	//
	// In kitty layout_state: horizontal=true means left/right (vsplit), false means top/bottom (hsplit)
	// Expected creation order: win0 (tab), win1 (vsplit from root), win2 (hsplit from nested)
	idx0, idx1, idx2 := 0, 1, 2
	root := &model.SplitNode{
		Horizontal: true, // left/right = vsplit
		Children: [2]*model.SplitNode{
			{WindowIdx: &idx0},
			{
				Horizontal: false, // top/bottom = hsplit
				Children: [2]*model.SplitNode{
					{WindowIdx: &idx1},
					{WindowIdx: &idx2},
				},
			},
		},
	}

	windows := []model.Window{
		{CWD: "/a"},
		{CWD: "/b"},
		{CWD: "/c"},
	}

	var calls []restoreCall
	traverseForRestore(root, "", windows, func(win model.Window, launchType string) {
		calls = append(calls, restoreCall{cwd: win.CWD, launchType: launchType})
	})

	expected := []restoreCall{
		{cwd: "/a", launchType: "tab"},
		{cwd: "/b", launchType: "vsplit"}, // inherits from root's Horizontal=true
		{cwd: "/c", launchType: "hsplit"}, // uses nested's Horizontal=false
	}

	if len(calls) != len(expected) {
		t.Fatalf("got %d calls, want %d", len(calls), len(expected))
	}

	for i, call := range calls {
		if call != expected[i] {
			t.Errorf("call[%d] = %+v, want %+v", i, call, expected[i])
		}
	}
}

type restoreCall struct {
	cwd        string
	launchType string
}
