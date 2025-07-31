// Package main is the entry point for the loco application.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// Style definitions for the UI.
var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		MarginTop(1).
		MarginBottom(1)

	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
)

// Model represents our application state
type model struct {
	messages []string
	input    string
	width    int
	height   int
}

// Initial model
func initialModel() model {
	return model{
		messages: []string{"🚂 Welcome to Loco!"},
	}
}

// Init returns an initial command
func (m model) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

// View renders the UI
func (m model) View() tea.View {
	s := titleStyle.Render("Loco - Local Coding Companion")
	s += "\n\n"
	
	for _, msg := range m.messages {
		s += msg + "\n"
	}
	
	s += "\n\n" + helpStyle.Render("Press 'q' to quit")
	
	return tea.NewView(s)
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}