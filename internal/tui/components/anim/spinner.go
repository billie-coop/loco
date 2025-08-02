package anim

import (
	"fmt"
	"image/color"
	"time"

	"github.com/billie-coop/loco/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// SpinnerType defines different spinner animations
type SpinnerType int

const (
	SpinnerDots SpinnerType = iota
	SpinnerLine
	SpinnerCircle
	SpinnerSquare
	SpinnerGradient
)

// Spinner is an animated loading indicator
type Spinner struct {
	Type       SpinnerType
	Label      string
	frame      int
	lastUpdate time.Time
	speed      time.Duration
	color1     color.Color
	color2     color.Color
}

// NewSpinner creates a new spinner
func NewSpinner(spinnerType SpinnerType) *Spinner {
	theme := styles.CurrentTheme()
	return &Spinner{
		Type:   spinnerType,
		speed:  80 * time.Millisecond,
		color1: theme.Primary,
		color2: theme.Secondary,
	}
}

// WithLabel sets the spinner label
func (s *Spinner) WithLabel(label string) *Spinner {
	s.Label = label
	return s
}

// WithSpeed sets the animation speed
func (s *Spinner) WithSpeed(speed time.Duration) *Spinner {
	s.speed = speed
	return s
}

// WithColors sets the gradient colors
func (s *Spinner) WithColors(c1, c2 color.Color) *Spinner {
	s.color1 = c1
	s.color2 = c2
	return s
}

// Init starts the spinner animation
func (s *Spinner) Init() tea.Cmd {
	return s.tick()
}

// Update handles spinner animation
func (s *Spinner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if msg.id == s {
			s.frame++
			s.lastUpdate = time.Now()
			return s, s.tick()
		}
	}
	return s, nil
}

// View renders the spinner
func (s *Spinner) View() string {
	frames := s.getFrames()
	if len(frames) == 0 {
		return ""
	}
	
	currentFrame := frames[s.frame%len(frames)]
	
	// Apply gradient coloring for gradient spinner
	if s.Type == SpinnerGradient {
		colors := styles.GetGradientColors(len(currentFrame))
		coloredFrame := ""
		for i, ch := range currentFrame {
			style := lipgloss.NewStyle().Foreground(colors[i%len(colors)])
			coloredFrame += style.Render(string(ch))
		}
		currentFrame = coloredFrame
	} else {
		// Use theme colors for other spinners
		theme := styles.CurrentTheme()
		currentFrame = styles.RenderThemeGradient(currentFrame, false)
	}
	
	if s.Label != "" {
		theme := styles.CurrentTheme()
		label := theme.S().Subtle.Render(s.Label)
		return fmt.Sprintf("%s %s", currentFrame, label)
	}
	
	return currentFrame
}

// getFrames returns animation frames based on spinner type
func (s *Spinner) getFrames() []string {
	switch s.Type {
	case SpinnerDots:
		return []string{
			"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
		}
	case SpinnerLine:
		return []string{
			"-", "\\", "|", "/",
		}
	case SpinnerCircle:
		return []string{
			"◐", "◓", "◑", "◒",
		}
	case SpinnerSquare:
		return []string{
			"◰", "◳", "◲", "◱",
		}
	case SpinnerGradient:
		return []string{
			"█▁▁▁▁▁▁▁", "▁█▁▁▁▁▁▁", "▁▁█▁▁▁▁▁", "▁▁▁█▁▁▁▁",
			"▁▁▁▁█▁▁▁", "▁▁▁▁▁█▁▁", "▁▁▁▁▁▁█▁", "▁▁▁▁▁▁▁█",
		}
	default:
		return []string{" "}
	}
}

// tick creates a command to advance the animation
func (s *Spinner) tick() tea.Cmd {
	return tea.Tick(s.speed, func(t time.Time) tea.Msg {
		return tickMsg{id: s, time: t}
	})
}

// tickMsg is sent to advance the animation
type tickMsg struct {
	id   *Spinner
	time time.Time
}

// GradientBar creates an animated gradient progress bar
type GradientBar struct {
	Width    int
	Progress float64
	offset   int
	speed    time.Duration
}

// NewGradientBar creates a new gradient progress bar
func NewGradientBar(width int) *GradientBar {
	return &GradientBar{
		Width: width,
		speed: 100 * time.Millisecond,
	}
}

// SetProgress updates the progress (0.0 to 1.0)
func (g *GradientBar) SetProgress(progress float64) {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	g.Progress = progress
}

// Init starts the gradient animation
func (g *GradientBar) Init() tea.Cmd {
	return g.tick()
}

// Update handles gradient animation
func (g *GradientBar) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case gradientTickMsg:
		if msg.id == g {
			g.offset++
			return g, g.tick()
		}
	}
	return g, nil
}

// View renders the gradient bar
func (g *GradientBar) View() string {
	theme := styles.CurrentTheme()
	filled := int(float64(g.Width) * g.Progress)
	
	if filled <= 0 {
		return lipgloss.NewStyle().Width(g.Width).Render("")
	}
	
	// Create animated gradient
	colors := styles.GetGradientColors(g.Width * 2)
	bar := ""
	
	for i := 0; i < g.Width; i++ {
		if i < filled {
			colorIdx := (i + g.offset) % len(colors)
			style := lipgloss.NewStyle().Foreground(colors[colorIdx])
			bar += style.Render("█")
		} else {
			bar += theme.S().Subtle.Render("░")
		}
	}
	
	return bar
}

// tick creates a command to advance the gradient animation
func (g *GradientBar) tick() tea.Cmd {
	return tea.Tick(g.speed, func(t time.Time) tea.Msg {
		return gradientTickMsg{id: g, time: t}
	})
}

// gradientTickMsg is sent to advance the gradient animation
type gradientTickMsg struct {
	id   *GradientBar
	time time.Time
}