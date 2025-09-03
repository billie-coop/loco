package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/billie-coop/loco/internal/analysis"
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/permission"
	"github.com/billie-coop/loco/internal/session"
	"github.com/billie-coop/loco/internal/tools"
	"github.com/billie-coop/loco/internal/tui/events"
)

// ToolExecutor handles execution of tools from any source.
type ToolExecutor struct {
	registry          *tools.Registry
	eventBroker       *events.Broker
	sessions          *session.Manager
	llmService        *LLMService
	permissionService permission.Service

	// Active job control (universal interrupt)
	activeMu     sync.Mutex
	activeName   string
	activeCancel context.CancelFunc

	// Model team selection (for display in welcome tool)
	teamClients *llm.TeamClients
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

// SetTeamClients sets the active team clients (used for display)
func (e *ToolExecutor) SetTeamClients(tc *llm.TeamClients) {
	e.teamClients = tc
}

// IsBusy reports whether a tool is currently running (used for scheduling)
func (e *ToolExecutor) IsBusy() bool {
	e.activeMu.Lock()
	defer e.activeMu.Unlock()
	return e.activeCancel != nil
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

// CancelCurrent cancels any in-flight tool execution.
func (e *ToolExecutor) CancelCurrent() {
	e.activeMu.Lock()
	cancel := e.activeCancel
	name := e.activeName
	e.activeCancel = nil
	e.activeName = ""
	e.activeMu.Unlock()

	if cancel != nil {
		cancel()
		// Inform UI
		e.eventBroker.Publish(events.Event{
			Type: events.StatusMessageEvent,
			Payload: events.StatusMessagePayload{
				Message: fmt.Sprintf("⏸️ Interrupted%s", func() string {
					if name != "" {
						return " (" + name + ")"
					}
					return ""
				}()),
				Type: "warning",
			},
		})

		// Also stop streaming if any
		if e.llmService != nil {
			e.llmService.CancelStreaming()
		}
	}
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

	// If welcome tool and we have team clients, attach selected models
	if call.Name == tools.StartupWelcomeToolName && e.teamClients != nil {
		team := &llm.ModelTeam{Name: "Active"}
		if c, ok := e.teamClients.Small.(*llm.LMStudioClient); ok {
			team.Small = c.CurrentModel()
		}
		if c, ok := e.teamClients.Medium.(*llm.LMStudioClient); ok {
			team.Medium = c.CurrentModel()
		}
		if c, ok := e.teamClients.Large.(*llm.LMStudioClient); ok {
			team.Large = c.CurrentModel()
		}
		ctx = context.WithValue(ctx, "model_team", team)
	}

	// If analyze tool, wrap context with progress publisher
	if call.Name == "analyze" {
		ctx = analysis.WithProgressCallback(ctx, func(p analysis.Progress) {
			e.eventBroker.Publish(events.Event{
				Type: events.AnalysisProgressEvent,
				Payload: events.AnalysisProgressPayload{
					Phase:          p.Phase,
					TotalFiles:     p.TotalFiles,
					CompletedFiles: p.CompletedFiles,
					CurrentFile:    p.CurrentFile,
				},
			})
		})
	}

	// Handle special tools that need async execution
	if call.Name == "analyze" {
		// Handle analysis tool specially - run it asynchronously
		e.handleAnalyzeAsync(call, ctx, initiator)
		return
	}

	if call.Name == "startup_scan" {
		// During system startup, run synchronously to avoid conflicts with RAG indexing
		// For user-initiated calls, run async
		if initiator == "system" {
			// Run synchronously during startup - continue to normal sync execution below
		} else {
			// Handle startup scan asynchronously for user calls
			e.handleStartupScanAsync(call, ctx, initiator)
			return
		}
	}

	// For synchronous tools, set up cancelable context and track active job
	ctx, cancel := context.WithCancel(ctx)
	e.setActiveJob(call.Name, cancel)
	defer e.clearActiveJob()

	// Emit tool message showing the tool is running
	e.eventBroker.Publish(events.Event{
		Type: events.SystemMessageEvent,
		Payload: events.MessagePayload{
			Message: llm.Message{
				Role: "tool",
				ToolExecution: &llm.ToolExecution{
					Name:     call.Name,
					Status:   "running",
					Progress: fmt.Sprintf("Running %s...", call.Name),
				},
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
					Role:    "tool",
					Content: fmt.Sprintf("Tool execution failed: %v", err),
					ToolExecution: &llm.ToolExecution{
						Name:   call.Name,
						Status: "error",
					},
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
				Role:    "tool",
				Content: result.Content,
				ToolExecution: &llm.ToolExecution{
					Name:   call.Name,
					Status: "complete",
				},
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
	}
}

func (e *ToolExecutor) setActiveJob(name string, cancel context.CancelFunc) {
	e.activeMu.Lock()
	defer e.activeMu.Unlock()
	e.activeName = name
	e.activeCancel = cancel
}

func (e *ToolExecutor) clearActiveJob() {
	e.activeMu.Lock()
	defer e.activeMu.Unlock()
	e.activeName = ""
	e.activeCancel = nil
}

// handleAnalyzeAsync runs the analyze tool asynchronously
func (e *ToolExecutor) handleAnalyzeAsync(call tools.ToolCall, parentCtx context.Context, initiator string) {
	// Create cancelable context for this async job and track it
	ctx, cancel := context.WithCancel(parentCtx)
	e.setActiveJob("analyze", cancel)

	go func() {
		defer e.clearActiveJob()
		// Small delay to ensure dialog has closed and UI is ready
		time.Sleep(100 * time.Millisecond)

		// Add tool message to chat showing the analysis is starting
		e.eventBroker.Publish(events.Event{
			Type: events.SystemMessageEvent,
			Payload: events.MessagePayload{
				Message: llm.Message{
					Role: "tool",
					ToolExecution: &llm.ToolExecution{
						Name:     "analyze",
						Status:   "pending",
						Progress: fmt.Sprintf("Starting %s analysis...", call.Input),
					},
				},
			},
		})

		// Emit analysis started event
		e.eventBroker.Publish(events.Event{
			Type: events.AnalysisStartedEvent,
			Payload: events.AnalysisProgressPayload{
				Phase:       extractTierFromInput(call.Input),
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
					Role:    "tool",
					Content: result.Content,
					ToolExecution: &llm.ToolExecution{
						Name:     "analyze",
						Status:   "complete",
						Progress: "Analysis complete",
					},
				},
			},
		})

		// Emit analysis completed event
		e.eventBroker.Publish(events.Event{
			Type: events.AnalysisCompletedEvent,
			Payload: events.AnalysisProgressPayload{
				Phase:       extractTierFromInput(call.Input),
				CurrentFile: "Analysis complete",
			},
		})
	}()
}

// handleStartupScanAsync runs the startup scan tool asynchronously
func (e *ToolExecutor) handleStartupScanAsync(call tools.ToolCall, parentCtx context.Context, initiator string) {
	// Create cancelable context for this async job and track it
	ctx, cancel := context.WithCancel(parentCtx)
	e.setActiveJob("startup_scan", cancel)

	go func() {
		defer e.clearActiveJob()
		// Add tool message to chat
		e.eventBroker.Publish(events.Event{
			Type: events.SystemMessageEvent,
			Payload: events.MessagePayload{
				Message: llm.Message{
					Role: "tool",
					ToolExecution: &llm.ToolExecution{
						Name:     "startup_scan",
						Status:   "pending",
						Progress: "Scanning project structure...",
					},
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
					Role:    "tool",
					Content: result.Content,
					ToolExecution: &llm.ToolExecution{
						Name:     "startup_scan",
						Status:   "complete",
						Progress: "Scan complete",
					},
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

// extractTierFromInput extracts tier value from JSON input for display
func extractTierFromInput(input string) string {
	// naive parse; input is small
	if strings.Contains(input, "\"detailed\"") {
		return "detailed"
	}
	if strings.Contains(input, "\"deep\"") {
		return "deep"
	}
	return "quick"
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
