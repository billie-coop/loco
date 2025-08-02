package dialog

import (
	"github.com/billie-coop/loco/internal/tui/components/core"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// BaseDialog provides common dialog functionality
type BaseDialog struct {
	core.FocusableBase
	core.SizeableBase

	title     string
	isOpen    bool
	result    interface{}
	cancelled bool

	// Styling
	borderStyle     lipgloss.Style
	titleStyle      lipgloss.Style
	contentStyle    lipgloss.Style
	overlayStyle    lipgloss.Style
}

// NewBaseDialog creates a new base dialog
func NewBaseDialog(title string) *BaseDialog {
	return &BaseDialog{
		title:     title,
		isOpen:    false,
		cancelled: false,

		borderStyle: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("86")).
			Padding(1),

		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1),

		contentStyle: lipgloss.NewStyle(),

		overlayStyle: lipgloss.NewStyle().
			Background(lipgloss.Color("0")).
			Foreground(lipgloss.Color("7")),
	}
}

// IsOpen returns whether the dialog is open
func (d *BaseDialog) IsOpen() bool {
	return d.isOpen
}

// Open opens the dialog
func (d *BaseDialog) Open() tea.Cmd {
	d.isOpen = true
	d.cancelled = false
	d.result = nil
	return d.Focus()
}

// Close closes the dialog
func (d *BaseDialog) Close() tea.Cmd {
	d.isOpen = false
	return d.Blur()
}

// Cancel closes the dialog as cancelled
func (d *BaseDialog) Cancel() tea.Cmd {
	d.cancelled = true
	return d.Close()
}

// GetResult returns the dialog result
func (d *BaseDialog) GetResult() interface{} {
	return d.result
}

// IsCancelled returns whether the dialog was cancelled
func (d *BaseDialog) IsCancelled() bool {
	return d.cancelled
}

// SetResult sets the dialog result
func (d *BaseDialog) SetResult(result interface{}) {
	d.result = result
}

// RenderDialog renders the dialog with overlay
func (d *BaseDialog) RenderDialog(content string) string {
	if !d.isOpen {
		return ""
	}

	// Render title if present
	var dialogContent string
	if d.title != "" {
		title := d.titleStyle.Render(d.title)
		dialogContent = lipgloss.JoinVertical(lipgloss.Left, title, content)
	} else {
		dialogContent = content
	}

	// Apply border and padding - let it size to content
	dialog := d.borderStyle.Render(dialogContent)

	// Create overlay background
	overlay := d.overlayStyle.
		Width(d.Width).
		Height(d.Height)

	// Center the dialog on the overlay
	overlayView := overlay.Render(lipgloss.Place(
		d.Width,
		d.Height,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
	))

	return overlayView
}

// HandleEscape handles the escape key
func (d *BaseDialog) HandleEscape() tea.Cmd {
	if d.isOpen {
		return d.Cancel()
	}
	return nil
}