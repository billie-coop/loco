package core

import tea "github.com/charmbracelet/bubbletea/v2"

// SizeableBase provides basic size management
type SizeableBase struct {
	Width  int
	Height int
}

// SetSize sets the component size
func (s *SizeableBase) SetSize(width, height int) tea.Cmd {
	s.Width = width
	s.Height = height
	return nil
}

// GetSize returns the component size
func (s *SizeableBase) GetSize() (width, height int) {
	return s.Width, s.Height
}