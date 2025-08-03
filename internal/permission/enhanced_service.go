package permission

import (
	"sync"

	"github.com/billie-coop/loco/internal/tui/events"
	"github.com/google/uuid"
)

// EnhancedService is an improved permission service with session memory.
type EnhancedService struct {
	eventBroker         *events.Broker
	allowedTools        []string                       // Tools that are always allowed
	sessionPermissions  map[string][]PermissionRecord // Permissions granted for this session
	pendingRequests     map[string]chan bool           // Pending permission requests
	mu                  sync.RWMutex
}

// PermissionRecord tracks a granted permission.
type PermissionRecord struct {
	ToolName string
	Action   string
	Path     string
}

// PermissionRequestEvent is published when permission is needed.
type PermissionRequestEvent struct {
	ID      string
	Request CreatePermissionRequest
}

// PermissionResponseEvent is published when user responds to permission.
type PermissionResponseEvent struct {
	ID           string
	Granted      bool
	ForSession   bool
	Request      CreatePermissionRequest
}

// NewEnhancedService creates a new enhanced permission service.
func NewEnhancedService(eventBroker *events.Broker, allowedTools []string) Service {
	s := &EnhancedService{
		eventBroker:        eventBroker,
		allowedTools:       allowedTools,
		sessionPermissions: make(map[string][]PermissionRecord),
		pendingRequests:    make(map[string]chan bool),
	}

	// Subscribe to permission responses
	go s.listenForResponses()

	return s
}

// Request asks for permission synchronously.
func (s *EnhancedService) Request(req CreatePermissionRequest) bool {
	// Check if tool is in allowed list
	if s.isToolAllowed(req.ToolName) {
		return true
	}

	// Check if we have session permission
	if s.hasSessionPermission(req) {
		return true
	}

	// Generate request ID
	requestID := uuid.New().String()

	// Create response channel
	respCh := make(chan bool, 1)
	s.mu.Lock()
	s.pendingRequests[requestID] = respCh
	s.mu.Unlock()

	// Publish permission request event
	s.eventBroker.PublishAsync(events.Event{
		Type: "permission.request",
		Payload: PermissionRequestEvent{
			ID:      requestID,
			Request: req,
		},
	})

	// Wait for response
	granted := <-respCh

	// Clean up
	s.mu.Lock()
	delete(s.pendingRequests, requestID)
	s.mu.Unlock()

	return granted
}

// RequestAsync asks for permission asynchronously.
func (s *EnhancedService) RequestAsync(req CreatePermissionRequest) <-chan bool {
	ch := make(chan bool, 1)
	go func() {
		result := s.Request(req)
		ch <- result
		close(ch)
	}()
	return ch
}

// SetHandler is not used in enhanced service (we use events instead).
func (s *EnhancedService) SetHandler(handler RequestHandler) {
	// No-op - we use event-based approach
}

// GrantPermission grants a permission (called by UI).
func (s *EnhancedService) GrantPermission(requestID string, forSession bool, req CreatePermissionRequest) {
	// If for session, remember it
	if forSession {
		s.mu.Lock()
		sessionID := req.SessionID
		if sessionID == "" {
			sessionID = "default"
		}
		s.sessionPermissions[sessionID] = append(s.sessionPermissions[sessionID], PermissionRecord{
			ToolName: req.ToolName,
			Action:   req.Action,
			Path:     req.Path,
		})
		s.mu.Unlock()
	}

	// Send response to waiting request
	s.mu.RLock()
	if respCh, ok := s.pendingRequests[requestID]; ok {
		s.mu.RUnlock()
		respCh <- true
	} else {
		s.mu.RUnlock()
	}
}

// DenyPermission denies a permission (called by UI).
func (s *EnhancedService) DenyPermission(requestID string) {
	s.mu.RLock()
	if respCh, ok := s.pendingRequests[requestID]; ok {
		s.mu.RUnlock()
		respCh <- false
	} else {
		s.mu.RUnlock()
	}
}

// isToolAllowed checks if a tool is in the allowed list.
func (s *EnhancedService) isToolAllowed(toolName string) bool {
	for _, allowed := range s.allowedTools {
		if allowed == toolName {
			return true
		}
	}
	return false
}

// hasSessionPermission checks if we have session permission for this request.
func (s *EnhancedService) hasSessionPermission(req CreatePermissionRequest) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = "default"
	}

	permissions, ok := s.sessionPermissions[sessionID]
	if !ok {
		return false
	}

	for _, perm := range permissions {
		if perm.ToolName == req.ToolName && perm.Action == req.Action {
			// If path matches or permission was for parent directory
			if perm.Path == req.Path || perm.Path == "" {
				return true
			}
		}
	}

	return false
}

// listenForResponses listens for permission response events from UI.
func (s *EnhancedService) listenForResponses() {
	// Subscribe to dialog response events
	eventSub := s.eventBroker.Subscribe()
	
	for event := range eventSub {
		switch event.Type {
		case "tool.approved":
			// Handle approval - try struct first, then map
			var requestID string
			
			// Try as ToolExecutionPayload struct
			if payload, ok := event.Payload.(events.ToolExecutionPayload); ok {
				requestID = payload.ID
			} else if payload, ok := event.Payload.(map[string]interface{}); ok {
				requestID, _ = payload["ID"].(string)
			}
			
			if requestID != "" {
				s.mu.RLock()
				if respCh, ok := s.pendingRequests[requestID]; ok {
					s.mu.RUnlock()
					respCh <- true
				} else {
					s.mu.RUnlock()
				}
			}
			
		case "tool.denied":
			// Handle denial - try struct first, then map
			var requestID string
			
			// Try as ToolExecutionPayload struct
			if payload, ok := event.Payload.(events.ToolExecutionPayload); ok {
				requestID = payload.ID
			} else if payload, ok := event.Payload.(map[string]interface{}); ok {
				requestID, _ = payload["ID"].(string)
			}
			
			if requestID != "" {
				s.mu.RLock()
				if respCh, ok := s.pendingRequests[requestID]; ok {
					s.mu.RUnlock()
					respCh <- false
				} else {
					s.mu.RUnlock()
				}
			}
		}
	}
}

// ClearSessionPermissions clears all session permissions.
func (s *EnhancedService) ClearSessionPermissions(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if sessionID == "" {
		sessionID = "default"
	}
	delete(s.sessionPermissions, sessionID)
}

// AddAllowedTool adds a tool to the allowed list.
func (s *EnhancedService) AddAllowedTool(toolName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Check if already in list
	for _, tool := range s.allowedTools {
		if tool == toolName {
			return
		}
	}
	
	s.allowedTools = append(s.allowedTools, toolName)
}

// GetAllowedTools returns the list of allowed tools.
func (s *EnhancedService) GetAllowedTools() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make([]string, len(s.allowedTools))
	copy(result, s.allowedTools)
	return result
}