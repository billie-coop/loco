package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

func (m *Model) renderMessages() string {
	var sb strings.Builder

	// Count non-system messages
	visibleMessages := 0
	for _, msg := range m.messages {
		if msg.Role != "system" {
			visibleMessages++
		}
	}

	// Show welcome message if no conversation yet
	if visibleMessages == 0 && !m.isStreaming {
		welcome := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			Render("Ready to chat. Running locally via LM Studio.")
		sb.WriteString(welcome)
		sb.WriteString("\n\n")

		// Show quick start hint
		hint := lipgloss.NewStyle().
			Foreground(lipgloss.Color("239")).
			Render("Type a message or use /help for commands")
		sb.WriteString(hint)
		sb.WriteString("\n")
		return sb.String()
	}

	// Render each message
	for i, msg := range m.messages {
		// Skip system messages unless debug mode
		if msg.Role == "system" && !m.showDebug {
			continue
		}

		// Style based on role
		var rolePrefix string
		var contentStyle lipgloss.Style

		switch msg.Role {
		case "user":
			rolePrefix = "You:"
			contentStyle = userStyle
		case "assistant":
			rolePrefix = "Loco:"
			contentStyle = assistantStyle
		case "system":
			rolePrefix = "System:"
			contentStyle = systemStyle
		}

		// Add role prefix
		sb.WriteString(rolePrefix)
		sb.WriteString("\n")

		// Render content
		content := msg.Content

		// Apply markdown rendering for assistant messages
		if msg.Role == "assistant" {
			rendered, err := m.renderMarkdown(content)
			if err == nil {
				content = rendered
			}
		}

		// Apply style
		sb.WriteString(contentStyle.Render(content))

		// Add metadata if in debug mode
		if m.showDebug {
			if meta, exists := m.messagesMeta[i]; exists {
				metaInfo := m.formatMetadata(meta)
				sb.WriteString("\n")
				sb.WriteString(metaStyle.Render(metaInfo))
			}
		}

		sb.WriteString("\n")
	}

	// Add streaming message if any
	if m.isStreaming && m.streamingMsg != "" {
		sb.WriteString("Loco:\n")
		sb.WriteString(assistantStyle.Render(m.streamingMsg))
		sb.WriteString(" ")
		sb.WriteString(m.spinner.View())
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m *Model) formatMetadata(meta *MessageMetadata) string {
	parts := []string{
		"🕐 " + meta.Timestamp.Format("15:04:05"),
	}

	if meta.Duration > 0 {
		parts = append(parts, fmt.Sprintf("%.1fs", meta.Duration))
	}

	if meta.TokenCount > 0 {
		parts = append(parts, fmt.Sprintf("~%d tokens", meta.TokenCount))
	}

	if meta.ModelName != "" {
		parts = append(parts, meta.ModelName)
	}

	if meta.ToolsFound > 0 {
		parts = append(parts, fmt.Sprintf("%d tools", meta.ToolsFound))
	}

	return strings.Join(parts, " • ")
}

func (m *Model) renderMarkdown(content string) (string, error) {
	// Create a glamour renderer with a custom style
	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dracula"),
		glamour.WithWordWrap(m.width-4), // Account for padding
		glamour.WithPreservedNewLines(),
		glamour.WithEmoji(),
	)
	if err != nil {
		return content, err
	}

	rendered, err := r.Render(content)
	if err != nil {
		return content, err
	}

	// Remove extra newlines that glamour adds
	rendered = strings.TrimRight(rendered, "\n")

	return rendered, nil
}
