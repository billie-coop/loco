package chat

import (
	"fmt"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/tui/components/core"
	"github.com/billie-coop/loco/internal/tui/styles"
	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/glamour/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// MessageMetadata contains metadata about a message
type MessageMetadata struct {
	Timestamp  time.Time
	Duration   float64 // in seconds
	TokenCount int
	ModelName  string
	ToolsFound int
}

// MessageListModel implements the message viewport component
type MessageListModel struct {
	viewport viewport.Model
	spinner  spinner.Model
	width    int
	height   int

	// State
	messages       []llm.Message
	messagesMeta   map[int]*MessageMetadata
	isStreaming    bool
	streamingMsg   string
	showDebug      bool
	
	// Tool rendering
	toolRegistry   *ToolRegistry
}

// Ensure MessageListModel implements required interfaces
var _ core.Component = (*MessageListModel)(nil)
var _ core.Sizeable = (*MessageListModel)(nil)

// NewMessageList creates a new message list component
func NewMessageList() *MessageListModel {
	vp := viewport.New()
	vp.MouseWheelEnabled = true

	s := spinner.New(spinner.WithSpinner(spinner.Dot))

	return &MessageListModel{
		viewport:     vp,
		spinner:      s,
		messagesMeta: make(map[int]*MessageMetadata),
		toolRegistry: NewToolRegistry(),
	}
}

// Init initializes the message list component
func (ml *MessageListModel) Init() tea.Cmd {
	// Set initial welcome content
	ml.refreshContent()
	return ml.spinner.Tick
}

// Update handles messages for the message list
func (ml *MessageListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Update spinner if streaming
	if ml.isStreaming {
		ml.spinner, cmd = ml.spinner.Update(msg)
	}

	// Update viewport
	ml.viewport, cmd = ml.viewport.Update(msg)

	return ml, cmd
}

// SetSize sets the dimensions of the message list
func (ml *MessageListModel) SetSize(width, height int) tea.Cmd {
	ml.width = width
	ml.height = height

	ml.viewport = viewport.New(
		viewport.WithWidth(width),
		viewport.WithHeight(height),
	)
	ml.viewport.MouseWheelEnabled = true

	// Re-render with new size
	ml.refreshContent()

	return nil
}

// View renders the message list
func (ml *MessageListModel) View() string {
	return ml.viewport.View()
}

// SetMessages updates the messages list
func (ml *MessageListModel) SetMessages(messages []llm.Message) {
	ml.messages = messages
	ml.refreshContent()
}

// SetMessageMeta updates the metadata for messages
func (ml *MessageListModel) SetMessageMeta(meta map[int]*MessageMetadata) {
	ml.messagesMeta = meta
	ml.refreshContent()
}

// SetStreamingState updates the streaming state
func (ml *MessageListModel) SetStreamingState(isStreaming bool, streamingMsg string) {
	ml.isStreaming = isStreaming
	ml.streamingMsg = streamingMsg
	ml.refreshContent()
}

// SetDebugMode toggles debug information display
func (ml *MessageListModel) SetDebugMode(showDebug bool) {
	ml.showDebug = showDebug
	ml.refreshContent()
}

// GotoBottom scrolls to the bottom of the viewport
func (ml *MessageListModel) GotoBottom() {
	ml.viewport.GotoBottom()
}

// GotoTop scrolls to the top of the viewport
func (ml *MessageListModel) GotoTop() {
	ml.viewport.GotoTop()
}

// SetContent sets custom content in the viewport (for special views)
func (ml *MessageListModel) SetContent(content string) {
	ml.viewport.SetContent(content)
}

// Private methods

func (ml *MessageListModel) refreshContent() {
	content := ml.renderMessages()
	ml.viewport.SetContent(content)
	// Ensure we scroll to bottom if there are messages
	if len(ml.messages) > 0 {
		ml.viewport.GotoBottom()
	}
}

func (ml *MessageListModel) renderMessages() string {
	var sb strings.Builder

	// Show welcome message only if there are truly no messages at all
	if len(ml.messages) == 0 && !ml.isStreaming {
		theme := styles.CurrentTheme()
		welcome := lipgloss.NewStyle().
			Foreground(theme.FgSubtle).
			Italic(true).
			Render("Ready to chat. Running locally via LM Studio.")
		sb.WriteString(welcome)
		sb.WriteString("\n\n")

		// Show quick start hint
		hint := lipgloss.NewStyle().
			Foreground(theme.FgMuted).
			Render("Type a message or use /help for commands")
		sb.WriteString(hint)
		sb.WriteString("\n")
		return sb.String()
	}

	// Render each message
	for i, msg := range ml.messages {
		// Always show system messages (they contain important info like analysis results)
		// Debug metadata is handled separately below

		// Style based on role
		var rolePrefix string
		var contentStyle lipgloss.Style

		switch msg.Role {
		case "user":
			rolePrefix = "You:"
			contentStyle = getUserStyle()
		case "assistant":
			rolePrefix = "Loco:"
			contentStyle = getAssistantStyle()
		case "system":
			// Check if this is analysis results
			if strings.Contains(msg.Content, "Analysis Results") {
				rolePrefix = "📊 Analysis:"
			} else {
				rolePrefix = "🔧 System:"
			}
			contentStyle = getSystemStyle()
		}

		// Add role prefix
		sb.WriteString(rolePrefix)
		sb.WriteString("\n")

		// Render content
		content := msg.Content

		// Apply markdown rendering for assistant messages
		if msg.Role == "assistant" {
			rendered, err := ml.renderMarkdown(content)
			if err == nil {
				content = rendered
			}
		} else {
			// Apply word wrapping for non-assistant messages
			content = ml.wrapText(content, ml.width-4)
		}

		// Apply style
		sb.WriteString(contentStyle.Render(content))
		
		// Render tool calls if present
		if len(msg.ToolCalls) > 0 {
			sb.WriteString("\n\n")
			for _, toolCall := range msg.ToolCalls {
				// For now, render tool calls without results
				// In real implementation, we'd track tool results separately
				toolView := ml.toolRegistry.Get(toolCall.Name).Render(toolCall, nil, ml.width-4)
				sb.WriteString(toolView)
				sb.WriteString("\n")
			}
		}

		// Add metadata if in debug mode
		if ml.showDebug {
			if meta, exists := ml.messagesMeta[i]; exists {
				metaInfo := ml.formatMetadata(meta)
				sb.WriteString("\n")
				sb.WriteString(getMetaStyle().Render(metaInfo))
			}
		}

		sb.WriteString("\n")
	}

	// Add streaming message if any
	if ml.isStreaming && ml.streamingMsg != "" {
		sb.WriteString("Loco:\n")
		sb.WriteString(getAssistantStyle().Render(ml.streamingMsg))
		sb.WriteString(" ")
		sb.WriteString(ml.spinner.View())
		sb.WriteString("\n")
	}

	return sb.String()
}

func (ml *MessageListModel) formatMetadata(meta *MessageMetadata) string {
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

func (ml *MessageListModel) renderMarkdown(content string) (string, error) {
	// Create a glamour renderer with a custom style
	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dracula"),
		glamour.WithWordWrap(ml.width-4), // Account for padding
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
func (ml *MessageListModel) wrapText(text string, width int) string {
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

// Message styling functions that use current theme
func getUserStyle() lipgloss.Style {
	theme := styles.CurrentTheme()
	return lipgloss.NewStyle().
		Foreground(theme.Accent).
		Bold(true)
}

func getAssistantStyle() lipgloss.Style {
	theme := styles.CurrentTheme()
	return lipgloss.NewStyle().
		Foreground(theme.Primary)
}

func getSystemStyle() lipgloss.Style {
	theme := styles.CurrentTheme()
	return lipgloss.NewStyle().
		Foreground(theme.FgSubtle).
		Italic(true)
}

func getMetaStyle() lipgloss.Style {
	theme := styles.CurrentTheme()
	return lipgloss.NewStyle().
		Foreground(theme.FgMuted).
		Italic(true)
}