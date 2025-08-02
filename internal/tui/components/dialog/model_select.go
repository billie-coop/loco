package dialog

import (
	"fmt"
	"strings"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/tui/events"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// ModelSelectDialog allows selecting an LLM model
type ModelSelectDialog struct {
	*BaseDialog

	models          []llm.Model
	selectedIndex   int
	eventBroker     *events.Broker
	
	// Styling
	itemStyle       lipgloss.Style
	selectedStyle   lipgloss.Style
	sizeStyle       lipgloss.Style
	descStyle       lipgloss.Style
}

// NewModelSelectDialog creates a new model selection dialog
func NewModelSelectDialog(eventBroker *events.Broker) *ModelSelectDialog {
	d := &ModelSelectDialog{
		BaseDialog:     NewBaseDialog("Select Model"),
		models:         []llm.Model{},
		selectedIndex:  0,
		eventBroker:    eventBroker,

		itemStyle: lipgloss.NewStyle().
			PaddingLeft(2),

		selectedStyle: lipgloss.NewStyle().
			PaddingLeft(1).
			Foreground(lipgloss.Color("205")).
			Bold(true),

		sizeStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")),

		descStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true),
	}
	return d
}

// SetModels sets the available models
func (d *ModelSelectDialog) SetModels(models []llm.Model) {
	d.models = models
	// Try to maintain selection
	if d.selectedIndex >= len(d.models) {
		d.selectedIndex = 0
	}
}

// Init initializes the dialog
func (d *ModelSelectDialog) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (d *ModelSelectDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !d.isOpen {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return d, d.HandleEscape()
		case "q":
			if d.isOpen {
				return d, d.Cancel()
			}
		case "up", "k":
			if d.selectedIndex > 0 {
				d.selectedIndex--
			}
		case "down", "j":
			if d.selectedIndex < len(d.models)-1 {
				d.selectedIndex++
			}
		case "home", "g":
			d.selectedIndex = 0
		case "end", "G":
			d.selectedIndex = len(d.models) - 1
		case "enter":
			if d.selectedIndex < len(d.models) {
				selected := d.models[d.selectedIndex]
				d.SetResult(selected)
				
				// Publish model selected event
				if d.eventBroker != nil {
					d.eventBroker.PublishAsync(events.Event{
						Type: events.ModelSelectedEvent,
						Payload: events.ModelSelectedPayload{
							ModelID:   selected.ID,
							ModelSize: selected.Size,
						},
					})
				}
				
				return d, d.Close()
			}
		}
	}

	return d, nil
}

// View renders the dialog
func (d *ModelSelectDialog) View() string {
	if !d.isOpen {
		return ""
	}

	// Build model list
	var items []string
	for i, model := range d.models {
		var item string
		
		// Selection indicator
		if i == d.selectedIndex {
			item = d.selectedStyle.Render("▶ ")
		} else {
			item = d.itemStyle.Render("  ")
		}
		
		// Model name
		item += model.ID
		
		// Model size badge
		sizeStr := string(model.Size)
		if sizeStr == "" {
			sizeStr = "?"
		}
		item += " " + d.sizeStyle.Render(fmt.Sprintf("[%s]", sizeStr))
		
		// Model description if available
		if model.Name != "" && model.Name != model.ID {
			item += "\n" + d.itemStyle.Render("  ") + d.descStyle.Render(model.Name)
		}
		
		items = append(items, item)
	}
	
	// Add instructions
	instructions := d.descStyle.Render("\n\n↑/↓ Navigate • Enter Select • Esc Cancel")
	
	content := strings.Join(items, "\n") + instructions
	
	return d.RenderDialog(content)
}