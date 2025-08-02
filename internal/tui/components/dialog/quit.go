package dialog

import (
	"github.com/billie-coop/loco/internal/tui/events"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// QuitDialog asks for confirmation before quitting
type QuitDialog struct {
	*BaseDialog

	selectedNo  bool // true if "No" is selected (default for safety)
	eventBroker *events.Broker

	// Styling
	buttonStyle         lipgloss.Style
	selectedButtonStyle lipgloss.Style
	questionStyle       lipgloss.Style
}

// NewQuitDialog creates a new quit confirmation dialog
func NewQuitDialog(eventBroker *events.Broker) *QuitDialog {
	d := &QuitDialog{
		BaseDialog:  NewBaseDialog("Quit Loco?"),
		selectedNo:  true, // Default to "No" for safety
		eventBroker: eventBroker,

		questionStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")),

		buttonStyle: lipgloss.NewStyle().
			Padding(0, 3).
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("252")),

		selectedButtonStyle: lipgloss.NewStyle().
			Padding(0, 3).
			Background(lipgloss.Color("205")).
			Foreground(lipgloss.Color("0")).
			Bold(true),
	}
	return d
}

// Init initializes the dialog
func (d *QuitDialog) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (d *QuitDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !d.isOpen {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "n", "N":
			// Close dialog without quitting
			return d, d.Close()
		case "y", "Y":
			// Quit immediately
			return d, tea.Quit
		case "left", "right", "tab", "h", "l":
			// Toggle selection
			d.selectedNo = !d.selectedNo
		case "enter", " ":
			// Execute selected option
			if d.selectedNo {
				return d, d.Close()
			} else {
				return d, tea.Quit
			}
		}
	}

	return d, nil
}

// View renders the dialog
func (d *QuitDialog) View() string {
	if !d.isOpen {
		return ""
	}

	// Render the title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		MarginBottom(1)
	title := titleStyle.Render("Quit Loco?")

	question := d.questionStyle.Render("Are you sure you want to quit?")

	// Style the buttons based on selection
	yesStyle := d.buttonStyle
	noStyle := d.buttonStyle
	if d.selectedNo {
		noStyle = d.selectedButtonStyle
	} else {
		yesStyle = d.selectedButtonStyle
	}

	yesButton := yesStyle.Render("Yes")
	noButton := noStyle.Render("No")

	// Join buttons horizontally with spacing
	buttons := lipgloss.JoinHorizontal(
		lipgloss.Center,
		yesButton,
		"  ",
		noButton,
	)

	// Center the buttons under the question
	buttonsContainer := lipgloss.NewStyle().
		Width(lipgloss.Width(question)).
		Align(lipgloss.Right).
		Render(buttons)

	// Join all elements vertically
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		question,
		"",
		buttonsContainer,
	)

	// Create a smaller custom dialog - much smaller than base dialog default
	dialogStyle := lipgloss.NewStyle().
		Padding(1, 3).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205"))

	dialog := dialogStyle.Render(content)

	// Create overlay with semi-transparent background
	overlay := lipgloss.NewStyle().
		Width(d.Width).
		Height(d.Height).
		Background(lipgloss.Color("0"))

	// Place the dialog centered on the overlay
	overlayView := overlay.Render(lipgloss.Place(
		d.Width,
		d.Height,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
	))

	return overlayView
}