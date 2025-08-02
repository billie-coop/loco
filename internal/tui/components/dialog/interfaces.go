package dialog

import (
	tea "github.com/charmbracelet/bubbletea/v2"
)

// Dialog represents a modal dialog component
type Dialog interface {
	// Core component methods
	Init() tea.Cmd
	Update(tea.Msg) (tea.Model, tea.Cmd)
	View() string

	// Dialog-specific methods
	SetSize(width, height int) tea.Cmd
	IsOpen() bool
	Open() tea.Cmd
	Close() tea.Cmd
	Focus() tea.Cmd
	Blur() tea.Cmd
	IsFocused() bool

	// Result handling
	GetResult() interface{}
	IsCancelled() bool
}

// DialogResult represents the result of a dialog
type DialogResult struct {
	Action    string      // "ok", "cancel", "select", etc.
	Value     interface{} // The selected/entered value
	Cancelled bool
}