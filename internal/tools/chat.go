package tools

import (
	"context"
	"encoding/json"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/session"
)

// LLMService interface for what ChatTool needs
type LLMService interface {
	HandleUserMessage(messages []llm.Message, userInput string)
}

// ChatTool handles regular chat messages with the LLM.
type ChatTool struct {
	llmService LLMService
	sessions   *session.Manager
}

// ChatParams represents the parameters for the chat tool.
type ChatParams struct {
	Message string `json:"message"` // The message to send to the LLM
}

// NewChatTool creates a new chat tool.
func NewChatTool(llmService LLMService, sessions *session.Manager) *ChatTool {
	return &ChatTool{
		llmService: llmService,
		sessions:   sessions,
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
					"description": "The message to send to the AI assistant",
				},
			},
			"required": []string{"message"},
		},
		Required: []string{"message"},
		Commands: []CommandInfo{
			{
				Command:     "chat",
				Description: "Chat with AI (default for non-slash input)",
				Examples:    []string{"Hello, how are you?", "Tell me about Go"},
			},
		},
	}
}

// Run executes the chat tool.
func (t *ChatTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params ChatParams
	if call.Input != "" {
		if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
			return NewTextErrorResponse("Invalid parameters: " + err.Error()), nil
		}
	}

	if params.Message == "" {
		return NewTextErrorResponse("Message cannot be empty"), nil
	}

	// Get current messages from session
	var messages []llm.Message
	if t.sessions != nil {
		if sessionMessages, err := t.sessions.GetMessages(); err == nil {
			messages = sessionMessages
		}
	}

	// Add user message
	userMsg := llm.Message{
		Role:    "user",
		Content: params.Message,
	}
	messages = append(messages, userMsg)

	// Save user message to session
	if t.sessions != nil {
		if err := t.sessions.AddMessage(userMsg); err != nil {
			// Log error but don't fail
			_ = err
		}
	}

	// Send to LLM service
	if t.llmService != nil {
		// This will handle streaming responses and events asynchronously
		go func() {
			t.llmService.HandleUserMessage(messages, params.Message)
		}()
	}

	// Return immediate response (the streaming response will come via events)
	return NewTextResponse("Message sent to AI assistant"), nil
}