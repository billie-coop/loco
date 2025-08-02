package dialog

import (
	"strings"

	"github.com/billie-coop/loco/internal/tui/events"
	"github.com/billie-coop/loco/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// Command represents a command in the palette
type Command struct {
	Name        string
	Description string
	Shortcut    string
	Action      string // The actual command to execute
}

// CommandPaletteDialog displays a searchable list of all commands
type CommandPaletteDialog struct {
	*BaseDialog

	commands         []Command
	filteredCommands []Command
	searchQuery      string
	selectedIndex    int
	eventBroker      *events.Broker

	// Styling
	searchStyle       lipgloss.Style
	itemStyle         lipgloss.Style
	selectedItemStyle lipgloss.Style
	shortcutStyle     lipgloss.Style
	descStyle         lipgloss.Style
}

// NewCommandPaletteDialog creates a new command palette dialog
func NewCommandPaletteDialog(eventBroker *events.Broker) *CommandPaletteDialog {
	theme := styles.CurrentTheme()
	
	d := &CommandPaletteDialog{
		BaseDialog:  NewBaseDialog("Command Palette"),
		eventBroker: eventBroker,

		searchStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Primary).
			Padding(0, 1).
			MarginBottom(1),

		itemStyle: lipgloss.NewStyle().
			Padding(0, 2),

		selectedItemStyle: lipgloss.NewStyle().
			Padding(0, 2).
			Background(theme.Primary).
			Foreground(theme.FgInverted),

		shortcutStyle: lipgloss.NewStyle().
			Foreground(theme.FgSubtle),

		descStyle: lipgloss.NewStyle().
			Foreground(theme.FgMuted),
	}

	// Initialize default commands
	d.commands = []Command{
		{Name: "Help", Description: "Show help message", Shortcut: "/help", Action: "/help"},
		{Name: "Clear Messages", Description: "Clear all messages", Shortcut: "/clear", Action: "/clear"},
		{Name: "Select Model", Description: "Choose an LLM model", Shortcut: "/model select", Action: "/model select"},
		{Name: "Show Current Model", Description: "Display current model", Shortcut: "/model", Action: "/model"},
		{Name: "Select Team", Description: "Choose a model team", Shortcut: "/team select", Action: "/team select"},
		{Name: "Show Current Team", Description: "Display current team", Shortcut: "/team", Action: "/team"},
		{Name: "Settings", Description: "Open settings dialog", Shortcut: "/settings", Action: "/settings"},
		{Name: "Toggle Debug", Description: "Toggle debug mode", Shortcut: "/debug", Action: "/debug"},
		{Name: "Quit", Description: "Exit the application", Shortcut: "/quit", Action: "/quit"},
		{Name: "Clear Messages", Description: "Clear the message history", Shortcut: "Ctrl+L", Action: "ctrl+l"},
		{Name: "Command Palette", Description: "Open this command palette", Shortcut: "Ctrl+P", Action: "ctrl+p"},
	}

	d.filteredCommands = d.commands
	return d
}

// Init initializes the dialog
func (d *CommandPaletteDialog) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (d *CommandPaletteDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !d.isOpen {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return d, d.Close()
		case "up", "k":
			if d.selectedIndex > 0 {
				d.selectedIndex--
			}
		case "down", "j":
			if d.selectedIndex < len(d.filteredCommands)-1 {
				d.selectedIndex++
			}
		case "enter":
			if d.selectedIndex < len(d.filteredCommands) {
				cmd := d.filteredCommands[d.selectedIndex]
				d.result = cmd.Action
				// Publish command execution event
				d.eventBroker.PublishAsync(events.Event{
					Type: events.CommandSelectedEvent,
					Payload: events.CommandSelectedPayload{
						Command: cmd.Action,
					},
				})
				return d, d.Close()
			}
		case "backspace":
			if len(d.searchQuery) > 0 {
				d.searchQuery = d.searchQuery[:len(d.searchQuery)-1]
				d.filterCommands()
			}
		default:
			// Handle text input for search
			if len(msg.String()) == 1 && msg.String()[0] >= 32 && msg.String()[0] < 127 {
				d.searchQuery += msg.String()
				d.filterCommands()
			}
		}
	}

	return d, nil
}

// View renders the dialog
func (d *CommandPaletteDialog) View() string {
	if !d.isOpen {
		return ""
	}

	// Set a reasonable width for command palette
	maxWidth := 80

	// Search box
	searchBox := d.searchStyle.Width(maxWidth).Render("🔍 " + d.searchQuery)

	// Command list
	var items []string
	visibleItems := 10 // Show max 10 items

	start := 0
	if d.selectedIndex >= visibleItems {
		start = d.selectedIndex - visibleItems + 1
	}

	end := start + visibleItems
	if end > len(d.filteredCommands) {
		end = len(d.filteredCommands)
	}

	for i := start; i < end; i++ {
		cmd := d.filteredCommands[i]
		style := d.itemStyle
		if i == d.selectedIndex {
			style = d.selectedItemStyle
		}

		name := cmd.Name
		shortcut := d.shortcutStyle.Render(" " + cmd.Shortcut)
		desc := d.descStyle.Render(" - " + cmd.Description)

		item := style.Width(maxWidth).Render(name + shortcut + desc)
		items = append(items, item)
	}

	// Join all items
	list := lipgloss.JoinVertical(lipgloss.Left, items...)

	// Combine search and list
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		searchBox,
		list,
	)

	// Use the base dialog renderer which now auto-sizes
	return d.RenderDialog(content)
}

// filterCommands filters commands based on search query
func (d *CommandPaletteDialog) filterCommands() {
	query := strings.ToLower(d.searchQuery)
	d.filteredCommands = nil

	for _, cmd := range d.commands {
		if query == "" ||
			strings.Contains(strings.ToLower(cmd.Name), query) ||
			strings.Contains(strings.ToLower(cmd.Description), query) ||
			strings.Contains(strings.ToLower(cmd.Shortcut), query) {
			d.filteredCommands = append(d.filteredCommands, cmd)
		}
	}

	// Reset selection if out of bounds
	if d.selectedIndex >= len(d.filteredCommands) {
		d.selectedIndex = 0
	}
}

// Open opens the dialog
func (d *CommandPaletteDialog) Open() tea.Cmd {
	// Reset search on open
	d.searchQuery = ""
	d.selectedIndex = 0
	d.filterCommands()
	return d.BaseDialog.Open()
}