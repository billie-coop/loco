package permission

import "errors"

// Service interface defines what permission services must implement.
// This is what tools and other components depend on.
type Service interface {
	Request(req CreatePermissionRequest) bool
	RequestAsync(req CreatePermissionRequest) <-chan bool
	SetHandler(handler RequestHandler)
}

// CreatePermissionRequest represents a request for permission.
type CreatePermissionRequest struct {
	SessionID   string      `json:"session_id"`
	Path        string      `json:"path"`
	ToolCallID  string      `json:"tool_call_id"`
	ToolName    string      `json:"tool_name"`
	Action      string      `json:"action"`
	Description string      `json:"description"`
	Params      interface{} `json:"params,omitempty"`
}

// PermissionRequestEvent is sent when permission is requested.
type PermissionRequestEvent struct {
	ID      string                  `json:"id"`
	Request CreatePermissionRequest `json:"request"`
}

// RequestHandler is a function that handles permission requests.
type RequestHandler func(req CreatePermissionRequest) bool

// Common errors
var (
	ErrorPermissionDenied = errors.New("permission denied")
)