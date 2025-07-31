package chat

import (
	"fmt"
	"time"
	
	"github.com/billie-coop/loco/internal/llm"
)

// MessageMetadata stores debug info for each message
type MessageMetadata struct {
	Timestamp   time.Time
	ParseMethod string   // How we parsed tool calls
	ToolsFound  int      // Number of tools detected
	ToolNames   []string // Which tools were called
	Duration    float64  // Response time in seconds
	TokenCount  int      // Approximate tokens
	Error       string   // Any errors
}

// Format returns a formatted string for display
func (m *MessageMetadata) Format() string {
	if m == nil {
		return ""
	}
	
	result := fmt.Sprintf("🕐 %s", m.Timestamp.Format("15:04:05"))
	
	if m.Duration > 0 {
		result += fmt.Sprintf(" (%.1fs)", m.Duration)
	}
	
	if m.TokenCount > 0 {
		result += fmt.Sprintf(" • ~%d tokens", m.TokenCount)
	}
	
	if m.ParseMethod != "" && m.ParseMethod != "no_tools" {
		result += fmt.Sprintf(" • Parse: %s", m.ParseMethod)
	}
	
	if m.ToolsFound > 0 {
		result += fmt.Sprintf(" • Tools: %v", m.ToolNames)
	}
	
	if m.Error != "" {
		result += fmt.Sprintf(" • ⚠️ %s", m.Error)
	}
	
	return result
}

// ExtendedMessage wraps a message with metadata
type ExtendedMessage struct {
	Message  llm.Message
	Metadata *MessageMetadata
}