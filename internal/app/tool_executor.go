package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/session"
	"github.com/billie-coop/loco/internal/tools"
	"github.com/billie-coop/loco/internal/tui/events"
)

// ToolExecutor handles execution of tools from any source.
type ToolExecutor struct {
	registry    *tools.Registry
	eventBroker *events.Broker
	sessions    *session.Manager
	llmService  *LLMService
}

// NewToolExecutor creates a new tool executor.
func NewToolExecutor(
	registry *tools.Registry,
	eventBroker *events.Broker,
	sessions *session.Manager,
	llmService *LLMService,
) *ToolExecutor {
	return &ToolExecutor{
		registry:    registry,
		eventBroker: eventBroker,
		sessions:    sessions,
		llmService:  llmService,
	}
}

// Execute runs a tool and handles the result.
func (e *ToolExecutor) Execute(call tools.ToolCall) {
	// Get the tool
	tool, exists := e.registry.Get(call.Name)
	if !exists {
		e.eventBroker.Publish(events.Event{
			Type: events.ErrorMessageEvent,
			Payload: events.StatusMessagePayload{
				Message: fmt.Sprintf("Unknown tool: %s", call.Name),
				Type:    "error",
			},
		})
		return
	}

	// Create context (could add session/message IDs here)
	ctx := context.Background()

	// Run the tool
	result, err := tool.Run(ctx, call)
	if err != nil {
		e.eventBroker.Publish(events.Event{
			Type: events.ErrorMessageEvent,
			Payload: events.StatusMessagePayload{
				Message: fmt.Sprintf("Tool execution failed: %v", err),
				Type:    "error",
			},
		})
		return
	}

	// Handle special tools with side effects
	switch call.Name {
	case "clear":
		// Publish clear event
		e.eventBroker.Publish(events.Event{
			Type: events.MessagesClearEvent,
		})
		
	case "chat":
		// Parse chat params to get the message
		var params struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(call.Input), &params); err == nil && params.Message != "" {
			// Create user message
			userMsg := llm.Message{
				Role:    "user",
				Content: params.Message,
			}
			
			// Publish user message event
			e.eventBroker.Publish(events.Event{
				Type: events.UserMessageEvent,
				Payload: events.MessagePayload{
					Message: userMsg,
				},
			})
			
			// Send to LLM if available
			if e.llmService != nil && e.sessions != nil {
				go func() {
					messages, _ := e.sessions.GetMessages()
					messages = append(messages, userMsg)
					e.llmService.HandleUserMessage(messages, params.Message)
				}()
			}
		}
		
	case "help":
		// Show help as a system message
		if result.Content != "" {
			e.eventBroker.Publish(events.Event{
				Type: events.SystemMessageEvent,
				Payload: events.MessagePayload{
					Message: llm.Message{
						Role:    "system",
						Content: result.Content,
					},
				},
			})
		}
		
	case "copy":
		// Just show the status message
		if result.Content != "" {
			e.eventBroker.Publish(events.Event{
				Type: events.StatusMessageEvent,
				Payload: events.StatusMessagePayload{
					Message: result.Content,
					Type:    "success",
				},
			})
		}
		
	default:
		// For other tools, show result as system message if not empty
		if result.Content != "" {
			// Check if it's an error
			if result.IsError {
				e.eventBroker.Publish(events.Event{
					Type: events.ErrorMessageEvent,
					Payload: events.StatusMessagePayload{
						Message: result.Content,
						Type:    "error",
					},
				})
			} else {
				e.eventBroker.Publish(events.Event{
					Type: events.SystemMessageEvent,
					Payload: events.MessagePayload{
						Message: llm.Message{
							Role:    "system",
							Content: result.Content,
						},
					},
				})
			}
		}
	}
}

// ExecuteFromAgent handles tool calls from LLM agents.
func (e *ToolExecutor) ExecuteFromAgent(call tools.ToolCall) tools.ToolResponse {
	// Get the tool
	tool, exists := e.registry.Get(call.Name)
	if !exists {
		return tools.NewTextErrorResponse(fmt.Sprintf("Unknown tool: %s", call.Name))
	}

	// Create context
	ctx := context.Background()

	// Run the tool
	result, err := tool.Run(ctx, call)
	if err != nil {
		return tools.NewTextErrorResponse(fmt.Sprintf("Tool execution failed: %v", err))
	}

	// For agent calls, we return the result directly
	// The agent will decide what to do with it
	return result
}