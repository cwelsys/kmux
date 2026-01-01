package kitty

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
	Pairs interface{} `json:"pairs"` // Complex nested structure
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
	ForegroundProcesses []ForegroundProcess `json:"foreground_processes"`
	Neighbors           map[string][]int    `json:"neighbors"`
}

// ForegroundProcess represents a process running in a window.
type ForegroundProcess struct {
	PID     int      `json:"pid"`
	CWD     string   `json:"cwd"`
	Cmdline []string `json:"cmdline"`
}
