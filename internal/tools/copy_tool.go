package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/permission"
	"github.com/billie-coop/loco/internal/session"
)

// CopyTool copies recent messages to the clipboard.
type CopyTool struct {
	permissions permission.Service
	sessions    *session.Manager
}

// CopyParams represents the parameters for the copy tool.
type CopyParams struct {
	Count int `json:"count,omitempty"` // Number of messages to copy (default: 1)
}

// NewCopyTool creates a new copy tool.
func NewCopyTool(permissions permission.Service, sessions *session.Manager) *CopyTool {
	return &CopyTool{
		permissions: permissions,
		sessions:    sessions,
	}
}

// Name returns the tool name.
func (t *CopyTool) Name() string {
	return "copy"
}

// Info returns the tool information.
func (t *CopyTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "copy",
		Description: "Copy the last N messages to the clipboard",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"count": map[string]any{
					"type":        "integer",
					"description": "Number of messages to copy (default: 1)",
					"minimum":     1,
				},
			},
		},
		Commands: []CommandInfo{
			{
				Command:     "copy",
				Description: "Copy messages",
				Examples:    []string{"/copy 3", "/copy 1"},
			},
		},
	}
}

// Run executes the copy tool.
func (t *CopyTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	// Parse parameters
	var params CopyParams
	if call.Input != "" {
		if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
			return NewTextErrorResponse(fmt.Sprintf("Failed to parse parameters: %v", err)), nil
		}
	}

	// Default to 1 message if not specified
	if params.Count <= 0 {
		params.Count = 1
	}

	// Get messages from session
	if t.sessions == nil {
		return NewTextErrorResponse("Session manager not available"), nil
	}

	messages, err := t.sessions.GetMessages()
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Failed to get messages: %v", err)), nil
	}

	if len(messages) == 0 {
		return NewTextErrorResponse("No messages to copy"), nil
	}

	// Get the last N messages
	start := len(messages) - params.Count
	if start < 0 {
		start = 0
	}
	messagesToCopy := messages[start:]

	// Format messages
	formatted := formatMessages(messagesToCopy)

	// Copy to clipboard
	if err := copyToClipboard(formatted); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Failed to copy to clipboard: %v", err)), nil
	}

	// Return success message
	messageWord := "message"
	if len(messagesToCopy) > 1 {
		messageWord = "messages"
	}
	return NewTextResponse(fmt.Sprintf("📋 Copied %d %s to clipboard!", len(messagesToCopy), messageWord)), nil
}

// formatMessages formats messages for clipboard.
func formatMessages(messages []llm.Message) string {
	var formatted strings.Builder
	for i, msg := range messages {
		if i > 0 {
			formatted.WriteString("\n\n")
		}

		// Add role prefix
		switch msg.Role {
		case "user":
			formatted.WriteString("👤 User: ")
		case "assistant":
			formatted.WriteString("🤖 Assistant: ")
		case "system":
			formatted.WriteString("🔧 System: ")
		default:
			formatted.WriteString(fmt.Sprintf("%s: ", strings.Title(msg.Role)))
		}

		formatted.WriteString(msg.Content)
	}
	return formatted.String()
}

// copyToClipboard copies text to the system clipboard.
func copyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try xclip first, then xsel
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else {
			return fmt.Errorf("no clipboard command found (install xclip or xsel)")
		}
	case "windows":
		cmd = exec.Command("cmd", "/c", "clip")
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// Set up stdin
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start clipboard command: %w", err)
	}

	// Write the text
	if _, err := stdin.Write([]byte(text)); err != nil {
		return fmt.Errorf("failed to write to clipboard: %w", err)
	}

	// Close stdin
	if err := stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("clipboard command failed: %w", err)
	}

	return nil
}