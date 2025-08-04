package permission

import (
	"github.com/billie-coop/loco/internal/state"
	"github.com/billie-coop/loco/internal/tui/events"
	"github.com/google/uuid"
	"sync"
)

// service is the permission service implementation using persistent state.
type service struct {
	store           *state.PermissionStore
	eventBroker     *events.Broker
	allowedTools    []string // Tools that never need permission
	pendingRequests map[string]chan bool
	mu              sync.RWMutex
}

// NewService creates a new permission service.
func NewService(eventBroker *events.Broker, allowedTools []string, statePath string) Service {
	s := &service{
		store:           state.NewPermissionStore(statePath),
		eventBroker:     eventBroker,
		allowedTools:    allowedTools,
		pendingRequests: make(map[string]chan bool),
	}
	
	// Start listening for permission responses
	go s.listenForResponses()
	
	return s
}

// Request checks for permission, using stored grants or asking the user.
func (s *service) Request(req CreatePermissionRequest) bool {
	// Check if tool is in allowed list (like "help", "clear", etc.)
	for _, allowed := range s.allowedTools {
		if allowed == req.ToolName {
			return true
		}
	}
	
	// Check if we already have permission stored
	if s.store.IsGranted(req.ToolName, req.Path) {
		return true // Previously granted with "Always allow"
	}
	
	// Need to ask the user
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

// GrantPermission handles a grant from the UI.
func (s *service) GrantPermission(requestID string, forSession bool, req CreatePermissionRequest) {
	// Save to persistent store if "Always allow"
	if forSession {
		s.store.Grant(req.ToolName, req.Path, true)
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

// DenyPermission handles a denial from the UI.
func (s *service) DenyPermission(requestID string, alwaysDeny bool, req CreatePermissionRequest) {
	// Save to persistent store if "Always deny"
	if alwaysDeny {
		s.store.Deny(req.ToolName, req.Path, true)
	}
	
	// Send response to waiting request
	s.mu.RLock()
	if respCh, ok := s.pendingRequests[requestID]; ok {
		s.mu.RUnlock()
		respCh <- false
	} else {
		s.mu.RUnlock()
	}
}

// RequestAsync is for compatibility - just wraps Request.
func (s *service) RequestAsync(req CreatePermissionRequest) <-chan bool {
	ch := make(chan bool, 1)
	go func() {
		result := s.Request(req)
		ch <- result
		close(ch)
	}()
	return ch
}

// SetHandler is not used in simple service (we use events).
func (s *service) SetHandler(handler RequestHandler) {
	// No-op - we use event-based approach
}

// listenForResponses listens for permission response events from UI.
func (s *service) listenForResponses() {
	eventSub := s.eventBroker.Subscribe()
	
	for event := range eventSub {
		switch event.Type {
		case "tool.approved":
			// Handle approval
			if data, ok := event.Payload.(map[string]interface{}); ok {
				if requestID, ok := data["requestID"].(string); ok {
					if forSession, ok := data["forSession"].(bool); ok {
						if request, ok := data["request"].(CreatePermissionRequest); ok {
							s.GrantPermission(requestID, forSession, request)
						}
					}
				}
			}
			
		case "tool.denied":
			// Handle denial
			if data, ok := event.Payload.(map[string]interface{}); ok {
				if requestID, ok := data["requestID"].(string); ok {
					alwaysDeny := false
					if ad, ok := data["alwaysDeny"].(bool); ok {
						alwaysDeny = ad
					}
					if request, ok := data["request"].(CreatePermissionRequest); ok {
						s.DenyPermission(requestID, alwaysDeny, request)
					}
				}
			}
		}
	}
}