package chat

import (
	"time"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/tools"
)

// MessageType identifies the type of chat message
type MessageType string

const (
	UserMessageType      MessageType = "user"
	AssistantMessageType MessageType = "assistant"
	SystemMessageType    MessageType = "system"
	ToolMessageType      MessageType = "tool"
)

// Message is the interface for all chat messages
type Message interface {
	Type() MessageType
	Content() string
	Timestamp() time.Time
	ID() string
}

// BaseMessage contains common fields for all message types
type BaseMessage struct {
	id        string
	content   string
	timestamp time.Time
}

// ID returns the message ID
func (m *BaseMessage) ID() string {
	return m.id
}

// Content returns the message content
func (m *BaseMessage) Content() string {
	return m.content
}

// Timestamp returns when the message was created
func (m *BaseMessage) Timestamp() time.Time {
	return m.timestamp
}

// UserMessage represents a message from the user
type UserMessage struct {
	BaseMessage
}

// Type returns the message type
func (m *UserMessage) Type() MessageType {
	return UserMessageType
}

// AssistantMessage represents a message from the assistant
type AssistantMessage struct {
	BaseMessage
	ToolCalls []llm.ToolCall // Tool calls requested by assistant
}

// Type returns the message type
func (m *AssistantMessage) Type() MessageType {
	return AssistantMessageType
}

// SystemMessage represents a system notification
type SystemMessage struct {
	BaseMessage
}

// Type returns the message type
func (m *SystemMessage) Type() MessageType {
	return SystemMessageType
}

// ToolStatus represents the execution status of a tool
type ToolStatus string

const (
	ToolStatusPending  ToolStatus = "pending"
	ToolStatusRunning  ToolStatus = "running"
	ToolStatusComplete ToolStatus = "complete"
	ToolStatusError    ToolStatus = "error"
)

// ToolMessage represents a tool execution in the chat
type ToolMessage struct {
	BaseMessage
	ToolName   string
	ToolCall   tools.ToolCall
	Status     ToolStatus
	Progress   string
	Result     tools.ToolResponse
	StartTime  time.Time
	EndTime    time.Time
}

// Type returns the message type
func (m *ToolMessage) Type() MessageType {
	return ToolMessageType
}

// UpdateStatus updates the tool execution status
func (m *ToolMessage) UpdateStatus(status ToolStatus) {
	m.Status = status
	if status == ToolStatusComplete || status == ToolStatusError {
		m.EndTime = time.Now()
	}
}

// UpdateProgress updates the progress message
func (m *ToolMessage) UpdateProgress(progress string) {
	m.Progress = progress
}

// UpdateResult updates the tool result
func (m *ToolMessage) UpdateResult(result tools.ToolResponse) {
	m.Result = result
	m.content = result.Content
	if result.IsError {
		m.Status = ToolStatusError
	} else {
		m.Status = ToolStatusComplete
	}
	m.EndTime = time.Now()
}

// Duration returns how long the tool took to execute
func (m *ToolMessage) Duration() time.Duration {
	if m.EndTime.IsZero() {
		return time.Since(m.StartTime)
	}
	return m.EndTime.Sub(m.StartTime)
}

// Converter functions to bridge with existing LLM messages

// FromLLMMessage converts an LLM message to a chat message
func FromLLMMessage(msg llm.Message, id string) Message {
	base := BaseMessage{
		id:        id,
		content:   msg.Content,
		timestamp: time.Now(),
	}
	
	switch msg.Role {
	case "user":
		return &UserMessage{BaseMessage: base}
	case "assistant":
		return &AssistantMessage{
			BaseMessage: base,
			ToolCalls:   msg.ToolCalls,
		}
	case "system":
		return &SystemMessage{BaseMessage: base}
	case "tool":
		// For tool messages from LLM
		toolMsg := &ToolMessage{
			BaseMessage: base,
			StartTime:   time.Now(),
		}
		// Extract tool execution details if present
		if msg.ToolExecution != nil {
			toolMsg.ToolName = msg.ToolExecution.Name
			toolMsg.Status = ToolStatus(msg.ToolExecution.Status)
			toolMsg.Progress = msg.ToolExecution.Progress
		}
		return toolMsg
	default:
		return &SystemMessage{BaseMessage: base}
	}
}

// ToLLMMessage converts a chat message to an LLM message for API calls
func ToLLMMessage(msg Message) llm.Message {
	llmMsg := llm.Message{
		Content: msg.Content(),
	}
	
	switch m := msg.(type) {
	case *UserMessage:
		llmMsg.Role = "user"
	case *AssistantMessage:
		llmMsg.Role = "assistant"
		llmMsg.ToolCalls = m.ToolCalls
	case *SystemMessage:
		llmMsg.Role = "system"
	case *ToolMessage:
		// Tool messages don't usually go to LLM, but if needed:
		llmMsg.Role = "tool"
		llmMsg.ToolExecution = &llm.ToolExecution{
			Name:     m.ToolName,
			Status:   string(m.Status),
			Progress: m.Progress,
		}
	default:
		llmMsg.Role = "system"
	}
	
	return llmMsg
}