package app

import (
	"github.com/billie-coop/loco/internal/csync"
	"github.com/billie-coop/loco/internal/tui/events"
)

// PermissionService handles tool execution permissions
type PermissionService struct {
	eventBroker *events.Broker
	
	// Permission rules
	alwaysAllow *csync.Map[string, bool]
	neverAllow  *csync.Map[string, bool]
}

// NewPermissionService creates a new permission service
func NewPermissionService(eventBroker *events.Broker) *PermissionService {
	return &PermissionService{
		eventBroker: eventBroker,
		alwaysAllow: csync.NewMap[string, bool](),
		neverAllow:  csync.NewMap[string, bool](),
	}
}

// RequestPermission asks for permission to execute a tool
func (s *PermissionService) RequestPermission(toolName string, args map[string]interface{}, requestID string) {
	// Check if we have a rule for this tool
	if allowed, exists := s.alwaysAllow.Get(toolName); exists && allowed {
		s.eventBroker.Publish(events.Event{
			Type: events.ToolExecutionApprovedEvent,
			Payload: events.ToolExecutionPayload{
				ToolName: toolName,
				Args:     args,
				ID:       requestID,
			},
		})
		return
	}
	
	if denied, exists := s.neverAllow.Get(toolName); exists && denied {
		s.eventBroker.Publish(events.Event{
			Type: events.ToolExecutionDeniedEvent,
			Payload: events.ToolExecutionPayload{
				ToolName: toolName,
				Args:     args,
				ID:       requestID,
			},
		})
		return
	}
	
	// No rule - request permission via dialog
	s.eventBroker.Publish(events.Event{
		Type: events.ToolExecutionRequestEvent,
		Payload: events.ToolExecutionPayload{
			ToolName: toolName,
			Args:     args,
			ID:       requestID,
		},
	})
}

// HandlePermissionDecision handles the user's permission decision
func (s *PermissionService) HandlePermissionDecision(toolName string, decision string, requestID string) {
	switch decision {
	case "always":
		s.alwaysAllow.Set(toolName, true)
		s.eventBroker.Publish(events.Event{
			Type: events.ToolExecutionApprovedEvent,
			Payload: events.ToolExecutionPayload{
				ToolName: toolName,
				ID:       requestID,
			},
		})
	case "never":
		s.neverAllow.Set(toolName, true)
		s.eventBroker.Publish(events.Event{
			Type: events.ToolExecutionDeniedEvent,
			Payload: events.ToolExecutionPayload{
				ToolName: toolName,
				ID:       requestID,
			},
		})
	case "once":
		s.eventBroker.Publish(events.Event{
			Type: events.ToolExecutionApprovedEvent,
			Payload: events.ToolExecutionPayload{
				ToolName: toolName,
				ID:       requestID,
			},
		})
	case "deny":
		s.eventBroker.Publish(events.Event{
			Type: events.ToolExecutionDeniedEvent,
			Payload: events.ToolExecutionPayload{
				ToolName: toolName,
				ID:       requestID,
			},
		})
	}
}

// ClearRules clears all permission rules
func (s *PermissionService) ClearRules() {
	s.alwaysAllow.Clear()
	s.neverAllow.Clear()
}