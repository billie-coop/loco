package chat

import (
	"context"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/llm"
	tea "github.com/charmbracelet/bubbletea/v2"
)

// Message types for streaming.
type streamDoneMsg struct {
	response string
}

type streamChunkMsg struct {
	chunk string
}

type streamStartMsg struct{}

type errorMsg struct {
	err error
}

type statusMsg struct {
	content string
	isError bool
}

// RequestTeamSelectMsg signals main.go to show team selection.
type RequestTeamSelectMsg struct{}

// Model needs a channel to receive streaming chunks.
var streamChannel chan tea.Msg

func (m *Model) streamResponse() tea.Cmd {
	// Initialize the stream channel
	streamChannel = make(chan tea.Msg, 100)
	// Return a command that starts streaming
	return func() tea.Msg {
		return streamStartMsg{}
	}
}

func (m *Model) doStream() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Check if knowledge base can answer the question first
		if m.knowledgeManager != nil && len(m.messages) > 0 {
			lastMsg := m.messages[len(m.messages)-1]
			if lastMsg.Role == "user" && m.knowledgeManager.HasInfo(lastMsg.Content) {
				// Try to answer from knowledge base using small model
				currentSession, err := m.sessionManager.GetCurrent()
				if err != nil {
					currentSession = nil
				}
				if currentSession != nil && currentSession.Team != nil && currentSession.Team.Small != "" {
					m.showStatus("📚 Checking knowledge base...")

					// Use small model to query knowledge
					answer, err := m.knowledgeManager.QueryKnowledge(lastMsg.Content, currentSession.Team.Small)
					if err == nil && answer != "" {
						// Got an answer from knowledge! Stream it
						go func() {
							defer close(streamChannel)
							// Simulate streaming the knowledge answer
							words := strings.Fields(answer)
							for i, word := range words {
								streamChannel <- streamChunkMsg{chunk: word}
								if i < len(words)-1 {
									streamChannel <- streamChunkMsg{chunk: " "}
								}
							}
							streamChannel <- streamDoneMsg{response: answer}
						}()
						return m.waitForNextChunk()()
					}
				}
			}
		}

		// Fall back to normal LLM streaming
		go func() {
			defer close(streamChannel)

			// Select appropriate model based on context when using team selection
			modelToUse := m.selectModelForMessage()
			if modelToUse != "" {
				// Cast to concrete type to set model
				if lmClient, ok := m.llmClient.(*llm.LMStudioClient); ok {
					lmClient.SetModel(modelToUse)
				}
			}

			err := m.llmClient.Stream(ctx, m.messages, func(chunk string) {
				// Send each chunk as a message
				streamChannel <- streamChunkMsg{chunk: chunk}
			})

			if err != nil {
				streamChannel <- errorMsg{err: err}
			} else {
				streamChannel <- streamDoneMsg{response: m.streamingMsg}
			}
		}()

		// Return first chunk
		return m.waitForNextChunk()()
	}
}

func (m *Model) waitForNextChunk() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-streamChannel
		if !ok {
			return nil
		}
		return msg
	}
}

func (m *Model) sendMessage() tea.Cmd {
	userMsg := m.input.Value()
	m.input.Reset()
	// Add user message with metadata
	msgIndex := len(m.messages)
	m.messages = append(m.messages, llm.Message{
		Role:    "user",
		Content: userMsg,
	})
	m.messagesMeta[msgIndex] = &MessageMetadata{
		Timestamp: time.Now(),
	}
	// Save to session
	if m.sessionManager != nil {
		if err := m.sessionManager.UpdateCurrentMessages(m.messages); err != nil {
			// Log but continue - failed to update messages
			_ = err
		}
	}
	m.isStreaming = true
	m.streamingMsg = ""
	m.streamingTokens = 0
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m.streamResponse()
}
