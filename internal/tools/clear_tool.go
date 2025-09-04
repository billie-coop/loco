package tools

import (
	"context"

	"github.com/billie-coop/loco/internal/permission"
)

// ClearTool clears all messages from the current session.
type ClearTool struct {
	permissions permission.Service
}

// NewClearTool creates a new clear tool.
func NewClearTool(permissions permission.Service) *ClearTool {
	return &ClearTool{
		permissions: permissions,
	}
}

// Name returns the tool name.
func (t *ClearTool) Name() string {
	return "clear"
}

// Info returns the tool information.
func (t *ClearTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "clear",
		Description: "Clear all messages from the current session",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Commands: []CommandInfo{
			{
				Command:     "clear",
				Description: "Clear all messages",
				Examples:    []string{"/clear"},
			},
		},
	}
}

// Run executes the clear tool.
func (t *ClearTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	// The actual clearing will be handled by the event system
	// This tool just returns a success response
	// The ToolExecutor will publish the appropriate MessagesClearEvent
	
	return NewTextResponse("✅ Messages cleared"), nil
}