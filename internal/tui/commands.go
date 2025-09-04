package tui

import (
	"strings"

	"github.com/billie-coop/loco/internal/llm"
	tea "github.com/charmbracelet/bubbletea/v2"
)

// handleSendMessage processes sending a message to the LLM
func (m *Model) handleSendMessage(content string) tea.Cmd {
	// Trim the content
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	// Special case: /debug is UI-specific, handle locally
	if content == "/debug" {
		m.debugMode = !m.debugMode
		m.messageList.SetDebugMode(m.debugMode)
		if m.debugMode {
			m.showStatus("🐛 Debug mode ON")
		} else {
			m.showStatus("Debug mode OFF")
		}
		return nil
	}

	// Route all input through the unified InputRouter
	if m.app.InputRouter != nil {
		m.app.InputRouter.Route(content)
	} else {
		m.showStatus("⚠️ Input router not available")
	}

	return nil
}

// handleCommand processes slash commands
func (m *Model) handleCommand(input string) tea.Cmd {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	command := strings.ToLower(parts[0])
	args := parts[1:]

	switch command {
	case "/help":
		m.messages.Append(llm.Message{
			Role:    "system",
			Content: m.getHelpText(),
		})
		m.syncMessagesToComponents()
		return nil

	case "/clear":
		m.clearMessages()
		return nil

	case "/model":
		if len(args) > 0 {
			// Set model
			modelName := strings.Join(args, " ")
			m.showStatus("Model switching not yet implemented: " + modelName)
		} else {
			// Show current model
			m.messages.Append(llm.Message{
				Role:    "system",
				Content: "📊 Current model: (default)",
			})
			m.syncMessagesToComponents()
		}
		return nil

	case "/session":
		// Show session info
		m.messages.Append(llm.Message{
			Role:    "system",
			Content: "📁 Session: " + m.currentSessionID,
		})
		m.syncMessagesToComponents()
		return nil

	case "/debug":
		// Toggle debug mode
		m.debugMode = !m.debugMode
		m.messageList.SetDebugMode(m.debugMode)
		if m.debugMode {
			m.showStatus("🐛 Debug mode ON")
		} else {
			m.showStatus("Debug mode OFF")
		}
		return nil

	case "/quit", "/exit":
		return tea.Quit

	default:
		m.showStatus("Unknown command: " + command)
		return nil
	}
}

// getHelpText returns the help text for commands
func (m *Model) getHelpText() string {
	return `🔧 Available Commands:

/help          - Show this help message
/clear         - Clear all messages
/model [name]  - Set or show current model
/session       - Show session info
/debug         - Toggle debug mode
/quit          - Exit Loco

Keyboard Shortcuts:
Ctrl+L         - Clear messages
Ctrl+P         - Open command palette
Ctrl+C         - Quit
Tab            - Trigger completions`
}