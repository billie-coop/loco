package chat

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/lipgloss/v2"
)

// Custom spinner frames that look like a train!
var trainSpinner = spinner.Spinner{
	Frames: []string{
		"🚂      ",
		" 🚂     ",
		"  🚂    ",
		"   🚂   ",
		"    🚂  ",
		"     🚂 ",
		"      🚂",
		"     🚂 ",
		"    🚂  ",
		"   🚂   ",
		"  🚂    ",
		" 🚂     ",
	},
	FPS: time.Second / 10,
}

// Alternative animated spinners
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

var locoGears = spinner.Spinner{
	Frames: []string{
		"⚙️  Chugging along",
		"⚙️  Chugging along.",
		"⚙️  Chugging along..",
		"⚙️  Chugging along...",
		"⚙️  Chugging along..",
		"⚙️  Chugging along.",
	},
	FPS: time.Second / 5,
}

// Animated loading bar spinner
var locoBar = spinner.Spinner{
	Frames: []string{
		"[▱▱▱▱▱▱▱▱▱▱]",
		"[▰▱▱▱▱▱▱▱▱▱]",
		"[▰▰▱▱▱▱▱▱▱▱]",
		"[▰▰▰▱▱▱▱▱▱▱]",
		"[▰▰▰▰▱▱▱▱▱▱]",
		"[▰▰▰▰▰▱▱▱▱▱]",
		"[▰▰▰▰▰▰▱▱▱▱]",
		"[▰▰▰▰▰▰▰▱▱▱]",
		"[▰▰▰▰▰▰▰▰▱▱]",
		"[▰▰▰▰▰▰▰▰▰▱]",
		"[▰▰▰▰▰▰▰▰▰▰]",
		"[▱▰▰▰▰▰▰▰▰▰]",
		"[▱▱▰▰▰▰▰▰▰▰]",
		"[▱▱▱▰▰▰▰▰▰▰]",
		"[▱▱▱▱▰▰▰▰▰▰]",
		"[▱▱▱▱▱▰▰▰▰▰]",
		"[▱▱▱▱▱▱▰▰▰▰]",
		"[▱▱▱▱▱▱▱▰▰▰]",
		"[▱▱▱▱▱▱▱▱▰▰]",
		"[▱▱▱▱▱▱▱▱▱▰]",
	},
	FPS: time.Second / 15,
}

// A fun rainbow spinner
type rainbowSpinner struct {
	frames []string
	colors []string
	step   int
}

func newRainbowSpinner() *rainbowSpinner {
	frames := []string{"●", "●", "●", "●", "●", "●", "●", "●"}
	colors := []string{
		"196", // Red
		"208", // Orange  
		"226", // Yellow
		"46",  // Green
		"21",  // Blue
		"93",  // Purple
		"201", // Pink
		"205", // Magenta
	}
	return &rainbowSpinner{
		frames: frames,
		colors: colors,
	}
}

func (r *rainbowSpinner) View() string {
	var parts []string
	for i := 0; i < len(r.frames); i++ {
		colorIdx := (r.step + i) % len(r.colors)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(r.colors[colorIdx]))
		parts = append(parts, style.Render(r.frames[i]))
	}
	return strings.Join(parts, " ") + " Thinking..."
}

func (r *rainbowSpinner) tick() {
	r.step = (r.step + 1) % len(r.colors)
}

// Helper to create a styled spinner
func newStyledSpinner() spinner.Model {
	s := spinner.New()
	// Use the locomotive dots spinner
	s.Spinner = locoDots
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return s
}