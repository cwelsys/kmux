package protocol

import "encoding/json"

// Method constants
const (
	MethodPing     = "ping"
	MethodSessions = "sessions"
	MethodAttach   = "attach"
	MethodDetach   = "detach"
	MethodKill     = "kill"
	MethodShutdown = "shutdown"
	MethodSplit    = "split"
)

// Request is an RPC request.
type Request struct {
	Method      string          `json:"method"`
	Params      json.RawMessage `json:"params,omitempty"`
	KittySocket string          `json:"kitty_socket,omitempty"` // KITTY_LISTEN_ON value
}

// Response is an RPC response.
type Response struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// SessionInfo is returned by the sessions method.
type SessionInfo struct {
	Name           string `json:"name"`
	Status         string `json:"status"` // "attached", "detached"
	Panes          int    `json:"panes"`
	IsRestorePoint bool   `json:"is_restore_point,omitempty"`
}

// SessionsParams for sessions method.
type SessionsParams struct {
	IncludeRestorePoints bool `json:"include_restore_points,omitempty"`
}

// AttachParams for attach method.
type AttachParams struct {
	Name string `json:"name"`
	CWD  string `json:"cwd,omitempty"`
}

// DetachParams for detach method.
type DetachParams struct {
	Name string `json:"name"`
}

// KillParams for kill method.
type KillParams struct {
	Name string `json:"name"`
}

// AttachResult from attach method.
type AttachResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// SplitParams for split method.
type SplitParams struct {
	Session   string `json:"session"`   // KMUX_SESSION value
	Direction string `json:"direction"` // "vertical" or "horizontal"
	CWD       string `json:"cwd,omitempty"`
}

// SplitResult from split method.
type SplitResult struct {
	Success  bool   `json:"success"`
	WindowID int    `json:"window_id"`
	Message  string `json:"message"`
}

// SuccessResponse creates a success response with the given result.
func SuccessResponse(result any) Response {
	data, _ := json.Marshal(result)
	return Response{Result: data}
}

// ErrorResponse creates an error response.
func ErrorResponse(msg string) Response {
	return Response{Error: msg}
}

// NewRequest creates a request with no params.
func NewRequest(method string, kittySocket string) Request {
	return Request{Method: method, KittySocket: kittySocket}
}

// NewRequestWithParams creates a request with params.
func NewRequestWithParams(method string, kittySocket string, params any) (Request, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return Request{}, err
	}
	return Request{Method: method, Params: data, KittySocket: kittySocket}, nil
}
