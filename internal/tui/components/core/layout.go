package core

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
)

// SimpleLayout is a basic vertical layout manager
type SimpleLayout struct {
	components map[string]Component
	order      []string
	width      int
	height     int
}

// NewSimpleLayout creates a new simple vertical layout
func NewSimpleLayout() *SimpleLayout {
	return &SimpleLayout{
		components: make(map[string]Component),
		order:      make([]string, 0),
	}
}

// AddComponent adds a component to the layout
func (l *SimpleLayout) AddComponent(id string, component Component) {
	l.components[id] = component
	l.order = append(l.order, id)
	
	// If we have size information, pass it to sizeable components
	if sizeable, ok := component.(Sizeable); ok && l.width > 0 {
		sizeable.SetSize(l.width, 1) // Default to 1 line height for now
	}
}

// RemoveComponent removes a component from the layout
func (l *SimpleLayout) RemoveComponent(id string) {
	delete(l.components, id)
	
	// Remove from order slice
	for i, oid := range l.order {
		if oid == id {
			l.order = append(l.order[:i], l.order[i+1:]...)
			break
		}
	}
}

// GetComponent returns a component by ID
func (l *SimpleLayout) GetComponent(id string) Component {
	return l.components[id]
}

// SetSize implements Sizeable interface
func (l *SimpleLayout) SetSize(width, height int) tea.Cmd {
	l.width = width
	l.height = height
	
	var cmds []tea.Cmd
	
	// Update size for all sizeable components
	for _, component := range l.components {
		if sizeable, ok := component.(Sizeable); ok {
			cmds = append(cmds, sizeable.SetSize(width, 1))
		}
	}
	
	return tea.Batch(cmds...)
}

// Init implements Component interface
func (l *SimpleLayout) Init() tea.Cmd {
	var cmds []tea.Cmd
	
	for _, component := range l.components {
		cmds = append(cmds, component.Init())
	}
	
	return tea.Batch(cmds...)
}

// Update implements Component interface
func (l *SimpleLayout) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	// Update all components
	for id, component := range l.components {
		updated, cmd := component.Update(msg)
		l.components[id] = updated.(Component)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	
	return l, tea.Batch(cmds...)
}

// View implements Component interface
func (l *SimpleLayout) View() string {
	var views []string
	
	// Render components in order
	for _, id := range l.order {
		if component, exists := l.components[id]; exists {
			views = append(views, component.View())
		}
	}
	
	return strings.Join(views, "\n")
}