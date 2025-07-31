package chat

import (
	"fmt"
	"time"

	"github.com/billie-coop/loco/internal/llm"
)

// MessageMetadata stores debug info for each message.
type MessageMetadata struct {
	Timestamp   time.Time
	ParseMethod string
	Error       string
	ToolNames   []string
	ToolsFound  int
	Duration    float64
	TokenCount  int
}

// Format returns a formatted string for display.
func (m *MessageMetadata) Format() string {
	if m == nil {
		return ""
	}

	result := "🕐 " + m.Timestamp.Format("15:04:05")

	if m.Duration > 0 {
		result += fmt.Sprintf(" (%.1fs)", m.Duration)
	}

	if m.TokenCount > 0 {
		result += fmt.Sprintf(" • ~%d tokens", m.TokenCount)
	}

	if m.ParseMethod != "" && m.ParseMethod != "no_tools" {
		result += " • Parse: " + m.ParseMethod
	}

	if m.ToolsFound > 0 {
		result += fmt.Sprintf(" • Tools: %v", m.ToolNames)
	}

	if m.Error != "" {
		result += " • ⚠️ " + m.Error
	}

	return result
}

// ExtendedMessage wraps a message with metadata.
type ExtendedMessage struct {
	Metadata *MessageMetadata
	Message  llm.Message
}
