package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/permission"
)

// BaseTool is the interface that all tools must implement.
// Based on Crush's tool architecture but adapted for Loco.
type BaseTool interface {
	// Name returns the tool name used in function calls
	Name() string

	// Info returns OpenAI-compatible tool information
	Info() ToolInfo

	// Run executes the tool with given parameters
	Run(ctx context.Context, call ToolCall) (ToolResponse, error)
}

// CommandInfo represents a slash command declaration for a tool.
type CommandInfo struct {
	Command     string   `json:"command"`     // The slash command name (e.g., "help", "analyze")
	Aliases     []string `json:"aliases"`     // Alternative command names (e.g., ["h"] for help)
	Description string   `json:"description"` // Description shown in completion popup
	Examples    []string `json:"examples"`    // Usage examples for help text
}

// ToolInfo represents OpenAI-compatible tool information with command declarations.
type ToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	Required    []string       `json:"required"`
	Commands    []CommandInfo  `json:"commands,omitempty"` // Slash command declarations
}

// ToolCall represents a tool invocation request from the LLM.
type ToolCall struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"arguments"` // JSON string of parameters
}

// ToolResponse represents the result of a tool execution.
type ToolResponse struct {
	Content  string         `json:"content"`
	IsError  bool           `json:"is_error"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NewTextResponse creates a successful text response.
func NewTextResponse(content string) ToolResponse {
	return ToolResponse{
		Content: content,
		IsError: false,
	}
}

// NewTextErrorResponse creates an error response.
func NewTextErrorResponse(error string) ToolResponse {
	return ToolResponse{
		Content: error,
		IsError: true,
	}
}

// ProgressPublisher is a function type for reporting progress during tool execution
type ProgressPublisher func(phase string, total, completed int, current string)

// GetProgressPublisher extracts the progress publisher from context
// All tools have access to this as a baseline capability
func GetProgressPublisher(ctx context.Context) ProgressPublisher {
	if pub := ctx.Value("progress_publisher"); pub != nil {
		if p, ok := pub.(func(string, int, int, string)); ok {
			return p
		}
	}
	// Return no-op if no publisher available (shouldn't happen in normal execution)
	return func(string, int, int, string) {}
}

// WithResponseMetadata adds metadata to a response.
func WithResponseMetadata(response ToolResponse, metadata any) ToolResponse {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return response
	}

	var metadataMap map[string]any
	if err := json.Unmarshal(metadataJSON, &metadataMap); err != nil {
		return response
	}

	response.Metadata = metadataMap
	return response
}

// ContextKey is a type for context keys.
type ContextKey string

const (
	// SessionIDKey is the context key for session ID
	SessionIDKey ContextKey = "session_id"
	// MessageIDKey is the context key for message ID
	MessageIDKey ContextKey = "message_id"
	// InitiatorKey is the context key for who initiated the tool call (user/system/agent)
	InitiatorKey ContextKey = "initiator"
)

// GetContextValues extracts session and message IDs from context.
func GetContextValues(ctx context.Context) (sessionID, messageID string) {
	if v := ctx.Value(SessionIDKey); v != nil {
		sessionID = v.(string)
	}
	if v := ctx.Value(MessageIDKey); v != nil {
		messageID = v.(string)
	}
	return
}

// SetContextValues adds session and message IDs to context.
func SetContextValues(ctx context.Context, sessionID, messageID string) context.Context {
	ctx = context.WithValue(ctx, SessionIDKey, sessionID)
	ctx = context.WithValue(ctx, MessageIDKey, messageID)
	return ctx
}

// ConvertToOpenAIFormat converts tool info to OpenAI function format.
func ConvertToOpenAIFormat(tool BaseTool) map[string]any {
	info := tool.Info()
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        info.Name,
			"description": info.Description,
			"parameters": map[string]any{
				"type":       "object",
				"properties": info.Parameters,
				"required":   info.Required,
			},
		},
	}
}

// Registry manages available tools.
type Registry struct {
	tools map[string]BaseTool
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]BaseTool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool BaseTool) error {
	if _, exists := r.tools[tool.Name()]; exists {
		return fmt.Errorf("tool %s already registered", tool.Name())
	}
	r.tools[tool.Name()] = tool
	return nil
}

// Replace swaps in a tool implementation, overwriting any existing tool of the same name.
func (r *Registry) Replace(tool BaseTool) {
	r.tools[tool.Name()] = tool
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (BaseTool, bool) {
	tool, exists := r.tools[name]
	return tool, exists
}

// GetAll returns all registered tools.
func (r *Registry) GetAll() []BaseTool {
	tools := make([]BaseTool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// GetOpenAITools returns all tools in OpenAI format.
func (r *Registry) GetOpenAITools() []map[string]any {
	tools := make([]map[string]any, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, ConvertToOpenAIFormat(tool))
	}
	return tools
}

// GetCommandRegistry returns a map of command names to tool names.
// This enables dynamic command routing based on tool declarations.
func (r *Registry) GetCommandRegistry() map[string]string {
	commands := make(map[string]string)
	
	for _, tool := range r.tools {
		info := tool.Info()
		
		// Register each command declared by the tool
		for _, cmdInfo := range info.Commands {
			// Main command name
			commands[cmdInfo.Command] = info.Name
			
			// Register aliases
			for _, alias := range cmdInfo.Aliases {
				commands[alias] = info.Name
			}
		}
		
		// If tool declares no commands, auto-generate from tool name
		if len(info.Commands) == 0 {
			commands[info.Name] = info.Name
		}
	}
	
	return commands
}

// GetCompletionCommands returns command info for tab completion and auto-suggest.
// This replaces hardcoded command lists in the UI components.
func (r *Registry) GetCompletionCommands() []CompletionCommand {
	var commands []CompletionCommand
	
	for _, tool := range r.tools {
		info := tool.Info()
		
		// Add each declared command
		for _, cmdInfo := range info.Commands {
			commands = append(commands, CompletionCommand{
				Name:        "/" + cmdInfo.Command,
				Description: cmdInfo.Description,
			})
			
			// Add aliases
			for _, alias := range cmdInfo.Aliases {
				commands = append(commands, CompletionCommand{
					Name:        "/" + alias,
					Description: cmdInfo.Description + " (alias)",
				})
			}
		}
		
		// If tool declares no commands, auto-generate from tool name
		if len(info.Commands) == 0 {
			commands = append(commands, CompletionCommand{
				Name:        "/" + info.Name,
				Description: info.Description,
			})
		}
	}
	
	return commands
}

// CompletionCommand represents a command for tab completion UI.
type CompletionCommand struct {
	Name        string `json:"name"`        // The command with slash prefix
	Description string `json:"description"` // Description for the UI
}

// ParseCommand parses a user command string into a ToolCall.
// Supports simple argument parsing based on parameter schemas.
func (r *Registry) ParseCommand(command string) (*ToolCall, error) {
	if !strings.HasPrefix(command, "/") {
		return nil, fmt.Errorf("commands must start with /")
	}
	
	// Remove leading slash
	command = strings.TrimPrefix(command, "/")
	
	// Split into command and arguments
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	
	commandName := parts[0]
	args := parts[1:]
	
	// Look up tool name from command registry
	commandRegistry := r.GetCommandRegistry()
	toolName, exists := commandRegistry[commandName]
	if !exists {
		return nil, fmt.Errorf("unknown command: %s", commandName)
	}
	
	// Get the tool to access its parameter schema
	tool, exists := r.Get(toolName)
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}
	
	// Parse arguments based on parameter schema
	params, err := r.parseArguments(tool.Info(), args)
	if err != nil {
		return nil, fmt.Errorf("error parsing arguments: %v", err)
	}
	
	// Convert to JSON string for ToolCall.Input
	paramJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("error marshaling parameters: %v", err)
	}
	
	return &ToolCall{
		ID:    generateToolCallID(),
		Name:  toolName,
		Input: string(paramJSON),
	}, nil
}

// parseArguments parses command line arguments into a parameter map
// based on the tool's parameter schema.
func (r *Registry) parseArguments(info ToolInfo, args []string) (map[string]any, error) {
	params := make(map[string]any)
	
	// Get parameter properties from schema
	properties, ok := info.Parameters["properties"].(map[string]any)
	if !ok || properties == nil {
		// No parameters expected, but args provided
		if len(args) > 0 {
			return nil, fmt.Errorf("command does not accept arguments")
		}
		return params, nil
	}
	
	// Simple positional argument parsing
	// For now, we'll map arguments to required parameters in order
	requiredParams := info.Required
	
	// Map positional arguments to required parameters
	for i, arg := range args {
		if i >= len(requiredParams) {
			// Extra arguments - ignore for now or could add to an "extra" field
			continue
		}
		
		paramName := requiredParams[i]
		paramDef, exists := properties[paramName].(map[string]any)
		if !exists {
			continue
		}
		
		// Type conversion based on parameter schema
		paramType, _ := paramDef["type"].(string)
		switch paramType {
		case "integer", "number":
			if val, err := strconv.Atoi(arg); err == nil {
				params[paramName] = val
			} else {
				return nil, fmt.Errorf("parameter %s must be a number", paramName)
			}
		case "boolean":
			if val, err := strconv.ParseBool(arg); err == nil {
				params[paramName] = val
			} else {
				return nil, fmt.Errorf("parameter %s must be true or false", paramName)
			}
		case "array":
			// Simple array parsing - split on commas
			params[paramName] = strings.Split(arg, ",")
		default:
			// Default to string
			params[paramName] = arg
		}
	}
	
	// Check required parameters
	for _, required := range info.Required {
		if _, exists := params[required]; !exists {
			return nil, fmt.Errorf("required parameter missing: %s", required)
		}
	}
	
	return params, nil
}

// generateToolCallID creates a unique ID for tool calls
func generateToolCallID() string {
	return fmt.Sprintf("cmd_%d", time.Now().UnixNano())
}

// CreateDefaultRegistry creates a registry with all default tools.
func CreateDefaultRegistry(permissionService permission.Service, workingDir string, analysisService interface{}) *Registry {
	registry := NewRegistry()

	// Register all the core tools
	registry.Register(NewBashTool(permissionService, workingDir))
	registry.Register(NewViewTool(permissionService, workingDir))
	registry.Register(NewEditTool(permissionService, workingDir))
	registry.Register(NewWriteTool(permissionService, workingDir))

	// Register analysis tool if service is provided
	if analysisService != nil {
		// Type assert to analysis.Service when available
		// For now, we'll skip this since we need to set up the service
		// registry.Register(NewAnalyzeTool(permissionService, workingDir, analysisService))
	}

	return registry
}
