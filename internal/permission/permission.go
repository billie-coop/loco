package permission

import (
	"context"
	"fmt"
	"sync"
)

// Service manages permission requests for tool execution.
type Service interface {
	// Request asks for permission to execute an action
	Request(req CreatePermissionRequest) bool
	
	// RequestAsync asks for permission asynchronously
	RequestAsync(req CreatePermissionRequest) <-chan bool
	
	// SetHandler sets the permission request handler
	SetHandler(handler RequestHandler)
}

// RequestHandler handles permission requests.
type RequestHandler interface {
	// HandleRequest processes a permission request
	HandleRequest(ctx context.Context, req CreatePermissionRequest) bool
}

// CreatePermissionRequest contains information about a permission request.
type CreatePermissionRequest struct {
	SessionID   string      `json:"session_id"`
	ToolCallID  string      `json:"tool_call_id"`
	Path        string      `json:"path"`
	ToolName    string      `json:"tool_name"`
	Action      string      `json:"action"`
	Description string      `json:"description"`
	Params      interface{} `json:"params"`
}

// ErrorPermissionDenied is returned when permission is denied.
var ErrorPermissionDenied = fmt.Errorf("permission denied")

// service implements the Permission Service.
type service struct {
	handler RequestHandler
	mu      sync.RWMutex
}

// NewService creates a new permission service.
func NewService() Service {
	return &service{}
}

// Request asks for permission synchronously.
func (s *service) Request(req CreatePermissionRequest) bool {
	s.mu.RLock()
	handler := s.handler
	s.mu.RUnlock()
	
	if handler == nil {
		// No handler set, deny by default for safety
		return false
	}
	
	return handler.HandleRequest(context.Background(), req)
}

// RequestAsync asks for permission asynchronously.
func (s *service) RequestAsync(req CreatePermissionRequest) <-chan bool {
	ch := make(chan bool, 1)
	
	go func() {
		result := s.Request(req)
		ch <- result
		close(ch)
	}()
	
	return ch
}

// SetHandler sets the permission request handler.
func (s *service) SetHandler(handler RequestHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
}

// AlwaysAllowHandler is a handler that always allows requests.
// Useful for testing or when permissions are not needed.
type AlwaysAllowHandler struct{}

// HandleRequest always returns true.
func (h AlwaysAllowHandler) HandleRequest(ctx context.Context, req CreatePermissionRequest) bool {
	return true
}

// AlwaysDenyHandler is a handler that always denies requests.
// Useful for testing or high-security scenarios.
type AlwaysDenyHandler struct{}

// HandleRequest always returns false.
func (h AlwaysDenyHandler) HandleRequest(ctx context.Context, req CreatePermissionRequest) bool {
	return false
}

// InteractiveHandler prompts the user for permission.
// This will be enhanced later with a proper dialog UI.
type InteractiveHandler struct {
	// For now, we'll auto-approve. Later this will show a dialog.
	AutoApprove bool
}

// HandleRequest prompts the user for permission.
func (h *InteractiveHandler) HandleRequest(ctx context.Context, req CreatePermissionRequest) bool {
	if h.AutoApprove {
		return true
	}
	
	// TODO: Implement interactive dialog
	// For now, just return true
	return true
}