package chat

import (
	"strings"

	"github.com/billie-coop/loco/internal/tui/components/core"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// SimpleInputModel is a basic single-line input that works reliably
type SimpleInputModel struct {
	value       string
	placeholder string
	cursorPos   int
	width       int
	height      int
	focused     bool
	enabled     bool
}

// Ensure SimpleInputModel implements required interfaces
var _ core.Component = (*SimpleInputModel)(nil)
var _ core.Sizeable = (*SimpleInputModel)(nil)
var _ core.Focusable = (*SimpleInputModel)(nil)

// NewSimpleInput creates a new simple input component
func NewSimpleInput() *SimpleInputModel {
	return &SimpleInputModel{
		value:       "",
		placeholder: "Type a message or use /help for commands",
		cursorPos:   0,
		focused:     true,
		enabled:     true,
	}
}

// Init initializes the input component
func (im *SimpleInputModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the input component
func (im *SimpleInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !im.enabled || !im.focused {
		return im, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
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
		case "enter", "tab", "esc", "ctrl+c", "ctrl+l":
			// Don't handle these - let parent handle them
			return im, nil
		default:
			// Regular character input
			if len(msg.String()) == 1 && msg.String()[0] >= 32 && msg.String()[0] < 127 {
				im.value = im.value[:im.cursorPos] + msg.String() + im.value[im.cursorPos:]
				im.cursorPos++
			}
		}
	}

	return im, nil
}

// SetSize sets the dimensions of the input component
func (im *SimpleInputModel) SetSize(width, height int) tea.Cmd {
	im.width = width
	im.height = height
	return nil
}

// View renders the input component
func (im *SimpleInputModel) View() string {
	// Create styles
	inputStyle := lipgloss.NewStyle().
		Width(im.width - 2).
		Padding(0, 1)

	var display string
	if im.value == "" && im.placeholder != "" && !im.focused {
		// Show placeholder
		placeholderStyle := inputStyle.Copy().Foreground(lipgloss.Color("241"))
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
			
			cursorStyle := lipgloss.NewStyle().
				Background(lipgloss.Color("205")).
				Foreground(lipgloss.Color("0"))
			
			display = inputStyle.Render(before + cursorStyle.Render(cursor) + after)
		} else {
			display = inputStyle.Render(im.value)
		}
	}

	return display
}

// Focus focuses the input component
func (im *SimpleInputModel) Focus() tea.Cmd {
	im.focused = true
	return nil
}

// Blur removes focus from the input component
func (im *SimpleInputModel) Blur() tea.Cmd {
	im.focused = false
	return nil
}

// Focused returns whether the input component is focused
func (im *SimpleInputModel) Focused() bool {
	return im.focused
}

// Value returns the current input value
func (im *SimpleInputModel) Value() string {
	return im.value
}

// SetValue sets the input value
func (im *SimpleInputModel) SetValue(value string) {
	im.value = value
	im.cursorPos = len(value)
}

// Reset clears the input
func (im *SimpleInputModel) Reset() {
	im.value = ""
	im.cursorPos = 0
}

// CursorEnd moves the cursor to the end
func (im *SimpleInputModel) CursorEnd() {
	im.cursorPos = len(im.value)
}

// SetEnabled enables or disables the input
func (im *SimpleInputModel) SetEnabled(enabled bool) {
	im.enabled = enabled
	if !enabled {
		im.focused = false
	}
}

// IsEmpty returns true if the input is empty
func (im *SimpleInputModel) IsEmpty() bool {
	return strings.TrimSpace(im.value) == ""
}

// IsSlashCommand returns true if the input starts with a slash
func (im *SimpleInputModel) IsSlashCommand() bool {
	return strings.HasPrefix(strings.TrimSpace(im.value), "/")
}

// SetPlaceholder sets the placeholder text
func (im *SimpleInputModel) SetPlaceholder(placeholder string) {
	im.placeholder = placeholder
}