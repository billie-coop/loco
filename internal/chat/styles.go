package chat

import "github.com/charmbracelet/lipgloss/v2"

var (
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86"))

	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)

	metaStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("239")).
			Italic(true)
)
