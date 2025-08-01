package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

func (m *Model) renderMessages() string {
	var sb strings.Builder

	// Show welcome message only if there are truly no messages at all
	if len(m.messages) == 0 && !m.isStreaming {
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
		} else {
			// Apply word wrapping for non-assistant messages
			content = m.wrapText(content, m.width-4)
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

// wrapText wraps text at word boundaries to fit within the specified width.
func (m *Model) wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		// If line is already shorter than width, keep it as is
		if len(line) <= width {
			result.WriteString(line)
			continue
		}

		// Wrap long lines at word boundaries
		words := strings.Fields(line)
		currentLine := ""

		for _, word := range words {
			// If word itself is longer than width, it must be broken
			if len(word) > width {
				// Flush current line if any
				if currentLine != "" {
					result.WriteString(currentLine)
					result.WriteString("\n")
				}
				// Break long word
				for len(word) > width {
					result.WriteString(word[:width])
					result.WriteString("\n")
					word = word[width:]
				}
				currentLine = word
			} else if len(currentLine)+1+len(word) > width {
				// Adding this word would exceed width
				result.WriteString(currentLine)
				result.WriteString("\n")
				currentLine = word
			} else {
				// Add word to current line
				if currentLine != "" {
					currentLine += " "
				}
				currentLine += word
			}
		}

		// Write any remaining content
		if currentLine != "" {
			result.WriteString(currentLine)
		}
	}

	return result.String()
}
