package tools

import (
	"context"
	"encoding/json"
	"fmt"

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

// ToolInfo represents OpenAI-compatible tool information.
type ToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	Required    []string       `json:"required"`
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