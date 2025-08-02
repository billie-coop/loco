package tui

import (
	tea "github.com/charmbracelet/bubbletea/v2"
)

// resizeComponents resizes all components based on current window size
func (m *Model) resizeComponents() tea.Cmd {
	var cmds []tea.Cmd

	// Calculate layout dimensions
	sidebarWidth := m.calculateSidebarWidth()
	statusBarHeight := 1
	inputHeight := 3
	
	// Main content area (accounting for borders)
	contentWidth := m.width - sidebarWidth
	contentHeight := m.height - statusBarHeight
	
	// Message list gets remaining height
	messageListHeight := contentHeight - inputHeight
	
	// Set component sizes (subtract border size from dimensions)
	// Each bordered component loses 2 chars width and 2 lines height for borders
	cmds = append(cmds, m.sidebar.SetSize(sidebarWidth-2, contentHeight-2))
	cmds = append(cmds, m.messageList.SetSize(contentWidth-2, messageListHeight-2))
	cmds = append(cmds, m.input.SetSize(contentWidth-2, inputHeight-2))
	cmds = append(cmds, m.statusBar.SetSize(m.width, statusBarHeight))
	
	// Update dialog manager
	cmds = append(cmds, m.dialogManager.SetSize(m.width, m.height))
	
	// TODO: Update completions when SetSize is available

	return tea.Batch(cmds...)
}

// calculateSidebarWidth calculates the appropriate sidebar width
func (m *Model) calculateSidebarWidth() int {
	if m.width < 80 {
		return 20
	}
	if m.width < 120 {
		return 25
	}
	return 30
}

// calculateInputHeight calculates the appropriate input height
func (m *Model) calculateInputHeight() int {
	// Could be dynamic based on content
	return 3
}