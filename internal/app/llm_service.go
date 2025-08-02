package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/tui/events"
)

// LLMService handles all LLM-related business logic
type LLMService struct {
	client       llm.Client
	eventBroker  *events.Broker
	
	// Current state
	isStreaming  bool
	streamingMsg string
	streamingTokens int
	streamingStart time.Time
	
	// Debug mode
	debugMode    bool
}

// NewLLMService creates a new LLM service
func NewLLMService(eventBroker *events.Broker) *LLMService {
	return &LLMService{
		eventBroker: eventBroker,
	}
}

// SetClient sets the LLM client
func (s *LLMService) SetClient(client llm.Client) {
	s.client = client
}


// HandleUserMessage processes a user message and streams the response
func (s *LLMService) HandleUserMessage(messages []llm.Message, userMessage string) {
	// Enable debug mode for now since we don't have LLM client
	s.debugMode = true
	
	// If in debug mode, create debug echo response
	if s.debugMode {
		s.handleDebugEcho(userMessage)
		return
	}
	
	// Add user message to history
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: userMessage,
	})
	
	// Publish user message event
	s.eventBroker.Publish(events.Event{
		Type: events.UserMessageEvent,
		Payload: events.MessagePayload{
			Message: llm.Message{
				Role:    "user",
				Content: userMessage,
			},
		},
	})
	
	// Start streaming
	s.eventBroker.Publish(events.Event{
		Type: events.StreamStartEvent,
	})
	
	// Reset streaming state
	s.isStreaming = true
	s.streamingMsg = ""
	s.streamingTokens = 0
	s.streamingStart = time.Now()
	
	// Stream from LLM
	go s.streamResponse(messages)
}

// streamResponse handles the actual streaming from LLM
func (s *LLMService) streamResponse(messages []llm.Message) {
	ctx := context.Background()
	
	if s.client == nil {
		s.eventBroker.Publish(events.Event{
			Type: events.ErrorMessageEvent,
			Payload: events.StatusMessagePayload{
				Message: "No LLM client configured",
				Type:    "error",
			},
		})
		s.endStreaming()
		return
	}
	
	err := s.client.Stream(ctx, messages, func(chunk string) {
		s.streamingMsg += chunk
		s.streamingTokens += len(strings.Fields(chunk))
		
		// Send each chunk as an event
		s.eventBroker.Publish(events.Event{
			Type: events.StreamChunkEvent,
			Payload: events.StreamChunkPayload{
				Content:    chunk,
				TokenCount: len(strings.Fields(chunk)),
			},
		})
	})
	
	if err != nil {
		s.eventBroker.Publish(events.Event{
			Type: events.ErrorMessageEvent,
			Payload: events.StatusMessagePayload{
				Message: "LLM Error: " + err.Error(),
				Type:    "error",
			},
		})
	}
	
	// End streaming and convert to message
	s.endStreaming()
}

// endStreaming finalizes the streaming process
func (s *LLMService) endStreaming() {
	if s.streamingMsg != "" {
		// Publish assistant message
		s.eventBroker.Publish(events.Event{
			Type: events.AssistantMessageEvent,
			Payload: events.MessagePayload{
				Message: llm.Message{
					Role:    "assistant",
					Content: s.streamingMsg,
				},
			},
		})
	}
	
	// Reset state
	s.isStreaming = false
	s.streamingMsg = ""
	s.streamingTokens = 0
	
	// End streaming event
	s.eventBroker.Publish(events.Event{
		Type: events.StreamEndEvent,
	})
}

// IsStreaming returns whether the service is currently streaming
func (s *LLMService) IsStreaming() bool {
	return s.isStreaming
}

// handleDebugEcho creates a debug echo response with metadata
func (s *LLMService) handleDebugEcho(userMessage string) {
	// Create timestamp
	timestamp := time.Now()
	
	// Create debug response with metadata
	debugResponse := fmt.Sprintf(`🤖 DEBUG ECHO RESPONSE
━━━━━━━━━━━━━━━━━━━━━
📥 Input: "%s"
📏 Length: %d characters
🔤 Words: %d
⏰ Time: %s
🆔 Message ID: %d
━━━━━━━━━━━━━━━━━━━━━

📋 METADATA:
• Spaces detected: %d
• Has slash command: %v
• Starts with capital: %v
• Contains numbers: %v

🔄 ECHO: %s

🎯 This is a debug echo response showing message metadata!`,
		userMessage,
		len(userMessage),
		len(strings.Fields(userMessage)),
		timestamp.Format("15:04:05.000"),
		timestamp.UnixNano(),
		strings.Count(userMessage, " "),
		strings.HasPrefix(userMessage, "/"),
		len(userMessage) > 0 && userMessage[0] >= 'A' && userMessage[0] <= 'Z',
		strings.ContainsAny(userMessage, "0123456789"),
		userMessage,
	)
	
	// Simulate streaming by sending it in chunks
	go func() {
		// Start streaming
		s.eventBroker.Publish(events.Event{
			Type: events.StreamStartEvent,
		})
		
		s.isStreaming = true
		s.streamingMsg = ""
		
		// Split response into words and stream them
		words := strings.Fields(debugResponse)
		for i, word := range words {
			s.streamingMsg += word
			if i < len(words)-1 {
				s.streamingMsg += " "
			}
			
			// Send chunk event
			s.eventBroker.Publish(events.Event{
				Type: events.StreamChunkEvent,
				Payload: events.StreamChunkPayload{
					Content: word + " ",
				},
			})
			
			// Small delay to simulate streaming
			time.Sleep(20 * time.Millisecond)
		}
		
		// Send final assistant message
		s.eventBroker.Publish(events.Event{
			Type: events.AssistantMessageEvent,
			Payload: events.MessagePayload{
				Message: llm.Message{
					Role:    "assistant",
					Content: s.streamingMsg,
				},
			},
		})
		
		// End streaming
		s.isStreaming = false
		s.eventBroker.Publish(events.Event{
			Type: events.StreamEndEvent,
		})
	}()
}