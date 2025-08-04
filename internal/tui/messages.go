package tui

import (
	"github.com/billie-coop/loco/internal/csync"
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/tui/components/chat"
)

// syncMessagesToComponents syncs the current messages to all components that display them
func (m *Model) syncMessagesToComponents() {
	messages := m.messages.AllAsLLM()
	
	// Sync to message list
	m.messageList.SetMessages(messages)
	
	// Sync metadata
	meta := make(map[int]*chat.MessageMetadata)
	m.messagesMeta.Range(func(k int, v *chat.MessageMetadata) bool {
		meta[k] = v
		return true
	})
	m.messageList.SetMessageMeta(meta)
	
	// Sync to sidebar for message counts
	m.sidebar.SetMessages(messages)
}

// syncStateToComponents syncs all state to components
func (m *Model) syncStateToComponents() {
	m.syncMessagesToComponents()
	
	// Sync session manager to sidebar
	if m.app.Sessions != nil {
		m.sidebar.SetSessionManager(m.app.Sessions)
	}
	
	// Sync streaming state
	m.sidebar.SetStreamingState(m.isStreaming)
	
	// TODO: Sync model info when available from LLM service
}

// clearMessages clears all messages
func (m *Model) clearMessages() {
	m.messages.Clear()
	m.messagesMeta = csync.NewMap[int, *chat.MessageMetadata]()
	m.syncMessagesToComponents()
	
	// Clear session messages too
	if m.app.Sessions != nil {
		if err := m.app.Sessions.ClearMessages(); err != nil {
			m.showStatus("⚠️ Failed to clear session: " + err.Error())
		} else {
			m.showStatus("Messages cleared")
		}
	} else {
		m.showStatus("Messages cleared")
	}
	
	// Add welcome message
	welcomeMsg := llm.Message{
		Role:    "system",
		Content: "💬 Chat cleared. Ready for a new conversation!",
	}
	m.messages.Append(welcomeMsg)
	m.syncMessagesToComponents()
	
	// Save welcome message to session
	if m.app.Sessions != nil {
		m.app.Sessions.AddMessage(welcomeMsg)
	}
}

// addSystemMessage adds a system message
func (m *Model) addSystemMessage(content string) {
	m.messages.Append(llm.Message{
		Role:    "system",
		Content: content,
	})
	m.syncMessagesToComponents()
}

// showStatus shows a status message in the status bar
func (m *Model) showStatus(message string) {
	m.statusBar.ShowInfo(message)
}