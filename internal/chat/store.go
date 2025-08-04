package chat

import (
	"fmt"
	"sync"
	"time"
	
	"github.com/billie-coop/loco/internal/llm"
)

// MessageStore manages chat messages
type MessageStore struct {
	messages []Message
	mu       sync.RWMutex
	idCounter int
}

// NewMessageStore creates a new message store
func NewMessageStore() *MessageStore {
	return &MessageStore{
		messages: make([]Message, 0),
	}
}

// Add adds a new message to the store
func (s *MessageStore) Add(msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.messages = append(s.messages, msg)
}

// AddUser adds a user message
func (s *MessageStore) AddUser(content string) *UserMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.idCounter++
	msg := &UserMessage{
		BaseMessage: BaseMessage{
			id:        fmt.Sprintf("msg-%d", s.idCounter),
			content:   content,
			timestamp: s.now(),
		},
	}
	
	s.messages = append(s.messages, msg)
	return msg
}

// AddAssistant adds an assistant message
func (s *MessageStore) AddAssistant(content string) *AssistantMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.idCounter++
	msg := &AssistantMessage{
		BaseMessage: BaseMessage{
			id:        fmt.Sprintf("msg-%d", s.idCounter),
			content:   content,
			timestamp: s.now(),
		},
	}
	
	s.messages = append(s.messages, msg)
	return msg
}

// AddSystem adds a system message
func (s *MessageStore) AddSystem(content string) *SystemMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.idCounter++
	msg := &SystemMessage{
		BaseMessage: BaseMessage{
			id:        fmt.Sprintf("msg-%d", s.idCounter),
			content:   content,
			timestamp: s.now(),
		},
	}
	
	s.messages = append(s.messages, msg)
	return msg
}

// AddTool adds a tool message
func (s *MessageStore) AddTool(toolName string) *ToolMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.idCounter++
	msg := &ToolMessage{
		BaseMessage: BaseMessage{
			id:        fmt.Sprintf("msg-%d", s.idCounter),
			timestamp: s.now(),
		},
		ToolName:  toolName,
		Status:    ToolStatusPending,
		StartTime: s.now(),
	}
	
	s.messages = append(s.messages, msg)
	return msg
}

// Get returns a message by ID
func (s *MessageStore) Get(id string) (Message, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	for _, msg := range s.messages {
		if msg.ID() == id {
			return msg, true
		}
	}
	return nil, false
}

// GetTool returns a tool message by ID (for updating)
func (s *MessageStore) GetTool(id string) (*ToolMessage, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	for _, msg := range s.messages {
		if msg.ID() == id {
			if toolMsg, ok := msg.(*ToolMessage); ok {
				return toolMsg, true
			}
		}
	}
	return nil, false
}

// FindPendingTool finds a pending tool message by name
func (s *MessageStore) FindPendingTool(toolName string) (*ToolMessage, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Search from most recent backwards
	for i := len(s.messages) - 1; i >= 0; i-- {
		if toolMsg, ok := s.messages[i].(*ToolMessage); ok {
			if toolMsg.ToolName == toolName && 
			   (toolMsg.Status == ToolStatusPending || toolMsg.Status == ToolStatusRunning) {
				return toolMsg, true
			}
		}
	}
	return nil, false
}

// All returns all messages
func (s *MessageStore) All() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make([]Message, len(s.messages))
	copy(result, s.messages)
	return result
}

// Clear removes all messages
func (s *MessageStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.messages = make([]Message, 0)
	s.idCounter = 0
}

// Count returns the number of messages
func (s *MessageStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return len(s.messages)
}

// FilterByType returns messages of a specific type
func (s *MessageStore) FilterByType(msgType MessageType) []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []Message
	for _, msg := range s.messages {
		if msg.Type() == msgType {
			result = append(result, msg)
		}
	}
	return result
}

// GetRecent returns the most recent n messages
func (s *MessageStore) GetRecent(n int) []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if n >= len(s.messages) {
		result := make([]Message, len(s.messages))
		copy(result, s.messages)
		return result
	}
	
	start := len(s.messages) - n
	result := make([]Message, n)
	copy(result, s.messages[start:])
	return result
}

// Helper to get current time (mockable for tests)
func (s *MessageStore) now() time.Time {
	return time.Now()
}

// Append adds an llm.Message (for compatibility)
func (s *MessageStore) Append(msg llm.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.idCounter++
	id := fmt.Sprintf("msg-%d", s.idCounter)
	chatMsg := FromLLMMessage(msg, id)
	s.messages = append(s.messages, chatMsg)
}

// Replace replaces all messages with llm.Messages (for compatibility)
func (s *MessageStore) Replace(messages []llm.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.messages = make([]Message, 0, len(messages))
	s.idCounter = 0
	
	for _, msg := range messages {
		s.idCounter++
		id := fmt.Sprintf("msg-%d", s.idCounter)
		s.messages = append(s.messages, FromLLMMessage(msg, id))
	}
}

// AllAsLLM returns all messages as llm.Messages (for compatibility)
func (s *MessageStore) AllAsLLM() []llm.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make([]llm.Message, len(s.messages))
	for i, msg := range s.messages {
		result[i] = ToLLMMessage(msg)
	}
	return result
}