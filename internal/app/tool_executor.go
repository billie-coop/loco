package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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

// Execute runs a tool and handles the result (user-initiated).
func (e *ToolExecutor) Execute(call tools.ToolCall) {
	e.executeWithContext(call, "user")
}

// ExecuteSystem runs a tool from system context (system-initiated).
func (e *ToolExecutor) ExecuteSystem(call tools.ToolCall) {
	e.executeWithContext(call, "system")
}

// ExecuteAgent runs a tool from agent context (agent-initiated).
func (e *ToolExecutor) ExecuteAgent(call tools.ToolCall) {
	e.executeWithContext(call, "agent")
}

// executeWithContext runs a tool with the given initiation context.
func (e *ToolExecutor) executeWithContext(call tools.ToolCall, initiator string) {
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

	// Create context with session and message IDs for tools that need them
	ctx := context.Background()
	
	// Add initiator context
	ctx = context.WithValue(ctx, tools.InitiatorKey, initiator)
	
	// Add session ID if available
	if e.sessions != nil {
		if currentSession, err := e.sessions.GetCurrent(); err == nil && currentSession != nil {
			ctx = context.WithValue(ctx, tools.SessionIDKey, currentSession.ID)
			// Generate a message ID (could be more sophisticated)
			ctx = context.WithValue(ctx, tools.MessageIDKey, fmt.Sprintf("msg_%d", time.Now().Unix()))
		}
	}

	// Handle special tools that need async execution
	if call.Name == "analyze" {
		// Handle analysis tool specially - run it asynchronously
		e.handleAnalyzeAsync(call, ctx, initiator)
		return
	}
	
	// Run the tool synchronously for all other tools
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

// handleAnalyzeAsync runs the analyze tool asynchronously
func (e *ToolExecutor) handleAnalyzeAsync(call tools.ToolCall, ctx context.Context, initiator string) {
	// Parse the tier from the input
	var params struct {
		Tier string `json:"tier"`
	}
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		e.eventBroker.Publish(events.Event{
			Type: events.ErrorMessageEvent,
			Payload: events.StatusMessagePayload{
				Message: fmt.Sprintf("Invalid analyze parameters: %v", err),
				Type:    "error",
			},
		})
		return
	}
	
	// Run analysis in background since it can take time
	go func() {
		// Small delay to ensure dialog has closed and UI is ready
		time.Sleep(100 * time.Millisecond)
		
		// Emit analysis started event
		e.eventBroker.Publish(events.Event{
			Type: events.AnalysisStartedEvent,
			Payload: events.AnalysisProgressPayload{
				Phase:       params.Tier,
				TotalFiles:  0,
				CurrentFile: "Starting analysis...",
			},
		})
		
		// Get the tool
		tool, exists := e.registry.Get("analyze")
		if !exists {
			e.eventBroker.Publish(events.Event{
				Type: events.ErrorMessageEvent,
				Payload: events.StatusMessagePayload{
					Message: "Analysis tool not available",
					Type:    "error",
				},
			})
			return
		}
		
		// Run the analysis with the context that has initiator info
		result, err := tool.Run(ctx, call)
		if err != nil {
			e.eventBroker.Publish(events.Event{
				Type: events.AnalysisErrorEvent,
				Payload: events.StatusMessagePayload{
					Message: fmt.Sprintf("Analysis failed: %v", err),
					Type:    "error",
				},
			})
			return
		}
		
		// Show the result
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
		
		// Emit analysis completed event
		e.eventBroker.Publish(events.Event{
			Type: events.AnalysisCompletedEvent,
			Payload: events.AnalysisProgressPayload{
				Phase:       params.Tier,
				CurrentFile: "Analysis complete",
			},
		})
	}()
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