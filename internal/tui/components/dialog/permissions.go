package dialog

import (
	"fmt"
	"strings"

	"github.com/billie-coop/loco/internal/tui/events"
	"github.com/billie-coop/loco/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// PermissionsDialog asks user to approve or deny tool execution
type PermissionsDialog struct {
	*BaseDialog

	toolName       string
	toolArgs       map[string]interface{}
	requestID      string
	decision       string // "approve", "deny", "always", "never"
	selectedOption int    // 0=Approve, 1=Deny, 2=Always, 3=Never
	eventBroker    *events.Broker

	// Styling
	toolStyle     lipgloss.Style
	argsStyle     lipgloss.Style
	warningStyle  lipgloss.Style
	optionStyle   lipgloss.Style
	selectedStyle lipgloss.Style
}

// NewPermissionsDialog creates a new permissions dialog
func NewPermissionsDialog(eventBroker *events.Broker) *PermissionsDialog {
	theme := styles.CurrentTheme()
	
	d := &PermissionsDialog{
		BaseDialog:  NewBaseDialog("Tool Execution Request"),
		eventBroker: eventBroker,

		toolStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Accent),

		argsStyle: lipgloss.NewStyle().
			Foreground(theme.FgMuted).
			PaddingLeft(2),

		warningStyle: lipgloss.NewStyle().
			Foreground(theme.Warning).
			Bold(true),

		optionStyle: lipgloss.NewStyle().
			PaddingLeft(2),

		selectedStyle: lipgloss.NewStyle().
			Foreground(theme.Primary).
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
		case "up", "k":
			d.selectedOption = (d.selectedOption + 3) % 4
			return d, nil
		case "down", "j", "tab":
			d.selectedOption = (d.selectedOption + 1) % 4
			return d, nil
		case "esc":
			d.decision = "deny"
			return d, d.publishDecision()
		case "y", "Y":
			d.selectedOption = 0
			d.decision = "approve"
			return d, d.publishDecision()
		case "n", "N":
			d.selectedOption = 1
			d.decision = "deny"
			return d, d.publishDecision()
		case "a", "A":
			d.selectedOption = 2
			d.decision = "always"
			return d, d.publishDecision()
		case "d", "D":
			d.selectedOption = 3
			d.decision = "never"
			return d, d.publishDecision()
		case "enter", " ":
			// Select based on current selection
			switch d.selectedOption {
			case 0:
				d.decision = "approve"
			case 1:
				d.decision = "deny"
			case 2:
				d.decision = "always"
			case 3:
				d.decision = "never"
			}
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

	theme := styles.CurrentTheme()
	var content strings.Builder

	// Tool name (no duplicate header since BaseDialog has title)
	content.WriteString("Tool requesting permission:\n")
	content.WriteString(d.toolStyle.Render(fmt.Sprintf("  🔧 %s", d.toolName)) + "\n\n")

	// Arguments - display in a consistent order
	if len(d.toolArgs) > 0 {
		// Extract specific fields we know about
		if action, ok := d.toolArgs["action"].(string); ok && action != "" {
			content.WriteString("Action: ")
			content.WriteString(theme.S().Info.Render(action) + "\n")
		}
		if path, ok := d.toolArgs["path"].(string); ok && path != "" {
			content.WriteString("Path: ")
			content.WriteString(theme.S().Muted.Render(path) + "\n")
		}
		if desc, ok := d.toolArgs["description"].(string); ok && desc != "" {
			content.WriteString("Description: ")
			content.WriteString(theme.S().Text.Render(desc) + "\n")
		}
		content.WriteString("\n")
	}

	// Security warning for certain tools
	if d.isHighRiskTool() {
		content.WriteString(d.warningStyle.Render("⚠️  This tool can modify files on your system!") + "\n\n")
	}

	// Options - properly formatted as buttons (vertical layout)
	content.WriteString("Choose an action:\n\n")
	
	// Create button-like appearance for options
	buttons := []string{
		fmt.Sprintf(" [Y] Approve once "),
		fmt.Sprintf(" [N] Deny "),
		fmt.Sprintf(" [A] Always allow "),
		fmt.Sprintf(" [D] Never allow "),
	}
	
	for i, button := range buttons {
		if i == d.selectedOption {
			content.WriteString(d.selectedStyle.Render("▶ " + button))
		} else {
			content.WriteString(d.optionStyle.Render("  " + button))
		}
		content.WriteString("\n")
	}
	
	content.WriteString("\n")
	content.WriteString(theme.S().Subtle.Render("↵ Enter to select • Esc to cancel"))

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