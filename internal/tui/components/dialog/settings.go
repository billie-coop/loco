package dialog

import (
	"strings"

	"github.com/billie-coop/loco/internal/tui/events"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// SettingsDialog allows configuring application settings
type SettingsDialog struct {
	*BaseDialog

	fields          []settingField
	selectedIndex   int
	eventBroker     *events.Broker
	
	// Styling
	labelStyle      lipgloss.Style
	valueStyle      lipgloss.Style
	selectedStyle   lipgloss.Style
	descStyle       lipgloss.Style
}

type settingField struct {
	label       string
	description string
	fieldType   string // "text", "bool", "select"
	value       interface{}
	options     []string // for select fields
	input       *SimpleTextInput // for text fields
	editing     bool
}

// Settings represents the application settings
type Settings struct {
	APIEndpoint     string
	EnableTelemetry bool
	DebugMode       bool
	AutoSave        bool
	Theme           string
}

// NewSettingsDialog creates a new settings dialog
func NewSettingsDialog(eventBroker *events.Broker) *SettingsDialog {
	d := &SettingsDialog{
		BaseDialog:     NewBaseDialog("Settings"),
		selectedIndex:  0,
		eventBroker:    eventBroker,

		labelStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")),

		valueStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),

		selectedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true),

		descStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true),
	}

	// Initialize fields
	d.initFields()
	
	return d
}

func (d *SettingsDialog) initFields() {
	apiInput := NewSimpleTextInput()
	apiInput.Placeholder("http://localhost:1234")
	apiInput.SetValue("http://localhost:1234")

	d.fields = []settingField{
		{
			label:       "API Endpoint",
			description: "LM Studio API endpoint",
			fieldType:   "text",
			value:       "http://localhost:1234",
			input:       apiInput,
		},
		{
			label:       "Enable Telemetry",
			description: "Help improve Loco by sending anonymous usage data",
			fieldType:   "bool",
			value:       false,
		},
		{
			label:       "Debug Mode",
			description: "Show detailed debug information",
			fieldType:   "bool",
			value:       false,
		},
		{
			label:       "Auto Save",
			description: "Automatically save sessions",
			fieldType:   "bool",
			value:       true,
		},
		{
			label:       "Theme",
			description: "Color theme for the interface",
			fieldType:   "select",
			value:       "default",
			options:     []string{"default", "dark", "light", "solarized"},
		},
	}
}

// Init initializes the dialog
func (d *SettingsDialog) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (d *SettingsDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !d.isOpen {
		return d, nil
	}

	var cmds []tea.Cmd

	// Handle text input updates
	if d.selectedIndex < len(d.fields) && d.fields[d.selectedIndex].editing {
		field := &d.fields[d.selectedIndex]
		if field.fieldType == "text" {
			cmd := field.input.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If editing a text field, handle special keys
		if d.selectedIndex < len(d.fields) && d.fields[d.selectedIndex].editing {
			field := &d.fields[d.selectedIndex]
			switch msg.String() {
			case "enter":
				field.value = field.input.Value()
				field.editing = false
				field.input.Blur()
			case "esc":
				field.editing = false
				field.input.Blur()
				field.input.SetValue(field.value.(string))
			default:
				// Let text input handle other keys
				return d, tea.Batch(cmds...)
			}
		} else {
			// Normal navigation
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
				if d.selectedIndex < len(d.fields)-1 {
					d.selectedIndex++
				}
			case "enter", " ":
				if d.selectedIndex < len(d.fields) {
					field := &d.fields[d.selectedIndex]
					switch field.fieldType {
					case "text":
						field.editing = true
						field.input.Focus()
					case "bool":
						field.value = !field.value.(bool)
					case "select":
						// Cycle through options
						currentVal := field.value.(string)
						currentIdx := 0
						for i, opt := range field.options {
							if opt == currentVal {
								currentIdx = i
								break
							}
						}
						nextIdx := (currentIdx + 1) % len(field.options)
						field.value = field.options[nextIdx]
					}
				}
			case "s":
				// Save settings
				settings := d.getSettings()
				d.SetResult(settings)
				
				// Publish settings updated event
				if d.eventBroker != nil {
					d.eventBroker.PublishAsync(events.Event{
						Type: events.StatusMessageEvent,
						Payload: events.StatusMessagePayload{
							Message: "Settings saved",
							Type:    "success",
						},
					})
				}
				
				return d, d.Close()
			}
		}
	}

	return d, tea.Batch(cmds...)
}

// View renders the dialog
func (d *SettingsDialog) View() string {
	if !d.isOpen {
		return ""
	}

	// Build settings list
	var items []string
	for i, field := range d.fields {
		var item string
		
		// Selection indicator
		if i == d.selectedIndex {
			item = d.selectedStyle.Render("▶ ")
		} else {
			item = "  "
		}
		
		// Label
		item += d.labelStyle.Render(field.label) + ": "
		
		// Value
		switch field.fieldType {
		case "text":
			if field.editing {
				item += field.input.View()
			} else {
				item += d.valueStyle.Render(field.value.(string))
			}
		case "bool":
			if field.value.(bool) {
				item += d.valueStyle.Render("✓ Enabled")
			} else {
				item += d.valueStyle.Render("✗ Disabled")
			}
		case "select":
			item += d.valueStyle.Render(field.value.(string))
		}
		
		// Description
		if field.description != "" {
			item += "\n  " + d.descStyle.Render(field.description)
		}
		
		items = append(items, item)
	}
	
	// Add instructions
	instructions := d.descStyle.Render("\n\n↑/↓ Navigate • Enter/Space Toggle • S Save • Esc Cancel")
	
	content := strings.Join(items, "\n\n") + instructions
	
	return d.RenderDialog(content)
}

func (d *SettingsDialog) getSettings() *Settings {
	return &Settings{
		APIEndpoint:     d.fields[0].value.(string),
		EnableTelemetry: d.fields[1].value.(bool),
		DebugMode:       d.fields[2].value.(bool),
		AutoSave:        d.fields[3].value.(bool),
		Theme:           d.fields[4].value.(string),
	}
}

// SetSettings updates the dialog with current settings
func (d *SettingsDialog) SetSettings(settings *Settings) {
	if settings == nil {
		return
	}
	
	d.fields[0].value = settings.APIEndpoint
	d.fields[0].input.SetValue(settings.APIEndpoint)
	d.fields[1].value = settings.EnableTelemetry
	d.fields[2].value = settings.DebugMode
	d.fields[3].value = settings.AutoSave
	d.fields[4].value = settings.Theme
}