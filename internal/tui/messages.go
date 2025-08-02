package tui

import (
	"github.com/billie-coop/loco/internal/csync"
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/tui/components/chat"
)

// syncMessagesToComponents syncs the current messages to all components that display them
func (m *Model) syncMessagesToComponents() {
	messages := m.messages.All()
	
	// Sync to message list
	m.messageList.SetMessages(messages)
	
	// Sync metadata
	meta := make(map[int]*chat.MessageMetadata)
	m.messagesMeta.Range(func(k int, v *chat.MessageMetadata) bool {
		meta[k] = v
		return true
	})
	m.messageList.SetMessageMeta(meta)
	
	// TODO: Sync to sidebar when methods are available
}

// syncStateToComponents syncs all state to components
func (m *Model) syncStateToComponents() {
	m.syncMessagesToComponents()
	
	// TODO: Sync other state when methods are available
}

// clearMessages clears all messages
func (m *Model) clearMessages() {
	m.messages.Clear()
	m.messagesMeta = csync.NewMap[int, *chat.MessageMetadata]()
	m.syncMessagesToComponents()
	m.showStatus("Messages cleared")
	
	// Add welcome message
	m.messages.Append(llm.Message{
		Role:    "system",
		Content: "💬 Chat cleared. Ready for a new conversation!",
	})
	m.syncMessagesToComponents()
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