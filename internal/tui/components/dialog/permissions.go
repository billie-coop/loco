package dialog

import (
	"fmt"
	"strings"

	"github.com/billie-coop/loco/internal/tui/events"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// PermissionsDialog asks user to approve or deny tool execution
type PermissionsDialog struct {
	*BaseDialog

	toolName    string
	toolArgs    map[string]interface{}
	requestID   string
	decision    string // "approve", "deny", "always", "never"
	eventBroker *events.Broker

	// Styling
	toolStyle     lipgloss.Style
	argsStyle     lipgloss.Style
	warningStyle  lipgloss.Style
	optionStyle   lipgloss.Style
	selectedStyle lipgloss.Style
}

// NewPermissionsDialog creates a new permissions dialog
func NewPermissionsDialog(eventBroker *events.Broker) *PermissionsDialog {
	d := &PermissionsDialog{
		BaseDialog:  NewBaseDialog("Tool Execution Request"),
		eventBroker: eventBroker,

		toolStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")),

		argsStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			PaddingLeft(2),

		warningStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true),

		optionStyle: lipgloss.NewStyle().
			PaddingLeft(2),

		selectedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true),
	}
	return d
}

// SetToolRequest sets the tool execution request details
func (d *PermissionsDialog) SetToolRequest(toolName string, args map[string]interface{}, requestID string) {
	d.toolName = toolName
	d.toolArgs = args
	d.requestID = requestID
	d.decision = ""
}

// Init initializes the dialog
func (d *PermissionsDialog) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (d *PermissionsDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !d.isOpen {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			d.decision = "deny"
			return d, d.publishDecision()
		case "y", "Y":
			d.decision = "approve"
			return d, d.publishDecision()
		case "n", "N":
			d.decision = "deny"
			return d, d.publishDecision()
		case "a", "A":
			d.decision = "always"
			return d, d.publishDecision()
		case "d", "D":
			d.decision = "never"
			return d, d.publishDecision()
		case "enter":
			// Default to approve on enter
			d.decision = "approve"
			return d, d.publishDecision()
		}
	}

	return d, nil
}

// View renders the dialog
func (d *PermissionsDialog) View() string {
	if !d.isOpen {
		return ""
	}

	var content strings.Builder

	// Warning header
	content.WriteString(d.warningStyle.Render("⚠️  Tool Execution Request") + "\n\n")

	// Tool name
	content.WriteString("The AI wants to execute:\n")
	content.WriteString(d.toolStyle.Render(fmt.Sprintf("  %s", d.toolName)) + "\n\n")

	// Arguments
	if len(d.toolArgs) > 0 {
		content.WriteString("With arguments:\n")
		for key, value := range d.toolArgs {
			argLine := fmt.Sprintf("%s: %v", key, value)
			content.WriteString(d.argsStyle.Render(argLine) + "\n")
		}
		content.WriteString("\n")
	}

	// Security warning for certain tools
	if d.isHighRiskTool() {
		content.WriteString(d.warningStyle.Render("⚠️  This tool can modify files on your system!") + "\n\n")
	}

	// Options
	content.WriteString("Choose an action:\n")
	content.WriteString(d.optionStyle.Render(d.selectedStyle.Render("[Y]") + " Approve this time\n"))
	content.WriteString(d.optionStyle.Render(d.selectedStyle.Render("[N]") + " Deny this time\n"))
	content.WriteString(d.optionStyle.Render(d.selectedStyle.Render("[A]") + " Always allow " + d.toolName + "\n"))
	content.WriteString(d.optionStyle.Render(d.selectedStyle.Render("[D]") + " Never allow " + d.toolName + "\n"))
	content.WriteString("\n")
	content.WriteString(d.argsStyle.Render("Press Enter to approve, Esc to deny"))

	return d.RenderDialog(content.String())
}

func (d *PermissionsDialog) isHighRiskTool() bool {
	highRiskTools := []string{
		"write_file",
		"write",
		"delete_file",
		"execute_command",
		"run_command",
	}

	toolLower := strings.ToLower(d.toolName)
	for _, risk := range highRiskTools {
		if strings.Contains(toolLower, risk) {
			return true
		}
	}
	return false
}

func (d *PermissionsDialog) publishDecision() tea.Cmd {
	// Set result
	d.SetResult(d.decision)

	// Publish the appropriate event based on decision
	eventType := events.ToolExecutionDeniedEvent
	if d.decision == "approve" || d.decision == "always" {
		eventType = events.ToolExecutionApprovedEvent
	}

	if d.eventBroker != nil {
		d.eventBroker.PublishAsync(events.Event{
			Type: eventType,
			Payload: events.ToolExecutionPayload{
				ToolName: d.toolName,
				Args:     d.toolArgs,
				ID:       d.requestID,
			},
		})

		// If "always" or "never", also publish a status message
		if d.decision == "always" {
			d.eventBroker.PublishAsync(events.Event{
				Type: events.StatusMessageEvent,
				Payload: events.StatusMessagePayload{
					Message: fmt.Sprintf("Tool '%s' will be automatically approved", d.toolName),
					Type:    "info",
				},
			})
		} else if d.decision == "never" {
			d.eventBroker.PublishAsync(events.Event{
				Type: events.StatusMessageEvent,
				Payload: events.StatusMessagePayload{
					Message: fmt.Sprintf("Tool '%s' will be automatically denied", d.toolName),
					Type:    "warning",
				},
			})
		}
	}

	return d.Close()
}