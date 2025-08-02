package chat

import (
	"strings"

	"github.com/billie-coop/loco/internal/tui/components/core"
	"github.com/billie-coop/loco/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// InputModel implements the text input component for chat
type InputModel struct {
	value       string
	placeholder string
	cursorPos   int
	width       int
	height      int
	focused     bool
	enabled     bool
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
		default:
			// Regular character input - accept printable ASCII including space
			s := msg.String()
			if len(s) == 1 {
				char := s[0]
				// Printable ASCII: space (32) through tilde (126)
				if char >= 32 && char <= 126 {
					im.value = im.value[:im.cursorPos] + s + im.value[im.cursorPos:]
					im.cursorPos++
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