package modelselect

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/billie-coop/loco/internal/llm"
)

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		MarginBottom(2)

	selectedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	normalStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	dimStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))
)

// ModelSelectedMsg is sent when a model is selected
type ModelSelectedMsg struct {
	Model llm.Model
}

// Model represents the model selector
type Model struct {
	models   []llm.Model
	cursor   int
	client   *llm.LMStudioClient
	err      error
	loading  bool
	width    int
	height   int
}

// New creates a new model selector
func New(client *llm.LMStudioClient) Model {
	return Model{
		client:  client,
		loading: true,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return m.fetchModels
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.models)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.models) > 0 {
				return m, func() tea.Msg {
					return ModelSelectedMsg{Model: m.models[m.cursor]}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case modelsLoadedMsg:
		m.loading = false
		m.models = msg.models
		if len(m.models) == 0 {
			m.err = fmt.Errorf("no models available in LM Studio")
		}

	case errorMsg:
		m.loading = false
		m.err = msg.err
	}

	return m, nil
}

// View renders the UI
func (m Model) View() tea.View {
	var s strings.Builder

	s.WriteString(titleStyle.Render("🚂 Select a Model"))
	s.WriteString("\n\n")

	if m.loading {
		s.WriteString(dimStyle.Render("Loading models from LM Studio..."))
		return tea.NewView(s.String())
	}

	if m.err != nil {
		s.WriteString(errorStyle.Render("❌ Error: "))
		s.WriteString(m.err.Error())
		s.WriteString("\n\n")
		s.WriteString(dimStyle.Render("Make sure LM Studio is running with at least one model loaded."))
		s.WriteString("\n")
		s.WriteString(dimStyle.Render("Press Ctrl+C to exit."))
		return tea.NewView(s.String())
	}

	if len(m.models) == 0 {
		s.WriteString(dimStyle.Render("No models found."))
		s.WriteString("\n\n")
		s.WriteString(dimStyle.Render("Please load a model in LM Studio first."))
		return tea.NewView(s.String())
	}

	// Render model list
	for i, model := range m.models {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
			s.WriteString(selectedStyle.Render(cursor + model.ID))
		} else {
			s.WriteString(normalStyle.Render(cursor + model.ID))
		}
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(dimStyle.Render("↑/↓ or j/k to navigate • Enter to select • Ctrl+C to quit"))

	return tea.NewView(s.String())
}

type modelsLoadedMsg struct {
	models []llm.Model
}

type errorMsg struct {
	err error
}

func (m Model) fetchModels() tea.Msg {
	models, err := m.client.GetModels()
	if err != nil {
		return errorMsg{err: err}
	}
	return modelsLoadedMsg{models: models}
}