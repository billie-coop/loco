package chat

import (
	"fmt"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/tui/components/anim"
	"github.com/billie-coop/loco/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// ToolMessage represents a tool execution message in the chat
type ToolMessage struct {
	message   llm.Message
	width     int
	spinner   *anim.Spinner
	expanded  bool
	startTime time.Time
}

// NewToolMessage creates a new tool message component
func NewToolMessage(msg llm.Message) *ToolMessage {
	tm := &ToolMessage{
		message:   msg,
		expanded:  true, // Start expanded
		startTime: time.Now(),
	}

	// Create spinner for pending/running states
	if msg.ToolExecution != nil && (msg.ToolExecution.Status == "pending" || msg.ToolExecution.Status == "running") {
		tm.spinner = anim.NewSpinner(anim.SpinnerDots)
	}

	return tm
}

// Init implements tea.Model
func (tm *ToolMessage) Init() tea.Cmd {
	if tm.spinner != nil {
		return tm.spinner.Init()
	}
	return nil
}

// Update implements tea.Model
func (tm *ToolMessage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if tm.spinner != nil && tm.message.ToolExecution != nil &&
		(tm.message.ToolExecution.Status == "pending" || tm.message.ToolExecution.Status == "running") {
		s, cmd := tm.spinner.Update(msg)
		if sp, ok := s.(*anim.Spinner); ok {
			tm.spinner = sp
		}
		return tm, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			tm.expanded = !tm.expanded
		}
	}

	return tm, nil
}

// View implements tea.Model
func (tm *ToolMessage) View() string {
	if tm.message.ToolExecution == nil {
		return "" // No tool execution to display
	}

	theme := styles.CurrentTheme()

	// Build header with icon and tool name
	icon := tm.getStatusIcon()
	toolName := tm.prettifyToolName(tm.message.ToolExecution.Name)

	header := fmt.Sprintf("%s %s", icon, toolName)
	if tm.message.ToolExecution.Status == "error" {
		header = styles.RenderThemeGradient(header, false) // Use gradient for errors
	}

	// Add spinner and elapsed time if pending/running
	if tm.spinner != nil && (tm.message.ToolExecution.Status == "pending" || tm.message.ToolExecution.Status == "running") {
		elapsed := time.Since(tm.startTime).Round(100 * time.Millisecond)
		header = fmt.Sprintf("%s %s  ⏱ %s", header, tm.spinner.View(), elapsed)
	}

	// Add progress message if available
	if tm.message.ToolExecution.Progress != "" {
		// Just use plain text for progress
		header = fmt.Sprintf("%s\n  %s", header, tm.message.ToolExecution.Progress)
	}

	// Compose body (details/content when expanded)
	body := header
	if tm.expanded && tm.message.Content != "" {
		content := tm.renderContent()
		body = fmt.Sprintf("%s\n%s", header, content)
	}

	// Choose border color by status
	borderStyle := theme.BorderFocus
	switch tm.message.ToolExecution.Status {
	case "complete":
		borderStyle = theme.Success
	case "error":
		borderStyle = theme.Error
	case "pending", "running":
		borderStyle = theme.Warning
	}

	// Render in a visible card/bubble
	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderStyle).
		Background(theme.BgBaseLighter).
		Padding(0, 1).
		Width(max(10, tm.width-1))

	// Footer with timestamp and duration/elapsed
	elapsed := time.Since(tm.startTime).Round(100 * time.Millisecond)
	footer := theme.S().Subtle.Italic(true).Render(
		fmt.Sprintf("  %s • %s", tm.startTime.Format("15:04:05"), elapsed),
	)

	return card.Render(body + "\n" + footer)
}

// SetMessage updates the message (for progress updates)
func (tm *ToolMessage) SetMessage(msg llm.Message) {
	tm.message = msg

	// Stop spinner if completed
	if msg.ToolExecution != nil && (msg.ToolExecution.Status == "complete" || msg.ToolExecution.Status == "error") {
		tm.spinner = nil
	}
}

// SetWidth sets the width for rendering
func (tm *ToolMessage) SetWidth(width int) {
	tm.width = width
}

func (tm *ToolMessage) getStatusIcon() string {
	if tm.message.ToolExecution == nil {
		return "🔧"
	}
	switch tm.message.ToolExecution.Status {
	case "pending":
		return "⏳"
	case "running":
		return "🔧"
	case "complete":
		return "✅"
	case "error":
		return "❌"
	default:
		return "🔧"
	}
}

func (tm *ToolMessage) prettifyToolName(name string) string {
	// Convert snake_case to Title Case
	parts := strings.Split(name, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}

func (tm *ToolMessage) renderContent() string {
	if tm.message.ToolExecution == nil {
		return ""
	}
	// Special rendering based on tool type
	switch tm.message.ToolExecution.Name {
	case "startup_scan":
		return tm.renderStartupScan()
	case "analyze":
		return tm.renderAnalyze()
	case "copy":
		return tm.renderCopy()
	default:
		// Generic rendering - just indent the content
		lines := strings.Split(tm.message.Content, "\n")
		for i, line := range lines {
			lines[i] = "  " + line
		}
		return strings.Join(lines, "\n")
	}
}

func (tm *ToolMessage) renderStartupScan() string {
	// Parse the content to extract project info or show details
	lines := strings.Split(tm.message.Content, "\n")
	var output []string

	for _, line := range lines {
		if strings.Contains(line, "Project type:") ||
			strings.Contains(line, "Language:") ||
			strings.Contains(line, "Framework:") ||
			strings.Contains(line, "Files:") ||
			strings.HasPrefix(line, "Args:") ||
			strings.HasPrefix(line, "Initiator:") ||
			strings.HasPrefix(line, "Session:") ||
			strings.HasPrefix(line, "CWD:") {
			output = append(output, "  "+line)
		}
	}

	if len(output) == 0 {
		// Just indent all lines
		for _, line := range lines {
			output = append(output, "  "+line)
		}
	}

	return strings.Join(output, "\n")
}

func (tm *ToolMessage) renderAnalyze() string {
	// Extract key information from analysis or show details
	lines := strings.Split(tm.message.Content, "\n")
	var output []string

	for _, line := range lines {
		if strings.Contains(line, "Tier:") ||
			strings.Contains(line, "Files analyzed:") ||
			strings.Contains(line, "Documents generated:") ||
			strings.Contains(line, "Duration:") ||
			strings.HasPrefix(line, "Args:") ||
			strings.HasPrefix(line, "Initiator:") ||
			strings.HasPrefix(line, "Session:") ||
			strings.HasPrefix(line, "CWD:") {
			output = append(output, "  "+line)
		} else if line != "" {
			output = append(output, "  "+line)
		}
	}

	if len(output) == 0 {
		// Just indent all lines
		for _, line := range lines {
			output = append(output, "  "+line)
		}
	}

	return strings.Join(output, "\n")
}

func (tm *ToolMessage) renderCopy() string {
	// Just show with a checkmark
	return "  ✓ " + tm.message.Content
}

// max helper for width safety
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
