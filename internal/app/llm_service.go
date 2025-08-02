package app

import (
	"context"
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