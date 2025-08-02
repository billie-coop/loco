package status

import (
	"fmt"
	"time"

	"github.com/billie-coop/loco/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// MessageType represents the type of status message
type MessageType int

const (
	Info MessageType = iota
	Warning
	Error
	Success
)

// StatusMessage represents a status bar message
type StatusMessage struct {
	Content   string
	Type      MessageType
	Timestamp time.Time
}

// Component implements a status bar that shows temporary messages
type Component struct {
	message     *StatusMessage
	width       int
	leftContent string
	
	// Timer for clearing messages
	clearAfter time.Duration
}

// New creates a new status bar component
func New() *Component {
	return &Component{
		clearAfter: 5 * time.Second, // Clear messages after 5 seconds
	}
}

// SetMessage sets a status message with the given type
func (c *Component) SetMessage(content string, msgType MessageType) tea.Cmd {
	c.message = &StatusMessage{
		Content:   content,
		Type:      msgType,
		Timestamp: time.Now(),
	}
	
	// Return a command to clear the message after the timeout
	return tea.Tick(c.clearAfter, func(t time.Time) tea.Msg {
		return clearMessageMsg{timestamp: c.message.Timestamp}
	})
}

// ShowInfo shows an info message
func (c *Component) ShowInfo(message string) tea.Cmd {
	return c.SetMessage(message, Info)
}

// ShowWarning shows a warning message
func (c *Component) ShowWarning(message string) tea.Cmd {
	return c.SetMessage(message, Warning)
}

// ShowError shows an error message
func (c *Component) ShowError(message string) tea.Cmd {
	return c.SetMessage(message, Error)
}

// ShowSuccess shows a success message
func (c *Component) ShowSuccess(message string) tea.Cmd {
	return c.SetMessage(message, Success)
}

// SetLeftContent sets the left side content (usually activity indicator)
func (c *Component) SetLeftContent(content string) {
	c.leftContent = content
}

// SetSize implements the Sizeable interface
func (c *Component) SetSize(width, height int) tea.Cmd {
	c.width = width
	return nil
}

// clearMessageMsg is sent when a status message should be cleared
type clearMessageMsg struct {
	timestamp time.Time
}

// Init implements the Component interface
func (c *Component) Init() tea.Cmd {
	return nil
}

// Update implements the Component interface
func (c *Component) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case clearMessageMsg:
		// Only clear if this is for the current message
		if c.message != nil && msg.timestamp.Equal(c.message.Timestamp) {
			c.message = nil
		}
	}
	
	return c, nil
}

// View implements the Component interface
func (c *Component) View() string {
	if c.width == 0 {
		return ""
	}
	
	theme := styles.CurrentTheme()
	
	// Create status bar style with theme colors
	statusStyle := lipgloss.NewStyle().
		Width(c.width).
		Height(1).
		Background(theme.BgSubtle).
		Foreground(theme.FgBase).
		Padding(0, 1)
	
	// Prepare left and right content
	leftContent := c.leftContent
	rightContent := ""
	
	// Add status message to right side if present
	if c.message != nil {
		rightContent = c.formatMessage()
	}
	
	// Calculate available space for content
	availableWidth := c.width - 2 // Account for padding
	
	// Truncate content if necessary
	if len(leftContent)+len(rightContent) > availableWidth {
		if len(rightContent) > 40 {
			rightContent = rightContent[:37] + "..."
		}
		
		remaining := availableWidth - len(rightContent)
		if len(leftContent) > remaining && remaining > 3 {
			leftContent = leftContent[:remaining-3] + "..."
		}
	}
	
	// Create the status bar content
	content := leftContent
	if rightContent != "" {
		// Calculate spacing to right-align the status message
		spacesNeeded := availableWidth - len(leftContent) - len(rightContent)
		if spacesNeeded > 0 {
			content += fmt.Sprintf("%*s%s", spacesNeeded, "", rightContent)
		} else {
			content += " " + rightContent
		}
	}
	
	return statusStyle.Render(content)
}

// formatMessage formats the status message with appropriate styling
func (c *Component) formatMessage() string {
	if c.message == nil {
		return ""
	}
	
	// Add type-specific styling/icons
	switch c.message.Type {
	case Success:
		return "✅ " + c.message.Content
	case Warning:
		return "⚠️ " + c.message.Content
	case Error:
		return "❌ " + c.message.Content
	default: // Info
		return c.message.Content
	}
}