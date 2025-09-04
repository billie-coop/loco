package app

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/billie-coop/loco/internal/tools"
	"github.com/billie-coop/loco/internal/tui/events"
)

// UserInputRouter parses user input and routes it to the appropriate handler.
type UserInputRouter struct {
	toolExecutor *ToolExecutor
	toolRegistry *tools.Registry
}

// NewUserInputRouter creates a new input router.
func NewUserInputRouter(toolExecutor *ToolExecutor, toolRegistry *tools.Registry) *UserInputRouter {
	return &UserInputRouter{
		toolExecutor: toolExecutor,
		toolRegistry: toolRegistry,
	}
}

// Route parses and routes user input.
func (r *UserInputRouter) Route(input string) {
	// Trim whitespace
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// Check if it's a command
	if strings.HasPrefix(input, "/") {
		r.routeCommand(input)
	} else {
		// Regular chat message
		r.routeChat(input)
	}
}

// routeCommand parses and routes slash commands using the tool registry.
func (r *UserInputRouter) routeCommand(input string) {
	// Use the registry to parse the command into a ToolCall
	toolCall, err := r.toolRegistry.ParseCommand(input)
	if err != nil {
		// Show error via events
		if r.toolExecutor.eventBroker != nil {
			r.toolExecutor.eventBroker.Publish(events.Event{
				Type: events.StatusMessageEvent,
				Payload: events.StatusMessagePayload{
					Message: fmt.Sprintf("Command error: %v", err),
					Type:    "warning",
				},
			})
		}
		return
	}

	// Execute the parsed tool call
	r.toolExecutor.Execute(*toolCall)
}

// routeChat routes regular chat messages.
func (r *UserInputRouter) routeChat(message string) {
	params, _ := json.Marshal(map[string]string{"message": message})
	r.toolExecutor.Execute(tools.ToolCall{
		Name:  "chat",
		Input: string(params),
	})
}