package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/billie-coop/loco/internal/permission"
)

// ChatTool sends messages to the LLM.
type ChatTool struct {
	permissions permission.Service
}

// ChatParams represents the parameters for the chat tool.
type ChatParams struct {
	Message string `json:"message"`
}

// NewChatTool creates a new chat tool.
func NewChatTool(permissions permission.Service) *ChatTool {
	return &ChatTool{
		permissions: permissions,
	}
}

// Name returns the tool name.
func (t *ChatTool) Name() string {
	return "chat"
}

// Info returns the tool information.
func (t *ChatTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "chat",
		Description: "Send a message to the AI assistant",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{
					"type":        "string",
					"description": "The message to send",
				},
			},
			"required": []string{"message"},
		},
	}
}

// Run executes the chat tool.
func (t *ChatTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	// Parse parameters
	var params ChatParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Failed to parse parameters: %v", err)), nil
	}

	if params.Message == "" {
		return NewTextErrorResponse("Message cannot be empty"), nil
	}

	// The actual message sending will be handled by the ToolExecutor
	// which will create the appropriate UserMessageEvent
	// This tool just validates and returns success
	
	// Return empty response - the actual chat will be handled by events
	return NewTextResponse(""), nil
}