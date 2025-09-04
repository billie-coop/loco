package tools

import (
	"context"
	"strings"

	"github.com/billie-coop/loco/internal/permission"
)

// HelpTool provides help information about available commands.
type HelpTool struct {
	permissions permission.Service
	registry    *Registry
}

// NewHelpTool creates a new help tool.
func NewHelpTool(permissions permission.Service, registry *Registry) *HelpTool {
	return &HelpTool{
		permissions: permissions,
		registry:    registry,
	}
}

// Name returns the tool name.
func (t *HelpTool) Name() string {
	return "help"
}

// Info returns the tool information.
func (t *HelpTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "help",
		Description: "Show available commands and tools",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Commands: []CommandInfo{
			{
				Command:     "help",
				Aliases:     []string{"h"},
				Description: "Help",
				Examples:    []string{"/help", "/h"},
			},
		},
	}
}

// Run executes the help tool.
func (t *HelpTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var help strings.Builder
	
	help.WriteString("📚 **Available Commands**\n\n")
	
	// Chat Commands
	help.WriteString("**Chat Commands:**\n")
	help.WriteString("• /help - Show this help message\n")
	help.WriteString("• /clear - Clear all messages\n")
	help.WriteString("• /copy [N] - Copy last N messages to clipboard (default: 1)\n\n")
	
	// Analysis
	help.WriteString("**Analysis:**\n")
	help.WriteString("• /analyze [tier] - Analyze project (quick/detailed/deep/full)\n\n")
	
	// Settings
	help.WriteString("**Settings:**\n")
	help.WriteString("• /model - Show current model\n")
	help.WriteString("• /model select - Open model selection dialog\n")
	help.WriteString("• /session - Show current session info\n")
	help.WriteString("• /debug - Toggle debug mode\n\n")
	
	// System
	help.WriteString("**System:**\n")
	help.WriteString("• /quit - Exit Loco\n\n")
	
	// Keyboard Shortcuts
	help.WriteString("**Keyboard Shortcuts:**\n")
	help.WriteString("• Ctrl+L - Clear messages\n")
	help.WriteString("• Ctrl+P - Open command palette\n")
	help.WriteString("• Ctrl+C - Quit\n")
	help.WriteString("• Tab - Trigger completions\n\n")
	
	// Available Tools (if registry is available)
	if t.registry != nil {
		help.WriteString("**Available Tools:**\n")
		tools := t.registry.GetAll()
		for _, tool := range tools {
			info := tool.Info()
			// Skip command tools (they're listed above)
			if info.Name == "help" || info.Name == "clear" || info.Name == "copy" {
				continue
			}
			help.WriteString("• ")
			help.WriteString(info.Name)
			help.WriteString(" - ")
			help.WriteString(info.Description)
			help.WriteString("\n")
		}
	}
	
	return NewTextResponse(help.String()), nil
}