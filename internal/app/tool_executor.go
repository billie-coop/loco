package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/permission"
	"github.com/billie-coop/loco/internal/session"
	"github.com/billie-coop/loco/internal/tools"
	"github.com/billie-coop/loco/internal/tui/events"
)

// ToolExecutor handles execution of tools from any source.
type ToolExecutor struct {
	registry           *tools.Registry
	eventBroker        *events.Broker
	sessions           *session.Manager
	llmService         *LLMService
	permissionService  permission.Service
}

// NewToolExecutor creates a new tool executor.
func NewToolExecutor(
	registry *tools.Registry,
	eventBroker *events.Broker,
	sessions *session.Manager,
	llmService *LLMService,
	permissionService permission.Service,
) *ToolExecutor {
	return &ToolExecutor{
		registry:          registry,
		eventBroker:       eventBroker,
		sessions:          sessions,
		llmService:        llmService,
		permissionService: permissionService,
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
	
	if call.Name == "startup_scan" {
		// Handle startup scan specially - run it asynchronously
		e.handleStartupScanAsync(call, ctx, initiator)
		return
	}
	
	// Emit tool message showing the tool is running
	e.eventBroker.Publish(events.Event{
		Type: events.SystemMessageEvent,
		Payload: events.MessagePayload{
			Message: llm.Message{
				Role:       "tool",
				ToolName:   call.Name,
				ToolStatus: "running",
				ToolProgress: fmt.Sprintf("Running %s...", call.Name),
			},
		},
	})
	
	// Run the tool synchronously for all other tools
	result, err := tool.Run(ctx, call)
	if err != nil {
		// Update tool message to show error
		e.eventBroker.Publish(events.Event{
			Type: events.SystemMessageEvent,
			Payload: events.MessagePayload{
				Message: llm.Message{
					Role:       "tool",
					ToolName:   call.Name,
					ToolStatus: "error",
					Content:    fmt.Sprintf("Tool execution failed: %v", err),
				},
			},
		})
		return
	}
	
	// Update tool message to show completion
	e.eventBroker.Publish(events.Event{
		Type: events.SystemMessageEvent,
		Payload: events.MessagePayload{
			Message: llm.Message{
				Role:       "tool",
				ToolName:   call.Name,
				ToolStatus: "complete",
				Content:    result.Content,
			},
		},
	})

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
	// Parse the parameters from the input
	var params struct {
		Tier       string `json:"tier"`
		Continue   bool   `json:"continue"`
		ContinueTo string `json:"continue_to"`
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
		
		// Add tool message to chat showing the analysis is starting
		e.eventBroker.Publish(events.Event{
			Type: events.SystemMessageEvent,
			Payload: events.MessagePayload{
				Message: llm.Message{
					Role:       "tool",
					ToolName:   "analyze",
					ToolStatus: "pending",
					ToolProgress: fmt.Sprintf("Starting %s analysis...", params.Tier),
				},
			},
		})
		
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
		
		// Update tool message to show completion
		e.eventBroker.Publish(events.Event{
			Type: events.SystemMessageEvent,
			Payload: events.MessagePayload{
				Message: llm.Message{
					Role:       "tool",
					ToolName:   "analyze",
					ToolStatus: "complete",
					ToolProgress: fmt.Sprintf("%s analysis complete", params.Tier),
					Content:    result.Content,
				},
			},
		})
		
		// Emit analysis completed event
		e.eventBroker.Publish(events.Event{
			Type: events.AnalysisCompletedEvent,
			Payload: events.AnalysisProgressPayload{
				Phase:       params.Tier,
				CurrentFile: "Analysis complete",
			},
		})
		
		// Handle cascading to next tier if requested
		if params.Continue || params.ContinueTo != "" {
			e.handleAnalysisCascade(params.Tier, params.ContinueTo, ctx, initiator)
		}
	}()
}

// handleStartupScanAsync runs the startup scan tool asynchronously
func (e *ToolExecutor) handleStartupScanAsync(call tools.ToolCall, ctx context.Context, initiator string) {
	go func() {
		// Add tool message to chat
		e.eventBroker.Publish(events.Event{
			Type: events.SystemMessageEvent,
			Payload: events.MessagePayload{
				Message: llm.Message{
					Role:       "tool",
					ToolName:   "startup_scan",
					ToolStatus: "pending",
					ToolProgress: "Scanning project structure...",
				},
			},
		})
		
		// Emit startup scan started event
		e.eventBroker.Publish(events.Event{
			Type: events.StartupScanStartedEvent,
			Payload: events.AnalysisProgressPayload{
				Phase:       "startup",
				CurrentFile: "Scanning project structure...",
			},
		})
		
		// Get the tool
		tool, exists := e.registry.Get("startup_scan")
		if !exists {
			e.eventBroker.Publish(events.Event{
				Type: events.ErrorMessageEvent,
				Payload: events.StatusMessagePayload{
					Message: "Startup scan tool not available",
					Type:    "error",
				},
			})
			return
		}
		
		// Run the startup scan
		result, err := tool.Run(ctx, call)
		if err != nil {
			e.eventBroker.Publish(events.Event{
				Type: events.ErrorMessageEvent,
				Payload: events.StatusMessagePayload{
					Message: fmt.Sprintf("Startup scan failed: %v", err),
					Type:    "error",
				},
			})
			return
		}
		
		// Update tool message to show completion
		e.eventBroker.Publish(events.Event{
			Type: events.SystemMessageEvent,
			Payload: events.MessagePayload{
				Message: llm.Message{
					Role:       "tool",
					ToolName:   "startup_scan",
					ToolStatus: "complete",
					ToolProgress: "Scan complete",
					Content:    result.Content,
				},
			},
		})
		
		// Emit startup scan completed event
		e.eventBroker.Publish(events.Event{
			Type: events.StartupScanCompletedEvent,
			Payload: events.AnalysisProgressPayload{
				Phase:       "startup",
				CurrentFile: "Scan complete",
			},
		})
	}()
}

// handleAnalysisCascade continues analysis to the next tier
func (e *ToolExecutor) handleAnalysisCascade(currentTier, continueTo string, ctx context.Context, initiator string) {
	// Determine next tier
	nextTier := ""
	tierOrder := []string{"quick", "detailed", "deep", "full"}
	
	for i, tier := range tierOrder {
		if tier == currentTier && i < len(tierOrder)-1 {
			nextTier = tierOrder[i+1]
			break
		}
	}
	
	// If no next tier or we've reached the target, stop
	if nextTier == "" {
		return
	}
	
	// If continueTo is specified, check if we should stop
	if continueTo != "" && currentTier == continueTo {
		return
	}
	
	// Check if we should stop at this tier
	if continueTo != "" {
		// Find if continueTo comes before nextTier
		continueIdx := -1
		nextIdx := -1
		for i, tier := range tierOrder {
			if tier == continueTo {
				continueIdx = i
			}
			if tier == nextTier {
				nextIdx = i
			}
		}
		
		// If continueTo comes before next tier, stop
		if continueIdx != -1 && nextIdx != -1 && continueIdx < nextIdx {
			return
		}
	}
	
	// Wait a bit before continuing
	time.Sleep(2 * time.Second)
	
	// Create tool call for next tier
	nextInput := fmt.Sprintf(`{"tier": "%s"`, nextTier)
	if continueTo != "" && continueTo != nextTier {
		nextInput += fmt.Sprintf(`, "continue_to": "%s"`, continueTo)
	} else if continueTo == "" {
		// If no specific target, keep cascading
		nextInput += `, "continue": true`
	}
	nextInput += "}"
	
	nextCall := tools.ToolCall{
		Name:  "analyze",
		Input: nextInput,
	}
	
	// For cascading, inherit the permission from the initial request
	// This means if user said "Always allow" for the first tier,
	// all subsequent tiers in the cascade are automatically approved
	e.handleAnalyzeAsync(nextCall, ctx, initiator)
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