package dialog

import (
	"github.com/billie-coop/loco/internal/tui/events"
	"github.com/billie-coop/loco/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// HelpDialog displays help information
type HelpDialog struct {
	*BaseDialog

	eventBroker *events.Broker
	activeTab   int
	tabs        []string

	// Styling
	tabStyle         lipgloss.Style
	activeTabStyle   lipgloss.Style
	contentStyle     lipgloss.Style
	keyStyle         lipgloss.Style
	descStyle        lipgloss.Style
	sectionStyle     lipgloss.Style
}

// NewHelpDialog creates a new help dialog
func NewHelpDialog(eventBroker *events.Broker) *HelpDialog {
	theme := styles.CurrentTheme()
	
	d := &HelpDialog{
		BaseDialog:  NewBaseDialog("Help"),
		eventBroker: eventBroker,
		tabs:        []string{"Commands", "Keyboard Shortcuts", "Tips"},
		activeTab:   0,

		tabStyle: lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(theme.FgSubtle),

		activeTabStyle: lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(theme.Accent).
			Bold(true).
			Underline(true),

		contentStyle: lipgloss.NewStyle().
			MarginTop(1),

		keyStyle: lipgloss.NewStyle().
			Foreground(theme.Primary).
			Bold(true),

		descStyle: lipgloss.NewStyle().
			Foreground(theme.FgMuted),

		sectionStyle: lipgloss.NewStyle().
			MarginBottom(1).
			Foreground(theme.Accent).
			Bold(true),
	}

	return d
}

// Init initializes the dialog
func (d *HelpDialog) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (d *HelpDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !d.isOpen {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "?":
			return d, d.Close()
		case "tab", "right", "l":
			d.activeTab = (d.activeTab + 1) % len(d.tabs)
		case "shift+tab", "left", "h":
			d.activeTab = (d.activeTab - 1 + len(d.tabs)) % len(d.tabs)
		case "1":
			d.activeTab = 0
		case "2":
			d.activeTab = 1
		case "3":
			d.activeTab = 2
		}
	}

	return d, nil
}

// View renders the dialog
func (d *HelpDialog) View() string {
	if !d.isOpen {
		return ""
	}

	// Render tabs
	var tabs []string
	for i, tab := range d.tabs {
		style := d.tabStyle
		if i == d.activeTab {
			style = d.activeTabStyle
		}
		tabs = append(tabs, style.Render(tab))
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	// Render content based on active tab
	var content string
	switch d.activeTab {
	case 0:
		content = d.renderCommands()
	case 1:
		content = d.renderKeyboardShortcuts()
	case 2:
		content = d.renderTips()
	}

	// Combine tabs and content
	fullContent := lipgloss.JoinVertical(
		lipgloss.Left,
		tabBar,
		d.contentStyle.Render(content),
	)

	return d.RenderDialog(fullContent)
}

func (d *HelpDialog) renderCommands() string {
	commands := [][]string{
		{"/help", "Show this help message"},
		{"/clear", "Clear all messages"},
		{"/model", "Show current model"},
		{"/model select", "Select a different model"},
		{"/team", "Show current team"},
		{"/team select", "Select a model team"},
		{"/settings", "Open settings dialog"},
		{"/debug", "Toggle debug mode"},
		{"/quit or /exit", "Exit the application"},
	}

	var lines []string
	lines = append(lines, d.sectionStyle.Render("Available Commands"))
	
	for _, cmd := range commands {
		key := d.keyStyle.Render(cmd[0])
		desc := d.descStyle.Render(" - " + cmd[1])
		lines = append(lines, key+desc)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (d *HelpDialog) renderKeyboardShortcuts() string {
	shortcuts := [][]string{
		{"Ctrl+C", "Quit confirmation dialog"},
		{"Ctrl+L", "Clear messages"},
		{"Ctrl+P", "Open command palette"},
		{"Tab", "Command completion"},
		{"Esc", "Clear input / Close dialogs"},
		{"↑/↓ or j/k", "Navigate in lists"},
		{"Enter", "Send message / Select item"},
	}

	var lines []string
	lines = append(lines, d.sectionStyle.Render("Keyboard Shortcuts"))
	
	for _, shortcut := range shortcuts {
		key := d.keyStyle.Render(shortcut[0])
		desc := d.descStyle.Render(" - " + shortcut[1])
		lines = append(lines, key+desc)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (d *HelpDialog) renderTips() string {
	tips := []string{
		"• Start typing '/' to see available commands",
		"• Press Tab after '/' for command completion",
		"• Use Ctrl+P to quickly access any command",
		"• The sidebar shows your current model and session info",
		"• Debug mode shows token counts and timing info",
		"• Model teams let you switch between different model sizes",
		"• All dialogs can be closed with Esc",
	}

	var lines []string
	lines = append(lines, d.sectionStyle.Render("Tips & Tricks"))
	
	for _, tip := range tips {
		lines = append(lines, d.descStyle.Render(tip))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}