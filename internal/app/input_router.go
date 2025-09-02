package app

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/billie-coop/loco/internal/tools"
	"github.com/billie-coop/loco/internal/tui/events"
)

// UserInputRouter parses user input and routes it to the appropriate handler.
type UserInputRouter struct {
	toolExecutor *ToolExecutor
}

// NewUserInputRouter creates a new input router.
func NewUserInputRouter(toolExecutor *ToolExecutor) *UserInputRouter {
	return &UserInputRouter{
		toolExecutor: toolExecutor,
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

// routeCommand parses and routes slash commands.
func (r *UserInputRouter) routeCommand(input string) {
	// Split into command and args
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	command := strings.ToLower(parts[0])
	args := parts[1:]

	// Route based on command
	switch command {
	case "/help":
		r.toolExecutor.Execute(tools.ToolCall{
			Name:  "help",
			Input: "{}",
		})

	case "/clear":
		r.toolExecutor.Execute(tools.ToolCall{
			Name:  "clear",
			Input: "{}",
		})

	case "/copy":
		// Parse count argument
		count := 1
		if len(args) > 0 {
			if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
				count = n
			}
		}
		
		params, _ := json.Marshal(map[string]int{"count": count})
		r.toolExecutor.Execute(tools.ToolCall{
			Name:  "copy",
			Input: string(params),
		})

	case "/analyze":
		// Parse tier argument
		tier := "quick"
		if len(args) > 0 {
			tier = strings.ToLower(args[0])
		}
		
		params, _ := json.Marshal(map[string]string{"tier": tier})
		r.toolExecutor.Execute(tools.ToolCall{
			Name:  "analyze",
			Input: string(params),
		})
	
	case "/scan":
		// Startup scan
		params, _ := json.Marshal(map[string]bool{"force": true})
		r.toolExecutor.Execute(tools.ToolCall{
			Name:  "startup_scan",
			Input: string(params),
		})

	case "/model":
		// Handle model commands
		if len(args) > 0 && args[0] == "select" {
			// This would open a dialog - handle via events
			// For now, just show current model
			r.toolExecutor.Execute(tools.ToolCall{
				Name:  "model_info",
				Input: "{}",
			})
		} else {
			// Show current model
			r.toolExecutor.Execute(tools.ToolCall{
				Name:  "model_info",
				Input: "{}",
			})
		}

	case "/session":
		// Show session info
		r.toolExecutor.Execute(tools.ToolCall{
			Name:  "session_info",
			Input: "{}",
		})
	
	case "/rag":
		// RAG semantic search
		if len(args) == 0 {
			// Show help for RAG
			if r.toolExecutor.eventBroker != nil {
				r.toolExecutor.eventBroker.Publish(events.Event{
					Type: events.StatusMessageEvent,
					Payload: events.StatusMessagePayload{
						Message: "Usage: /rag <query>",
						Type:    "info",
					},
				})
			}
			return
		}
		
		query := strings.Join(args, " ")
		params, _ := json.Marshal(map[string]interface{}{
			"query": query,
			"k": 5,
		})
		r.toolExecutor.Execute(tools.ToolCall{
			Name:  "rag_query",
			Input: string(params),
		})
	
	case "/rag-index":
		// Index files for RAG
		params, _ := json.Marshal(map[string]interface{}{})
		r.toolExecutor.Execute(tools.ToolCall{
			Name:  "rag_index",
			Input: string(params),
		})

	case "/debug":
		// Toggle debug mode - this is UI-specific, handle via events
		// For now, we'll need to keep this in TUI
		r.toolExecutor.Execute(tools.ToolCall{
			Name:  "debug_toggle",
			Input: "{}",
		})

	case "/quit", "/exit":
		// Handle quit - this should probably emit an event
		r.toolExecutor.Execute(tools.ToolCall{
			Name:  "quit",
			Input: "{}",
		})

	default:
		// Unknown command - show error directly via events
		// Don't try to execute an "unknown" tool
		if r.toolExecutor.eventBroker != nil {
			r.toolExecutor.eventBroker.Publish(events.Event{
				Type: events.StatusMessageEvent,
				Payload: events.StatusMessagePayload{
					Message: fmt.Sprintf("Unknown command: %s", command),
					Type:    "warning",
				},
			})
		}
	}
}

// routeChat routes regular chat messages.
func (r *UserInputRouter) routeChat(message string) {
	params, _ := json.Marshal(map[string]string{"message": message})
	r.toolExecutor.Execute(tools.ToolCall{
		Name:  "chat",
		Input: string(params),
	})
}