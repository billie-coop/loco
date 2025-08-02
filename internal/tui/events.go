package tui

import (
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/tui/components/dialog"
	"github.com/billie-coop/loco/internal/tui/events"
	tea "github.com/charmbracelet/bubbletea/v2"
)

// listenForEvents listens for events from the event broker
func (m *Model) listenForEvents() tea.Cmd {
	return func() tea.Msg {
		event := <-m.eventSub
		return event
	}
}

// handleEvent processes events from the event broker
func (m *Model) handleEvent(event events.Event) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch event.Type {
	case events.SessionCreatedEvent:
		// Handle new session creation
		if payload, ok := event.Payload.(events.SessionPayload); ok && payload.Session != nil {
			m.currentSessionID = payload.Session.ID
			// Clear messages for new session
			m.clearMessages()
		}

	case events.UserMessageEvent:
		// Handle user message
		if payload, ok := event.Payload.(events.MessagePayload); ok {
			m.messages.Append(payload.Message)
			m.syncMessagesToComponents()
			m.showStatus("Sending message...")
			
			// Set streaming state
			m.isStreaming = true
			m.streamingMessage = ""
			m.messageList.SetStreamingState(true, "")
		}

	case events.StreamStartEvent:
		// Handle stream start
		m.isStreaming = true
		m.streamingMessage = ""
		m.messageList.SetStreamingState(true, "")
		m.showStatus("Loco is thinking...")

	case events.StreamChunkEvent:
		// Handle streaming chunk
		if payload, ok := event.Payload.(events.StreamChunkPayload); ok {
			m.streamingMessage += payload.Content
			m.messageList.SetStreamingState(true, m.streamingMessage)
		}

	case events.StreamEndEvent:
		// Handle stream end
		if m.streamingMessage != "" {
			m.messages.Append(llm.Message{
				Role:    "assistant",
				Content: m.streamingMessage,
			})
		}

		// Clear streaming state
		m.isStreaming = false
		m.streamingMessage = ""
		m.messageList.SetStreamingState(false, "")
		m.syncMessagesToComponents()
		m.showStatus("Ready")

	case events.ErrorMessageEvent:
		// Handle errors
		if payload, ok := event.Payload.(events.StatusMessagePayload); ok {
			// Clear streaming state on error
			if m.isStreaming {
				m.isStreaming = false
				m.streamingMessage = ""
				m.messageList.SetStreamingState(false, "")
			}

			// Show error in status
			m.showStatus("❌ " + payload.Message)

			// Optionally add error as system message
			m.messages.Append(llm.Message{
				Role:    "system",
				Content: "❌ Error: " + payload.Message,
			})
			m.syncMessagesToComponents()
		}

	case events.StatusMessageEvent:
		// Handle status messages
		if payload, ok := event.Payload.(events.StatusMessagePayload); ok {
			m.showStatus(payload.Message)
		}

	case events.DialogOpenEvent:
		// Handle dialog open requests
		// For now, just open quit dialog as a default
		cmds = append(cmds, m.dialogManager.OpenDialog(dialog.QuitDialogType))
	}

	return m, tea.Batch(cmds...)
}