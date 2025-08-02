package core

import tea "github.com/charmbracelet/bubbletea/v2"

// FocusableBase provides basic focus management
type FocusableBase struct {
	focused bool
}

// IsFocused returns whether the component is focused
func (f *FocusableBase) IsFocused() bool {
	return f.focused
}

// Focus focuses the component
func (f *FocusableBase) Focus() tea.Cmd {
	f.focused = true
	return nil
}

// Blur removes focus from the component
func (f *FocusableBase) Blur() tea.Cmd {
	f.focused = false
	return nil
}