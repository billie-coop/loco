package dialog

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// SimpleTextInput is a basic text input field for dialogs
type SimpleTextInput struct {
	value       string
	placeholder string
	focused     bool
	cursorPos   int
	blinkState  bool
}

// NewSimpleTextInput creates a new text input
func NewSimpleTextInput() *SimpleTextInput {
	return &SimpleTextInput{
		value:       "",
		placeholder: "",
		focused:     false,
		cursorPos:   0,
		blinkState:  true,
	}
}

// Value returns the current value
func (t *SimpleTextInput) Value() string {
	return t.value
}

// SetValue sets the value
func (t *SimpleTextInput) SetValue(value string) {
	t.value = value
	t.cursorPos = len(value)
}

// Placeholder sets the placeholder text
func (t *SimpleTextInput) Placeholder(placeholder string) {
	t.placeholder = placeholder
}

// Focus focuses the input
func (t *SimpleTextInput) Focus() {
	t.focused = true
}

// Blur removes focus
func (t *SimpleTextInput) Blur() {
	t.focused = false
}

// Update handles input events
func (t *SimpleTextInput) Update(msg tea.Msg) tea.Cmd {
	if !t.focused {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "backspace":
			if t.cursorPos > 0 {
				t.value = t.value[:t.cursorPos-1] + t.value[t.cursorPos:]
				t.cursorPos--
			}
		case "left":
			if t.cursorPos > 0 {
				t.cursorPos--
			}
		case "right":
			if t.cursorPos < len(t.value) {
				t.cursorPos++
			}
		case "home":
			t.cursorPos = 0
		case "end":
			t.cursorPos = len(t.value)
		default:
			// Regular character input
			if len(msg.String()) == 1 {
				t.value = t.value[:t.cursorPos] + msg.String() + t.value[t.cursorPos:]
				t.cursorPos++
			}
		}
	}

	return nil
}

// View renders the input
func (t *SimpleTextInput) View() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	if !t.focused {
		if t.value == "" && t.placeholder != "" {
			return style.Foreground(lipgloss.Color("241")).Render(t.placeholder)
		}
		return style.Render(t.value)
	}

	// Show cursor
	display := t.value
	if t.cursorPos < len(display) {
		before := display[:t.cursorPos]
		after := display[t.cursorPos+1:]
		cursor := lipgloss.NewStyle().
			Background(lipgloss.Color("205")).
			Foreground(lipgloss.Color("0")).
			Render(string(display[t.cursorPos]))
		display = before + cursor + after
	} else {
		// Cursor at end
		cursor := lipgloss.NewStyle().
			Background(lipgloss.Color("205")).
			Foreground(lipgloss.Color("0")).
			Render(" ")
		display = display + cursor
	}

	return style.Render(display)
}