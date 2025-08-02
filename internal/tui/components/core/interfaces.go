package core

import tea "github.com/charmbracelet/bubbletea/v2"

// Component is the base interface for all TUI components
type Component interface {
	Init() tea.Cmd
	Update(tea.Msg) (tea.Model, tea.Cmd)
	View() string
}

// Sizeable components can be resized
type Sizeable interface {
	SetSize(width, height int) tea.Cmd
}

// Focusable components can receive keyboard focus
type Focusable interface {
	Focus() tea.Cmd
	Blur() tea.Cmd
	Focused() bool
}

// Layout manages the positioning and sizing of components
type Layout interface {
	Component
	Sizeable
	AddComponent(id string, component Component)
	RemoveComponent(id string)
	GetComponent(id string) Component
}