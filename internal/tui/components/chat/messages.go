package chat

import (
	"fmt"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/tui/components/core"
	"github.com/billie-coop/loco/internal/tui/components/list"
	"github.com/billie-coop/loco/internal/tui/styles"
	"github.com/charmbracelet/bubbles/v2/spinner"
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

// MessageItem represents a message that can be displayed in the list
type MessageItem interface {
	list.Item  // Must implement the list.Item interface
	GetMessage() llm.Message
	SetMessage(llm.Message)
}

// messageCmp wraps a message for use in the list component
type messageCmp struct {
	message      llm.Message
	meta         *MessageMetadata
	index        int
	width        int
	isStreaming  bool
	streamingMsg string
	spinner      spinner.Model
	showDebug    bool
	toolRegistry *ToolRegistry
	toolMessage  *ToolMessage // For tool execution messages
}

// Ensure messageCmp implements all required interfaces
var _ MessageItem = (*messageCmp)(nil)
var _ list.Item = (*messageCmp)(nil)

// NewMessageCmp creates a new message component
func NewMessageCmp(msg llm.Message, meta *MessageMetadata, showDebug bool, toolRegistry *ToolRegistry) MessageItem {
	mc := &messageCmp{
		message:      msg,
		meta:         meta,
		showDebug:    showDebug,
		toolRegistry: toolRegistry,
		spinner:      spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
	
	// Create tool message component if this is a tool message
	if msg.Role == "tool" && msg.ToolExecution != nil {
		mc.toolMessage = NewToolMessage(msg)
	}
	
	return mc
}

// ID implements list.Item
func (m *messageCmp) ID() string {
	return fmt.Sprintf("msg-%d", m.index)
}

// Init implements tea.Model
func (m *messageCmp) Init() tea.Cmd {
	if m.isStreaming {
		return m.spinner.Tick
	}
	return nil
}

// Update implements tea.Model
func (m *messageCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle tool message updates
	if m.toolMessage != nil {
		tm, cmd := m.toolMessage.Update(msg)
		if toolMsg, ok := tm.(*ToolMessage); ok {
			m.toolMessage = toolMsg
		}
		return m, cmd
	}
	
	if m.isStreaming {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

// View implements tea.ViewModel
func (m *messageCmp) View() string {
	// Early return if no width
	if m.width <= 0 {
		return ""
	}
	
	// If this is a tool message, use the tool message component
	if m.toolMessage != nil {
		m.toolMessage.SetWidth(m.width)
		return m.toolMessage.View()
	}
	
	var sb strings.Builder
	
	// Style based on role
	var rolePrefix string
	var contentStyle lipgloss.Style

	switch m.message.Role {
	case "user":
		rolePrefix = "You:"
		contentStyle = getUserStyle()
	case "assistant":
		rolePrefix = "Loco:"
		contentStyle = getAssistantStyle()
	case "system":
		// Check if this is analysis results
		if strings.Contains(m.message.Content, "Analysis Results") {
			rolePrefix = "📊 Analysis:"
		} else {
			rolePrefix = "🔧 System:"
		}
		contentStyle = getSystemStyle()
	case "tool":
		// Tool messages are handled by toolMessage component above
		return ""
	}

	// Add role prefix with proper spacing
	sb.WriteString(rolePrefix)
	sb.WriteString("\n\n")

	// Render content
	content := m.message.Content

	// Handle streaming
	if m.isStreaming && m.message.Role == "assistant" {
		if m.streamingMsg != "" {
			content = m.streamingMsg
		} else {
			// Show thinking indicator
			sb.WriteString(styles.RenderThemeGradient("🤔 Thinking...", false))
			sb.WriteString("\n")
			return sb.String()
		}
	}

	// Apply markdown rendering for assistant messages
	if m.message.Role == "assistant" && m.width > 4 {
		rendered, err := renderMarkdown(content, m.width-4)
		if err == nil {
			content = rendered
		}
	} else if m.width > 4 {
		// Apply word wrapping for non-assistant messages
		content = wrapText(content, m.width-4)
	}

	// Apply style
	sb.WriteString(contentStyle.Render(content))
	
	// Add spinner if streaming
	if m.isStreaming && m.streamingMsg != "" {
		sb.WriteString(" ")
		sb.WriteString(m.spinner.View())
	}
	
	// Render tool calls if present
	if len(m.message.ToolCalls) > 0 && m.toolRegistry != nil {
		sb.WriteString("\n\n")
		for _, toolCall := range m.message.ToolCalls {
			toolView := m.toolRegistry.Get(toolCall.Name).Render(toolCall, nil, m.width-4)
			sb.WriteString(toolView)
			sb.WriteString("\n")
		}
	}

	// Add metadata if in debug mode
	if m.showDebug && m.meta != nil {
		metaInfo := formatMetadata(m.meta)
		sb.WriteString("\n")
		sb.WriteString(getMetaStyle().Render(metaInfo))
	}

	return sb.String()
}

// GetSize implements list.Item
func (m *messageCmp) GetSize() (int, int) {
	return m.width, 0 // Height is calculated by list
}

// SetSize implements list.Item
func (m *messageCmp) SetSize(width, height int) tea.Cmd {
	m.width = width
	return nil
}

// GetMessage implements MessageItem
func (m *messageCmp) GetMessage() llm.Message {
	return m.message
}

// SetMessage implements MessageItem
func (m *messageCmp) SetMessage(msg llm.Message) {
	m.message = msg
	
	// Update tool message if present
	if m.toolMessage != nil {
		m.toolMessage.SetMessage(msg)
	}
}

// SetIndex sets the index for ID generation
func (m *messageCmp) SetIndex(index int) {
	m.index = index
}

// SetStreaming sets the streaming state
func (m *messageCmp) SetStreaming(isStreaming bool, msg string) {
	m.isStreaming = isStreaming
	m.streamingMsg = msg
}

// MessageListModel implements the message list component using virtualized list
type MessageListModel struct {
	list         list.List[MessageItem]
	width        int
	height       int

	// State
	messages       []llm.Message
	messagesMeta   map[int]*MessageMetadata
	isStreaming    bool
	streamingMsg   string
	showDebug      bool
	
	// Tool rendering
	toolRegistry   *ToolRegistry
	
	// Spinner for creating new items
	spinner        spinner.Model
}

// Ensure MessageListModel implements required interfaces
var _ core.Component = (*MessageListModel)(nil)
var _ core.Sizeable = (*MessageListModel)(nil)

// NewMessageList creates a new message list component
func NewMessageList() *MessageListModel {
	s := spinner.New(spinner.WithSpinner(spinner.Dot))

	// Create list with backward direction (newest at bottom)
	l := list.New([]MessageItem{}, 
		list.WithGap[MessageItem](1),
		list.WithDirectionBackward[MessageItem](),
	)

	return &MessageListModel{
		list:         l,
		spinner:      s,
		messagesMeta: make(map[int]*MessageMetadata),
		toolRegistry: NewToolRegistry(),
	}
}

// Init initializes the message list component
func (ml *MessageListModel) Init() tea.Cmd {
	return tea.Batch(
		ml.list.Init(),
		ml.spinner.Tick,
	)
}

// Update handles messages for the message list
func (ml *MessageListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Update spinner (for creating new items)
	if ml.isStreaming {
		var cmd tea.Cmd
		ml.spinner, cmd = ml.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update list
	listModel, cmd := ml.list.Update(msg)
	if l, ok := listModel.(list.List[MessageItem]); ok {
		ml.list = l
	}
	cmds = append(cmds, cmd)

	return ml, tea.Batch(cmds...)
}

// SetSize sets the dimensions of the message list
func (ml *MessageListModel) SetSize(width, height int) tea.Cmd {
	ml.width = width
	ml.height = height
	
	// Update all existing items with new width
	itemWidth := width - 2
	items := ml.list.Items()
	for _, item := range items {
		if mc, ok := item.(*messageCmp); ok {
			mc.width = itemWidth
		}
	}
	
	// Subtract padding like Crush does
	return ml.list.SetSize(width-2, height-1)
}

// View renders the message list
func (ml *MessageListModel) View() string {
	// If no messages, show welcome
	if len(ml.messages) == 0 && !ml.isStreaming {
		theme := styles.CurrentTheme()
		var sb strings.Builder
		
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
		
		return lipgloss.NewStyle().
			Width(ml.width).
			Height(ml.height).
			Padding(1, 1, 0, 1).
			Render(sb.String())
	}
	
	// Wrap with padding like Crush does
	theme := styles.CurrentTheme()
	return lipgloss.NewStyle().
		Padding(1, 1, 0, 1).
		Width(ml.width).
		Height(ml.height).
		Foreground(theme.FgBase).
		Render(ml.list.View())
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

// AppendStreamingChunk adds content to the current streaming message
func (ml *MessageListModel) AppendStreamingChunk(chunk string) {
	ml.streamingMsg += chunk
	
	// Update the last item if it's streaming
	items := ml.list.Items()
	if len(items) > 0 {
		if lastItem, ok := items[len(items)-1].(*messageCmp); ok && lastItem.isStreaming {
			lastItem.streamingMsg = ml.streamingMsg
			ml.list.UpdateItem(lastItem.ID(), lastItem)
		}
	}
}

// SetDebugMode toggles debug information display
func (ml *MessageListModel) SetDebugMode(showDebug bool) {
	ml.showDebug = showDebug
	ml.refreshContent()
}

// GotoBottom scrolls to the bottom of the list
func (ml *MessageListModel) GotoBottom() {
	ml.list.GoToBottom()
}

// GotoTop scrolls to the top of the list
func (ml *MessageListModel) GotoTop() {
	ml.list.GoToTop()
}

// SetContent sets custom content (no longer needed with virtualized list)
func (ml *MessageListModel) SetContent(content string) {
	// This is a no-op now, kept for compatibility
}

// Private methods

func (ml *MessageListModel) refreshContent() {
	// Convert messages to items
	items := make([]MessageItem, 0, len(ml.messages))
	
	// Calculate item width (accounting for list padding)
	itemWidth := ml.width - 2
	if itemWidth <= 0 {
		itemWidth = 80 // fallback
	}
	
	for i, msg := range ml.messages {
		item := NewMessageCmp(msg, ml.messagesMeta[i], ml.showDebug, ml.toolRegistry)
		if mc, ok := item.(*messageCmp); ok {
			mc.SetIndex(i)
			mc.width = itemWidth
		}
		items = append(items, item)
	}
	
	// Add streaming item if needed
	if ml.isStreaming {
		streamingItem := NewMessageCmp(
			llm.Message{
				Role:    "assistant",
				Content: ml.streamingMsg,
			},
			nil,
			ml.showDebug,
			ml.toolRegistry,
		)
		if mc, ok := streamingItem.(*messageCmp); ok {
			mc.SetIndex(len(items))
			mc.SetStreaming(true, ml.streamingMsg)
			mc.width = itemWidth
			mc.spinner = ml.spinner
		}
		items = append(items, streamingItem)
	}
	
	// Update list
	ml.list.SetItems(items)
	
	// Auto-scroll to bottom
	if len(items) > 0 {
		ml.list.GoToBottom()
	}
}

// Helper functions

func formatMetadata(meta *MessageMetadata) string {
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

func renderMarkdown(content string, width int) (string, error) {
	// Create a glamour renderer with a custom style
	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dracula"),
		glamour.WithWordWrap(width),
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

// wrapText wraps text at word boundaries to fit within the specified width
func wrapText(text string, width int) string {
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