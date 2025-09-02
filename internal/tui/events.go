package tui

import (
	"fmt"
	"time"

	chatcore "github.com/billie-coop/loco/internal/chat"
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/permission"
	"github.com/billie-coop/loco/internal/tools"
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
			m.sidebar.SetError(fmt.Errorf("%s", payload.Message))

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
		// Handle system and tool messages
		if payload, ok := event.Payload.(events.MessagePayload); ok {
			msg := payload.Message
			if msg.Role == "tool" && msg.ToolExecution != nil {
				// Merge into existing pending/running tool card if available
				toolName := msg.ToolExecution.Name
				status := msg.ToolExecution.Status
				progress := msg.ToolExecution.Progress
				content := msg.Content
				if tm, ok := m.messages.FindPendingTool(toolName); ok && tm != nil {
					// Update status/progress
					if status != "" {
						tm.UpdateStatus(chatcore.ToolStatus(status))
					}
					if progress != "" {
						tm.UpdateProgress(progress)
					}
					// If content provided (usually on complete/error), set result
					if content != "" {
						res := tools.ToolResponse{Content: content, IsError: status == "error"}
						tm.UpdateResult(res)
					}
					m.syncMessagesToComponents()
				} else {
					// No existing pending tool; add as a new message
					m.messages.Append(msg)
					m.syncMessagesToComponents()
				}
			} else {
				// Non-tool system message: append normally
				m.messages.Append(msg)
				m.syncStateToComponents()
				// Save to session
				if m.app.Sessions != nil {
					if err := m.app.Sessions.AddMessage(msg); err != nil {
						m.showStatus("⚠️ Failed to save system message: " + err.Error())
					}
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

	case events.StartupScanStartedEvent:
		// Handle startup scan started
		if _, ok := event.Payload.(events.AnalysisProgressPayload); ok {
			// Create or update analysis state
			if m.analysisState == nil {
				m.analysisState = &chat.AnalysisState{}
			}

			m.analysisState.IsRunning = true
			m.analysisState.CurrentPhase = "startup"
			m.lastProgress = time.Now()
			// Update sidebar
			cmd := m.sidebar.SetAnalysisState(m.analysisState)
			m.showStatus("⚡ Running startup scan...")
			m.updateToolProgress("startup_scan", "running", "Scanning project structure...", "")
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case events.StartupScanCompletedEvent:
		// Handle startup scan completed
		if _, ok := event.Payload.(events.AnalysisProgressPayload); ok {
			// Update existing state
			if m.analysisState == nil {
				m.analysisState = &chat.AnalysisState{}
			}

			m.analysisState.StartupScanCompleted = true
			m.analysisState.IsRunning = false // Unless other analysis is running
			m.analysisState.CurrentPhase = ""
			m.lastProgress = time.Now()
			m.sidebar.SetAnalysisState(m.analysisState)
			m.showStatus("✅ Startup scan complete")
			m.updateToolProgress("startup_scan", "complete", "Scan complete", "")
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
			m.lastProgress = time.Now()

			// Set analysis state based on phase
			switch payload.Phase {
			case "quick":
				m.analysisState.QuickCompleted = false // Mark as running
				m.showStatus("🔍 Quick analysis started…")
				m.updateToolProgress("analyze", "running", "Quick analysis started…", "")
			case "detailed":
				m.analysisState.DetailedRunning = true
				m.showStatus("📊 Reading key files…")
				m.updateToolProgress("analyze", "running", "Reading key files…", "")
			case "deep", "full":
				m.analysisState.KnowledgeRunning = true
				m.showStatus("💎 Deep analysis started…")
				m.updateToolProgress("analyze", "running", "Deep analysis started…", "")
			}

			// IMPORTANT: Start the timer for analysis feedback
			cmd := m.sidebar.SetAnalysisState(m.analysisState)
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
			m.lastProgress = time.Now()
			m.sidebar.SetAnalysisState(m.analysisState)

			// Show richer status
			if payload.Phase == "quick" {
				if payload.CompletedFiles == 0 && payload.TotalFiles > 0 && payload.CurrentFile == "discovered files" {
					m.showStatus(fmt.Sprintf("🔎 Discovered %d files", payload.TotalFiles))
					m.updateToolProgress("analyze", "running", fmt.Sprintf("Discovered %d files", payload.TotalFiles), "")
				} else if payload.TotalFiles > 0 {
					msg := fmt.Sprintf("Summarizing %d/%d: %s", payload.CompletedFiles, payload.TotalFiles, payload.CurrentFile)
					m.showStatus("🔍 " + msg)
					m.updateToolProgress("analyze", "running", msg, "")
				}
			} else if payload.Phase == "detailed" {
				if payload.TotalFiles > 0 {
					msg := fmt.Sprintf("Reading key files %d/%d: %s", payload.CompletedFiles, payload.TotalFiles, payload.CurrentFile)
					m.showStatus("📖 " + msg)
					m.updateToolProgress("analyze", "running", msg, "")
				}
			}
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
			m.lastProgress = time.Now()

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
			m.updateToolProgress("analyze", "complete", "Analysis complete", "")
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
			m.updateToolProgress("analyze", "error", payload.Message, "")
		}

	case events.DialogOpenEvent:
		// The dialog manager handles opening; nothing else to do

	case events.DialogCloseEvent:
		// Apply settings when settings dialog closes with a result
		if payload, ok := event.Payload.(events.DialogPayload); ok {
			if payload.DialogID == string(dialog.SettingsDialogType) {
				settings := m.dialogManager.GetSettings()
				if settings != nil {
					// Update config
					cfg := m.app.Config.Get()
					if cfg != nil {
						cfg.LMStudioURL = settings.APIEndpoint
						cfg.LMStudioContextSize = settings.ContextSize
						cfg.LMStudioNumKeep = settings.NumKeep
						_ = m.app.Config.Save()
					}
					// Update LLM client if LM Studio
					if client, ok := m.app.LLM.(*llm.LMStudioClient); ok {
						client.SetEndpoint(settings.APIEndpoint)
						client.SetContextSize(settings.ContextSize)
						client.SetNumKeep(settings.NumKeep)
					}
					// Also apply to team clients if present
					if m.app.TeamClients != nil {
						if c, ok := m.app.TeamClients.Small.(*llm.LMStudioClient); ok {
							c.SetEndpoint(settings.APIEndpoint)
							c.SetContextSize(settings.ContextSize)
							c.SetNumKeep(settings.NumKeep)
						}
						if c, ok := m.app.TeamClients.Medium.(*llm.LMStudioClient); ok {
							c.SetEndpoint(settings.APIEndpoint)
							c.SetContextSize(settings.ContextSize)
							c.SetNumKeep(settings.NumKeep)
						}
						if c, ok := m.app.TeamClients.Large.(*llm.LMStudioClient); ok {
							c.SetEndpoint(settings.APIEndpoint)
							c.SetContextSize(settings.ContextSize)
							c.SetNumKeep(settings.NumKeep)
						}
					}
					m.showStatus("Settings applied")
				}
			}
		}

	case events.ToolExecutionApprovedEvent, events.ToolExecutionDeniedEvent:
		// These events are handled by the permission service's listener
		// No need to handle them here

	case "permission.request":
		// Handle permission request from permission service
		// Try to handle as struct first (direct from service)
		if reqEvent, ok := event.Payload.(permission.PermissionRequestEvent); ok {
			// Set the request in the dialog
			m.dialogManager.SetToolRequest(reqEvent.Request.ToolName, map[string]interface{}{
				"action":      reqEvent.Request.Action,
				"path":        reqEvent.Request.Path,
				"description": reqEvent.Request.Description,
			}, reqEvent.ID)

			// Open the permissions dialog
			cmds = append(cmds, m.dialogManager.OpenDialog(dialog.PermissionsDialogType))
		}

	case events.ModelSelectedEvent:
		// Apply selected model to client and sidebar
		if payload, ok := event.Payload.(events.ModelSelectedPayload); ok {
			if client, ok := m.app.LLM.(*llm.LMStudioClient); ok {
				client.SetModel(payload.ModelID)
			}
			// Update sidebar display
			m.sidebar.SetModel(payload.ModelID, payload.ModelSize)
			m.showStatus("Model selected: " + payload.ModelID)
		}
	}

	return m, tea.Batch(cmds...)
}

// updateToolProgress finds the last pending/running tool card and updates its status/progress
func (m *Model) updateToolProgress(toolName, status, progress, content string) {
	if m.messages == nil {
		return
	}
	if tm, ok := m.messages.FindPendingTool(toolName); ok && tm != nil {
		if status != "" {
			// Cast string to internal chat ToolStatus
			tm.Status = chatcore.ToolStatus(status)
		}
		tm.Progress = progress
		if content != "" {
			// Not usually used for running; reserved for completion/error
			// We do not set content here to avoid clutter unless provided
		}
		// Refresh message list
		m.syncMessagesToComponents()
	}
}
