package chat

import (
	"strings"

	"github.com/billie-coop/loco/internal/llm"
)

// slashCommands defines all available slash commands.
var slashCommands = []struct {
	command     string
	description string
}{
	{"/analyze", "Re-analyze project with deep file reading"},
	{"/confirm-write", "Confirm pending file write operation"},
	{"/debug", "Toggle debug metadata visibility"},
	{"/list", "List all chat sessions"},
	{"/new", "Start a new chat session"},
	{"/switch", "Switch to session number N"},
	{"/team", "Change your model team (S/M/L)"},
	{"/knowledge", "View knowledge base files"},
	{"/project", "Show project context"},
	{"/reset", "Move all sessions to trash and start fresh"},
	{"/screenshot", "Capture UI state to file (also: Ctrl+S)"},
	{"/help", "Show help message"},
}

// getCompletions returns matching slash commands based on input.
func (m *Model) getCompletions(input string) []string {
	if !strings.HasPrefix(input, "/") {
		return nil
	}

	var completions []string
	lowerInput := strings.ToLower(input)

	for _, cmd := range slashCommands {
		if strings.HasPrefix(cmd.command, lowerInput) {
			completions = append(completions, cmd.command)
		}
	}

	return completions
}

// handleTabCompletion handles tab key for slash command completion.
func (m *Model) handleTabCompletion() {
	input := m.input.Value()
	completions := m.getCompletions(input)

	if len(completions) == 0 {
		return
	}

	if len(completions) == 1 {
		// Single match - complete it
		m.input.SetValue(completions[0] + " ")
		m.input.CursorEnd()
	} else {
		// Multiple matches - find common prefix
		commonPrefix := findCommonPrefix(completions)
		if len(commonPrefix) > len(input) {
			m.input.SetValue(commonPrefix)
			m.input.CursorEnd()
		} else {
			// Show available completions in viewport temporarily
			var msg strings.Builder
			msg.WriteString("Available commands:\n")
			for _, cmd := range completions {
				for _, sc := range slashCommands {
					if sc.command == cmd {
						msg.WriteString(cmd)
						msg.WriteString(" - ")
						msg.WriteString(sc.description)
						msg.WriteString("\n")
						break
					}
				}
			}

			// Add as temporary system message
			m.messages = append(m.messages, llm.Message{
				Role:    "system",
				Content: msg.String(),
			})
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()

			// Remove the temporary message after next render
			m.messages = m.messages[:len(m.messages)-1]
		}
	}
}

// findCommonPrefix finds the longest common prefix among strings.
func findCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}

	prefix := strs[0]
	for i := 1; i < len(strs); i++ {
		for !strings.HasPrefix(strs[i], prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}

	return prefix
}
