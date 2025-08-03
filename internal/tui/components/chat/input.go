package chat

import (
	"strings"

	"github.com/billie-coop/loco/internal/tui/components/chat/completions"
	"github.com/billie-coop/loco/internal/tui/components/core"
	"github.com/billie-coop/loco/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// Re-export completion types for easier access
type (
	Command              = completions.Command
	OpenCompletionsMsg   = completions.OpenCompletionsMsg
	FilterCompletionsMsg = completions.FilterCompletionsMsg
	CloseCompletionsMsg  = completions.CloseCompletionsMsg
)

// InputModel implements the text input component for chat
type InputModel struct {
	value       string
	placeholder string
	cursorPos   int
	width       int
	height      int
	x           int  // Position on screen
	y           int  // Position on screen
	focused     bool
	enabled     bool
	
	// For completion support
	completionsOpen bool
	completionQuery string
}

// Ensure InputModel implements required interfaces
var _ core.Component = (*InputModel)(nil)
var _ core.Sizeable = (*InputModel)(nil)
var _ core.Focusable = (*InputModel)(nil)

// NewInput creates a new input component
func NewInput() *InputModel {
	return &InputModel{
		value:       "",
		placeholder: "Type a message or use /help for commands",
		cursorPos:   0,
		focused:     true,
		enabled:     true,
	}
}

// Init initializes the input component
func (im *InputModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the input component
func (im *InputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !im.enabled || !im.focused {
		return im, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		keyStr := msg.String()
		
		// Handle tab for triggering completions
		if keyStr == "tab" {
			// Check if we should trigger completions
			word := im.GetCurrentWord()
			if strings.HasPrefix(word, "/") && !im.completionsOpen {
				im.completionsOpen = true
				im.completionQuery = word
				return im, im.triggerCompletions()
			}
			// Otherwise let tab be handled by completions component if open
			return im, nil
		}
		
		// Handle space explicitly - Bubble Tea v2 reports it as "space" not " "
		if keyStr == "space" {
			im.value = im.value[:im.cursorPos] + " " + im.value[im.cursorPos:]
			im.cursorPos++
			
			// Close completions on space
			if im.completionsOpen {
				im.completionsOpen = false
				im.completionQuery = ""
				return im, im.closeCompletions()
			}
			return im, nil
		}
		
		switch keyStr {
		case "backspace":
			if im.cursorPos > 0 {
				im.value = im.value[:im.cursorPos-1] + im.value[im.cursorPos:]
				im.cursorPos--
			}
		case "delete":
			if im.cursorPos < len(im.value) {
				im.value = im.value[:im.cursorPos] + im.value[im.cursorPos+1:]
			}
		case "left":
			if im.cursorPos > 0 {
				im.cursorPos--
			}
		case "right":
			if im.cursorPos < len(im.value) {
				im.cursorPos++
			}
		case "home":
			im.cursorPos = 0
		case "end":
			im.cursorPos = len(im.value)
		case "ctrl+a":
			im.cursorPos = 0
		case "ctrl+e":
			im.cursorPos = len(im.value)
		case "ctrl+k":
			// Kill to end of line
			im.value = im.value[:im.cursorPos]
		case "ctrl+u":
			// Kill to beginning of line
			im.value = im.value[im.cursorPos:]
			im.cursorPos = 0
		default:
			// Regular character input - accept printable ASCII EXCEPT space (handled above)
			s := msg.String()
			if len(s) == 1 {
				char := s[0]
				// Printable ASCII: exclamation (33) through tilde (126)
				// Space (32) is handled explicitly above
				if char >= 33 && char <= 126 {
					im.value = im.value[:im.cursorPos] + s + im.value[im.cursorPos:]
					im.cursorPos++
					
					// Handle special cases AFTER character insertion
					var cmd tea.Cmd
					
					// Check if we just typed a slash at the beginning or after whitespace
					if char == '/' && (im.cursorPos == 1 || (im.cursorPos > 1 && im.value[im.cursorPos-2] == ' ')) {
						im.completionsOpen = true
						im.completionQuery = ""
						cmd = im.triggerCompletions()
					}
					
					// Close completions on space
					if char == ' ' && im.completionsOpen {
						im.completionsOpen = false
						im.completionQuery = ""
						cmd = im.closeCompletions()
					}
					
					// Return with any command that was generated
					if cmd != nil {
						return im, cmd
					}
				}
			}
		}
	}

	return im, nil
}

// SetSize sets the dimensions of the input component
func (im *InputModel) SetSize(width, height int) tea.Cmd {
	im.width = width
	im.height = height
	return nil
}

// View renders the input component
func (im *InputModel) View() string {
	theme := styles.CurrentTheme()
	
	// Create styles with theme colors
	inputStyle := lipgloss.NewStyle().
		Width(im.width - 2).
		Padding(0, 1)

	var display string
	if im.value == "" && im.placeholder != "" && !im.focused {
		// Show placeholder with theme colors
		placeholderStyle := inputStyle.Copy().Foreground(theme.FgSubtle)
		display = placeholderStyle.Render(im.placeholder)
	} else {
		// Show value with cursor
		if im.focused && im.enabled {
			// Add cursor
			before := im.value[:im.cursorPos]
			after := ""
			cursor := " "
			
			if im.cursorPos < len(im.value) {
				cursor = string(im.value[im.cursorPos])
				after = im.value[im.cursorPos+1:]
			}
			
			// Use theme primary color for cursor
			cursorStyle := lipgloss.NewStyle().
				Background(theme.Primary).
				Foreground(theme.FgInverted)
			
			display = inputStyle.Render(before + cursorStyle.Render(cursor) + after)
		} else {
			display = inputStyle.Render(im.value)
		}
	}

	return display
}

// Focus focuses the input component
func (im *InputModel) Focus() tea.Cmd {
	im.focused = true
	return nil
}

// Blur removes focus from the input component
func (im *InputModel) Blur() tea.Cmd {
	im.focused = false
	return nil
}

// Focused returns whether the input component is focused
func (im *InputModel) Focused() bool {
	return im.focused
}

// Value returns the current input value
func (im *InputModel) Value() string {
	return im.value
}

// SetValue sets the input value
func (im *InputModel) SetValue(value string) {
	im.value = value
	im.cursorPos = len(value)
}

// Reset clears the input
func (im *InputModel) Reset() {
	im.value = ""
	im.cursorPos = 0
}

// CursorEnd moves the cursor to the end
func (im *InputModel) CursorEnd() {
	im.cursorPos = len(im.value)
}

// SetEnabled enables or disables the input
func (im *InputModel) SetEnabled(enabled bool) {
	im.enabled = enabled
	if !enabled {
		im.focused = false
	}
}

// IsEmpty returns true if the input is empty
func (im *InputModel) IsEmpty() bool {
	return strings.TrimSpace(im.value) == ""
}

// IsSlashCommand returns true if the input starts with a slash
func (im *InputModel) IsSlashCommand() bool {
	return strings.HasPrefix(strings.TrimSpace(im.value), "/")
}

// SetPlaceholder sets the placeholder text
func (im *InputModel) SetPlaceholder(placeholder string) {
	im.placeholder = placeholder
}

// Helper methods for completions

func (im *InputModel) triggerCompletions() tea.Cmd {
	return func() tea.Msg {
		// Get available commands
		commands := []Command{
			{Name: "/analyze", Description: "Analyze the current project"},
			{Name: "/copy", Description: "Copy the last N messages"},
			{Name: "/help", Description: "Show available commands"},
			{Name: "/clear", Description: "Clear the message history"},
			{Name: "/model", Description: "Switch to a different model"},
			{Name: "/session", Description: "Manage chat sessions"},
			{Name: "/quit", Description: "Exit the application"},
		}
		
		// Calculate position for popup
		// X: position of the cursor in the input field
		// Y: position of the input field on screen
		x := im.x + im.cursorPos + 2 // +2 for padding
		y := im.y
		
		return OpenCompletionsMsg{
			Commands: commands,
			X:        x,
			Y:        y,
		}
	}
}

func (im *InputModel) filterCompletions() tea.Cmd {
	return func() tea.Msg {
		return FilterCompletionsMsg{
			Query: im.value,
		}
	}
}

func (im *InputModel) closeCompletions() tea.Cmd {
	return func() tea.Msg {
		return CloseCompletionsMsg{}
	}
}

// HandleCompletionSelect handles when a completion is selected
func (im *InputModel) HandleCompletionSelect(value string) {
	im.value = value
	im.cursorPos = len(value)
	im.completionsOpen = false
	im.completionQuery = ""
}

// IsCompletionsOpen returns true if completions are open
func (im *InputModel) IsCompletionsOpen() bool {
	return im.completionsOpen
}

// GetCurrentWord returns the current word being typed (for slash commands)
func (im *InputModel) GetCurrentWord() string {
	// Find the start of the current word (after last space or beginning)
	start := 0
	for i := im.cursorPos - 1; i >= 0; i-- {
		if im.value[i] == ' ' {
			start = i + 1
			break
		}
	}
	
	// Return the word from start to cursor position
	if start < im.cursorPos {
		return im.value[start:im.cursorPos]
	}
	return ""
}

// SetCompletionsOpen sets the completions open state
func (im *InputModel) SetCompletionsOpen(open bool) {
	im.completionsOpen = open
	if !open {
		im.completionQuery = ""
	}
}

// SetPosition sets the screen position of the input component
func (im *InputModel) SetPosition(x, y int) {
	im.x = x
	im.y = y
}