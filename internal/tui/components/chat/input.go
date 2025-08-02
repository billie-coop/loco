package chat

import (
	"strings"

	"github.com/billie-coop/loco/internal/tui/components/core"
	"github.com/charmbracelet/bubbles/v2/textarea"
	tea "github.com/charmbracelet/bubbletea/v2"
)

// InputModel implements the text input component for chat
type InputModel struct {
	textarea textarea.Model
	width    int
	height   int

	// State
	isEnabled bool
}

// Ensure InputModel implements required interfaces
var _ core.Component = (*InputModel)(nil)
var _ core.Sizeable = (*InputModel)(nil)
var _ core.Focusable = (*InputModel)(nil)

// NewInput creates a new input component
func NewInput() *InputModel {
	ta := textarea.New()
	ta.Placeholder = "Type a message or use /help for commands"
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.CharLimit = 0         // No character limit
	ta.SetHeight(3)          // Allow 3 lines for better multi-line input
	// Note: DeleteLine removal might be in newer bubbles version

	return &InputModel{
		textarea:  ta,
		isEnabled: true,
	}
}

// Init initializes the input component
func (im *InputModel) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles messages for the input component
func (im *InputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !im.isEnabled {
		return im, nil
	}

	var cmd tea.Cmd
	im.textarea, cmd = im.textarea.Update(msg)
	return im, cmd
}

// SetSize sets the dimensions of the input component
func (im *InputModel) SetSize(width, height int) tea.Cmd {
	im.width = width
	im.height = height

	im.textarea.SetWidth(width - 2) // Account for padding
	im.textarea.SetHeight(height)

	return nil
}

// View renders the input component
func (im *InputModel) View() string {
	return im.textarea.View()
}

// Focus focuses the input component
func (im *InputModel) Focus() tea.Cmd {
	return im.textarea.Focus()
}

// Blur removes focus from the input component
func (im *InputModel) Blur() tea.Cmd {
	im.textarea.Blur()
	return nil
}

// Focused returns whether the input component is focused
func (im *InputModel) Focused() bool {
	return im.textarea.Focused()
}

// Value returns the current input value
func (im *InputModel) Value() string {
	return im.textarea.Value()
}

// SetValue sets the input value
func (im *InputModel) SetValue(value string) {
	im.textarea.SetValue(value)
}

// Reset clears the input
func (im *InputModel) Reset() {
	im.textarea.Reset()
}

// CursorEnd moves the cursor to the end
func (im *InputModel) CursorEnd() {
	im.textarea.CursorEnd()
}

// SetEnabled enables or disables the input
func (im *InputModel) SetEnabled(enabled bool) {
	im.isEnabled = enabled
	if !enabled {
		im.textarea.Blur()
	}
}

// IsEmpty returns true if the input is empty
func (im *InputModel) IsEmpty() bool {
	return strings.TrimSpace(im.textarea.Value()) == ""
}

// IsSlashCommand returns true if the input starts with a slash
func (im *InputModel) IsSlashCommand() bool {
	return strings.HasPrefix(strings.TrimSpace(im.textarea.Value()), "/")
}

// SetPlaceholder sets the placeholder text
func (im *InputModel) SetPlaceholder(placeholder string) {
	im.textarea.Placeholder = placeholder
}