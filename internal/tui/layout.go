package tui

import (
	tea "github.com/charmbracelet/bubbletea/v2"
)

// resizeComponents resizes all components based on current window size
func (m *Model) resizeComponents() tea.Cmd {
	var cmds []tea.Cmd

	// Calculate layout dimensions
	sidebarWidth := 30
	if m.width < 100 {
		sidebarWidth = 25
	}
	
	statusBarHeight := 1
	inputHeight := 3
	
	// Main content area
	contentWidth := m.width - sidebarWidth
	contentHeight := m.height - statusBarHeight
	
	// Message list gets remaining height
	messageListHeight := contentHeight - inputHeight
	
	// Set component sizes
	cmds = append(cmds, m.sidebar.SetSize(sidebarWidth, contentHeight))
	cmds = append(cmds, m.messageList.SetSize(contentWidth, messageListHeight))
	cmds = append(cmds, m.input.SetSize(contentWidth, inputHeight))
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