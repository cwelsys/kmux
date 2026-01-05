package kitty

import "encoding/json"

// KittyState represents the full output of `kitty @ ls`.
type KittyState []OSWindow

// OSWindow represents a kitty OS window.
type OSWindow struct {
	ID       int    `json:"id"`
	IsActive bool   `json:"is_active"`
	Tabs     []Tab  `json:"tabs"`
}

// Tab represents a kitty tab.
type Tab struct {
	ID          int         `json:"id"`
	IsActive    bool        `json:"is_active"`
	Title       string      `json:"title"`
	Layout      string      `json:"layout"`
	LayoutState LayoutState `json:"layout_state"`
	Windows     []Window    `json:"windows"`
}

// LayoutState contains split information.
type LayoutState struct {
	AllWindows *AllWindows `json:"all_windows,omitempty"`
	Pairs      *Pair       `json:"pairs,omitempty"`
}

// AllWindows contains window group mappings.
type AllWindows struct {
	WindowGroups []WindowGroup `json:"window_groups"`
}

// WindowGroup maps a group ID to window IDs.
// In kitty's layout_state.pairs, leaf nodes are group IDs, not window IDs.
type WindowGroup struct {
	ID        int   `json:"id"`
	WindowIDs []int `json:"window_ids"`
}

// GroupToWindowID builds a map from group ID to first window ID.
// Used to dereference pairs which contain group IDs.
func (a *AllWindows) GroupToWindowID() map[int]int {
	if a == nil {
		return nil
	}
	m := make(map[int]int)
	for _, g := range a.WindowGroups {
		if len(g.WindowIDs) > 0 {
			m[g.ID] = g.WindowIDs[0]
		}
	}
	return m
}

// Pair represents a split node in kitty's layout tree.
// Leaf nodes have GroupID set (an integer in JSON).
// Branch nodes have One/Two set (nested objects in JSON).
// Note: GroupID references layout_state.all_windows.window_groups[].id,
// which must be dereferenced to get actual window IDs.
type Pair struct {
	// Leaf node - this is a GROUP ID, not a window ID!
	// Use LayoutState.AllWindows.GroupToWindowID() to get actual window ID.
	GroupID *int `json:"-"` // populated during unmarshal

	// Branch node
	Horizontal bool    `json:"horizontal,omitempty"` // default true (omitted in JSON)
	Bias       float64 `json:"bias,omitempty"`       // default 0.5 (omitted in JSON)
	One        *Pair   `json:"one,omitempty"`
	Two        *Pair   `json:"two,omitempty"`
}

// UnmarshalJSON handles the polymorphic pairs structure.
// A pair can be either an int (group ID) or an object (split node).
func (p *Pair) UnmarshalJSON(data []byte) error {
	// Try as int first (leaf node - group ID)
	var id int
	if err := json.Unmarshal(data, &id); err == nil {
		p.GroupID = &id
		return nil
	}

	// Parse as raw map first to check which fields are present
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Parse each field
	if v, ok := raw["one"]; ok {
		p.One = &Pair{}
		if err := json.Unmarshal(v, p.One); err != nil {
			return err
		}
	}
	if v, ok := raw["two"]; ok {
		p.Two = &Pair{}
		if err := json.Unmarshal(v, p.Two); err != nil {
			return err
		}
	}
	if v, ok := raw["horizontal"]; ok {
		if err := json.Unmarshal(v, &p.Horizontal); err != nil {
			return err
		}
	} else {
		p.Horizontal = true // default when omitted
	}
	if v, ok := raw["bias"]; ok {
		if err := json.Unmarshal(v, &p.Bias); err != nil {
			return err
		}
	} else {
		p.Bias = 0.5 // default when omitted
	}

	return nil
}

// Window represents a kitty window (pane).
type Window struct {
	ID                  int                 `json:"id"`
	IsActive            bool                `json:"is_active"`
	IsSelf              bool                `json:"is_self"`
	Title               string              `json:"title"`
	CWD                 string              `json:"cwd"`
	PID                 int                 `json:"pid"`
	Cmdline             []string            `json:"cmdline"`
	Env                 map[string]string   `json:"env"`
	UserVars            map[string]string   `json:"user_vars"`
	ForegroundProcesses []ForegroundProcess `json:"foreground_processes"`
	Neighbors           map[string][]int    `json:"neighbors"`
}

// ForegroundProcess represents a process running in a window.
type ForegroundProcess struct {
	PID     int      `json:"pid"`
	CWD     string   `json:"cwd"`
	Cmdline []string `json:"cmdline"`
}
