package dialog

import (
	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/session"
	"github.com/billie-coop/loco/internal/tui/events"
	tea "github.com/charmbracelet/bubbletea/v2"
)

// DialogType identifies the type of dialog
type DialogType string

const (
	ModelSelectDialogType   DialogType = "model_select"
	TeamSelectDialogType    DialogType = "team_select"
	SettingsDialogType      DialogType = "settings"
	PermissionsDialogType   DialogType = "permissions"
	QuitDialogType          DialogType = "quit"
	CommandPaletteDialogType DialogType = "command_palette"
	HelpDialogType          DialogType = "help"
)

// Manager manages all dialogs in the application
type Manager struct {
	dialogs         map[DialogType]Dialog
	activeDialog    DialogType
	eventBroker     *events.Broker
	width           int
	height          int
}

// NewManager creates a new dialog manager
func NewManager(eventBroker *events.Broker) *Manager {
	m := &Manager{
		dialogs:      make(map[DialogType]Dialog),
		eventBroker:  eventBroker,
	}

	// Create all dialogs
	m.dialogs[ModelSelectDialogType] = NewModelSelectDialog(eventBroker)
	m.dialogs[TeamSelectDialogType] = NewTeamSelectDialog(eventBroker)
	m.dialogs[SettingsDialogType] = NewSettingsDialog(eventBroker)
	m.dialogs[PermissionsDialogType] = NewPermissionsDialog(eventBroker)
	m.dialogs[QuitDialogType] = NewQuitDialog(eventBroker)
	m.dialogs[CommandPaletteDialogType] = NewCommandPaletteDialog(eventBroker)
	m.dialogs[HelpDialogType] = NewHelpDialog(eventBroker)

	return m
}

// Init initializes all dialogs
func (m *Manager) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, dialog := range m.dialogs {
		cmds = append(cmds, dialog.Init())
	}
	return tea.Batch(cmds...)
}

// Update handles updates for the active dialog
func (m *Manager) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle window size changes
	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		m.SetSize(wsm.Width, wsm.Height)
	}

	// Update active dialog if any
	if m.activeDialog != "" {
		if dialog, ok := m.dialogs[m.activeDialog]; ok {
			model, cmd := dialog.Update(msg)
			if d, ok := model.(Dialog); ok {
				m.dialogs[m.activeDialog] = d
				
				// Check if dialog was closed
				if !d.IsOpen() {
					m.activeDialog = ""
					// Publish dialog close event
					m.eventBroker.PublishAsync(events.Event{
						Type: events.DialogCloseEvent,
						Payload: events.DialogPayload{
							DialogID: string(m.activeDialog),
							Data:     d.GetResult(),
						},
					})
				}
			}
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the active dialog
func (m *Manager) View() string {
	if m.activeDialog == "" {
		return ""
	}

	if dialog, ok := m.dialogs[m.activeDialog]; ok {
		return dialog.View()
	}

	return ""
}

// SetSize sets the size for all dialogs
func (m *Manager) SetSize(width, height int) tea.Cmd {
	m.width = width
	m.height = height
	
	var cmds []tea.Cmd
	for _, dialog := range m.dialogs {
		cmds = append(cmds, dialog.SetSize(width, height))
	}
	return tea.Batch(cmds...)
}

// OpenDialog opens a specific dialog
func (m *Manager) OpenDialog(dialogType DialogType) tea.Cmd {
	if dialog, ok := m.dialogs[dialogType]; ok {
		m.activeDialog = dialogType
		
		// Publish dialog open event
		m.eventBroker.PublishAsync(events.Event{
			Type: events.DialogOpenEvent,
			Payload: events.DialogPayload{
				DialogID: string(dialogType),
			},
		})
		
		return dialog.Open()
	}
	return nil
}

// CloseActiveDialog closes the currently active dialog
func (m *Manager) CloseActiveDialog() tea.Cmd {
	if m.activeDialog != "" {
		if dialog, ok := m.dialogs[m.activeDialog]; ok {
			m.activeDialog = ""
			return dialog.Close()
		}
	}
	return nil
}

// IsDialogOpen returns whether any dialog is open
func (m *Manager) IsDialogOpen() bool {
	return m.activeDialog != ""
}

// GetActiveDialog returns the currently active dialog type
func (m *Manager) GetActiveDialog() DialogType {
	return m.activeDialog
}

// SetModels sets the available models for the model selection dialog
func (m *Manager) SetModels(models []llm.Model) {
	if dialog, ok := m.dialogs[ModelSelectDialogType].(*ModelSelectDialog); ok {
		dialog.SetModels(models)
	}
}

// SetTeams sets the available teams for the team selection dialog
func (m *Manager) SetTeams(teams []*session.ModelTeam) {
	if dialog, ok := m.dialogs[TeamSelectDialogType].(*TeamSelectDialog); ok {
		dialog.SetTeams(teams)
	}
}

// SetSettings updates the settings dialog with current settings
func (m *Manager) SetSettings(settings *Settings) {
	if dialog, ok := m.dialogs[SettingsDialogType].(*SettingsDialog); ok {
		dialog.SetSettings(settings)
	}
}

// GetSettings returns the current settings from the settings dialog
func (m *Manager) GetSettings() *Settings {
	if dialog, ok := m.dialogs[SettingsDialogType].(*SettingsDialog); ok {
		if result := dialog.GetResult(); result != nil {
			if settings, ok := result.(*Settings); ok {
				return settings
			}
		}
	}
	return nil
}

// SetToolRequest sets the tool execution request for the permissions dialog
func (m *Manager) SetToolRequest(toolName string, args map[string]interface{}, requestID string) {
	if dialog, ok := m.dialogs[PermissionsDialogType].(*PermissionsDialog); ok {
		dialog.SetToolRequest(toolName, args, requestID)
	}
}