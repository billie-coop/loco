package tui

import (
	"fmt"
	"time"
	
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/permission"
	"github.com/billie-coop/loco/internal/tui/components/chat"
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
		m.sidebar.SetStreamingState(true)
		m.showStatus("Loco is thinking...")

	case events.StreamChunkEvent:
		// Handle streaming chunk
		if payload, ok := event.Payload.(events.StreamChunkPayload); ok {
			m.streamingMessage += payload.Content
			m.messageList.SetStreamingState(true, m.streamingMessage)
		}

	case events.StreamEndEvent:
		// Handle stream end
		// Note: The assistant message is now added via AssistantMessageEvent
		// from the LLM service, so we just clear the streaming state here
		
		// Clear streaming state
		m.isStreaming = false
		m.streamingMessage = ""
		m.messageList.SetStreamingState(false, "")
		m.sidebar.SetStreamingState(false)
		m.syncStateToComponents()
		m.showStatus("Ready")

	case events.ErrorMessageEvent:
		// Handle errors
		if payload, ok := event.Payload.(events.StatusMessagePayload); ok {
			// Clear streaming state on error
			if m.isStreaming {
				m.isStreaming = false
				m.streamingMessage = ""
				m.messageList.SetStreamingState(false, "")
				m.sidebar.SetStreamingState(false)
			}

			// Show error in status
			m.showStatus("❌ " + payload.Message)
			m.sidebar.SetError(fmt.Errorf(payload.Message))

			// Optionally add error as system message
			errorMsg := llm.Message{
				Role:    "system",
				Content: "❌ Error: " + payload.Message,
			}
			m.messages.Append(errorMsg)
			m.syncStateToComponents()
			
			// Save to session
			if m.app.Sessions != nil {
				m.app.Sessions.AddMessage(errorMsg)
			}
		}

	case events.StatusMessageEvent:
		// Handle status messages
		if payload, ok := event.Payload.(events.StatusMessagePayload); ok {
			m.showStatus(payload.Message)
		}

	case events.SystemMessageEvent:
		// Handle system messages
		if payload, ok := event.Payload.(events.MessagePayload); ok {
			m.messages.Append(payload.Message)
			m.syncStateToComponents()
			
			// Save to session
			if m.app.Sessions != nil {
				if err := m.app.Sessions.AddMessage(payload.Message); err != nil {
					m.showStatus("⚠️ Failed to save system message: " + err.Error())
				}
			}
		}
	
	case events.AssistantMessageEvent:
		// Handle assistant messages (separate from streaming)
		if payload, ok := event.Payload.(events.MessagePayload); ok {
			// Clear any streaming state first
			if m.isStreaming {
				m.isStreaming = false
				m.streamingMessage = ""
				m.messageList.SetStreamingState(false, "")
			}
			
			// Add the assistant message
			m.messages.Append(payload.Message)
			m.syncStateToComponents()
			
			// Save to session
			if m.app.Sessions != nil {
				if err := m.app.Sessions.AddMessage(payload.Message); err != nil {
					m.showStatus("⚠️ Failed to save assistant message: " + err.Error())
				}
			}
			
			m.showStatus("Ready")
		}

	case events.AnalysisStartedEvent:
		// Handle analysis started
		if payload, ok := event.Payload.(events.AnalysisProgressPayload); ok {
			// Create or update analysis state
			if m.analysisState == nil {
				m.analysisState = &chat.AnalysisState{}
			}
			
			m.analysisState.IsRunning = true
			m.analysisState.CurrentPhase = payload.Phase
			m.analysisState.StartTime = time.Now()
			m.analysisState.TotalFiles = payload.TotalFiles
			
			// Set analysis state based on phase
			switch payload.Phase {
			case "detailed":
				m.analysisState.DetailedRunning = true
			case "deep", "full":
				m.analysisState.KnowledgeRunning = true
			}
			
			// IMPORTANT: Start the timer for analysis feedback
			cmd := m.sidebar.SetAnalysisState(m.analysisState)
			m.showStatus("🔍 Analysis in progress...")
			if cmd != nil {
				// This command starts the timer - ensure it's added to commands
				cmds = append(cmds, cmd)
			}
		}

	case events.AnalysisProgressEvent:
		// Handle analysis progress updates
		if payload, ok := event.Payload.(events.AnalysisProgressPayload); ok {
			// Update existing state
			if m.analysisState == nil {
				m.analysisState = &chat.AnalysisState{}
			}
			
			m.analysisState.IsRunning = true
			m.analysisState.CurrentPhase = payload.Phase
			m.analysisState.TotalFiles = payload.TotalFiles
			m.analysisState.CompletedFiles = payload.CompletedFiles
			
			m.sidebar.SetAnalysisState(m.analysisState)
		}

	case events.AnalysisCompletedEvent:
		// Handle analysis completed
		if payload, ok := event.Payload.(events.AnalysisProgressPayload); ok {
			// Update existing state
			if m.analysisState == nil {
				m.analysisState = &chat.AnalysisState{}
			}
			
			m.analysisState.IsRunning = false
			m.analysisState.CurrentPhase = "complete"
			
			// Mark appropriate tier as complete (keep previous completions)
			switch payload.Phase {
			case "quick":
				m.analysisState.QuickCompleted = true
			case "detailed":
				m.analysisState.DetailedCompleted = true
				m.analysisState.DetailedRunning = false
			case "deep", "full":
				m.analysisState.DeepCompleted = true
				m.analysisState.KnowledgeRunning = false
			}
			
			m.sidebar.SetAnalysisState(m.analysisState)
			m.showStatus("✨ Analysis complete!")
		}

	case events.AnalysisErrorEvent:
		// Handle analysis errors
		if payload, ok := event.Payload.(events.StatusMessagePayload); ok {
			// Update existing state to stop running
			if m.analysisState != nil {
				m.analysisState.IsRunning = false
				m.analysisState.DetailedRunning = false
				m.analysisState.KnowledgeRunning = false
				m.sidebar.SetAnalysisState(m.analysisState)
			}
			
			// Show error
			m.showStatus("❌ Analysis failed: " + payload.Message)
		}

	case events.DialogOpenEvent:
		// Handle dialog open requests
		// The dialog manager already handles opening dialogs internally
		// This event is just for notification purposes

	case events.MessagesClearEvent:
		// Handle clear messages event
		m.clearMessages()
		m.showStatus("✅ Messages cleared")
	
	case events.ToolExecutionApprovedEvent, events.ToolExecutionDeniedEvent:
		// These events are handled by the enhanced service's listener
		// No need to handle them here
	
	case "permission.request":
		// Handle permission request from enhanced permission service
		// Try to handle as struct first (direct from service)
		if reqEvent, ok := event.Payload.(permission.PermissionRequestEvent); ok {
			// Set the request in the dialog
			m.dialogManager.SetToolRequest(reqEvent.Request.ToolName, map[string]interface{}{
				"action": reqEvent.Request.Action,
				"path": reqEvent.Request.Path,
				"description": reqEvent.Request.Description,
			}, reqEvent.ID)
			
			// Open the permissions dialog
			cmds = append(cmds, m.dialogManager.OpenDialog(dialog.PermissionsDialogType))
		}
	}

	return m, tea.Batch(cmds...)
}