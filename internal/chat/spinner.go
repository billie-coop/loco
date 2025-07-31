package chat

import (
	"time"

	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/lipgloss/v2"
)

// Alternative animated spinner that we actually use.
var locoDots = spinner.Spinner{
	Frames: []string{
		"⠋ Thinking",
		"⠙ Thinking.",
		"⠹ Thinking..",
		"⠸ Thinking...",
		"⠼ Thinking....",
		"⠴ Thinking.....",
		"⠦ Thinking......",
		"⠧ Thinking.....",
		"⠇ Thinking....",
		"⠏ Thinking...",
		"⠏ Thinking..",
		"⠏ Thinking.",
	},
	FPS: time.Second / 10,
}

// Helper to create a styled spinner.
func newStyledSpinner() spinner.Model {
	s := spinner.New()
	// Use the locomotive dots spinner
	s.Spinner = locoDots
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return s
}
