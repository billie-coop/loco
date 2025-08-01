package teamselect

import (
	"fmt"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/session"
	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// Stage represents which model size we're selecting.
type Stage int

const (
	StageLarge Stage = iota
	StageMedium
	StageSmall
	StageDone
)

// TeamSelectedMsg is sent when team selection is complete.
type TeamSelectedMsg struct {
	Team *session.ModelTeam
}

// Model is the team selection model.
type Model struct {
	list   list.Model
	team   *session.ModelTeam
	models map[string][]llm.Model
	width  int
	height int
	stage  Stage
}

// New creates a new team selection model.
func New(models []llm.Model) Model {
	// Group models by size
	grouped := make(map[string][]llm.Model)
	for _, model := range models {
		size := llm.DetectModelSize(model.ID)
		grouped[string(size)] = append(grouped[string(size)], model)
	}

	// Create initial list for large models
	items := []list.Item{}
	for _, model := range grouped["L"] {
		items = append(items, modelItem{model: model})
	}
	if len(items) == 0 {
		// Fallback if no large models
		for _, model := range grouped["XL"] {
			items = append(items, modelItem{model: model})
		}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select your LARGE model (for complex coding tasks)"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)
	l.KeyMap.Quit.SetEnabled(false) // Can't quit during selection

	return Model{
		list:   l,
		team:   &session.ModelTeam{},
		models: grouped,
		stage:  StageLarge,
	}
}

// Init initializes the model.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 3)
		return m, nil

	case tea.KeyMsg:
		switch {
		case msg.String() == "enter":
			// Select the current model
			if selected, ok := m.list.SelectedItem().(modelItem); ok {
				switch m.stage {
				case StageLarge:
					m.team.Large = selected.model.ID
					m.stage = StageMedium
					m.updateListForStage()
				case StageMedium:
					m.team.Medium = selected.model.ID
					m.stage = StageSmall
					m.updateListForStage()
				case StageSmall:
					m.team.Small = selected.model.ID
					m.stage = StageDone
					return m, func() tea.Msg {
						return TeamSelectedMsg{Team: m.team}
					}
				case StageDone:
					// Already done, return
					return m, nil
				}
			}
			return m, nil
		}
	}

	// Update the list
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// updateListForStage updates the list items based on current stage.
func (m *Model) updateListForStage() {
	items := []list.Item{}

	switch m.stage {
	case StageLarge:
		m.list.Title = "Select your LARGE model (for complex coding tasks)"
		for _, model := range m.models["L"] {
			items = append(items, modelItem{model: model})
		}
		if len(items) == 0 {
			// Fallback if no large models
			for _, model := range m.models["XL"] {
				items = append(items, modelItem{model: model})
			}
		}
	case StageMedium:
		m.list.Title = "Select your MEDIUM model (for general tasks)"
		for _, model := range m.models["M"] {
			items = append(items, modelItem{model: model})
		}
		if len(items) == 0 {
			// Fallback to large if no medium
			for _, model := range m.models["L"] {
				items = append(items, modelItem{model: model})
			}
		}
	case StageSmall:
		m.list.Title = "Select your SMALL model (for quick responses)"
		for _, model := range m.models["S"] {
			items = append(items, modelItem{model: model})
		}
		if len(items) == 0 {
			// Try XS models
			for _, model := range m.models["XS"] {
				items = append(items, modelItem{model: model})
			}
		}
		if len(items) == 0 {
			// Fallback to medium
			for _, model := range m.models["M"] {
				items = append(items, modelItem{model: model})
			}
		}
	case StageDone:
		// Nothing to do
		return
	}

	m.list.SetItems(items)
	m.list.Select(0)
}

// View renders the UI.
func (m *Model) View() tea.View {
	if m.stage == StageDone {
		return tea.NewView("Team selection complete!")
	}

	// Show current selections
	var status string
	if m.team.Large != "" {
		status += fmt.Sprintf("✓ Large: %s\n", m.team.Large)
	}
	if m.team.Medium != "" {
		status += fmt.Sprintf("✓ Medium: %s\n", m.team.Medium)
	}

	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginBottom(1)

	return tea.NewView(style.Render(status) + m.list.View())
}

// modelItem implements list.Item.
type modelItem struct {
	model llm.Model
}

func (i modelItem) Title() string {
	return i.model.ID
}

func (i modelItem) Description() string {
	size := llm.DetectModelSize(i.model.ID)
	return fmt.Sprintf("Size: %s", size)
}

func (i modelItem) FilterValue() string {
	return i.model.ID
}
