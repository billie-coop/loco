package anim

import (
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// ThinkingIndicator shows an animated "thinking" message
type ThinkingIndicator struct {
	messages []string
	current  int
	dots     int
	ticker   int
	active   bool // Track if animation should continue
}

// NewThinkingIndicator creates a new thinking indicator
func NewThinkingIndicator() *ThinkingIndicator {
	return &ThinkingIndicator{
		messages: []string{
			"🤔 Thinking",
			"💭 Pondering",
			"🧠 Processing",
			"✨ Analyzing",
			"🔮 Contemplating",
			"🎯 Focusing",
		},
		current: 0,
		dots:    0,
		ticker:  0,
	}
}

// Init starts the animation
func (t *ThinkingIndicator) Init() tea.Cmd {
	t.active = true
	return t.tick()
}

// Update handles animation ticks
func (t *ThinkingIndicator) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case thinkingTickMsg:
		if msg.id == t {
			t.ticker++
			
			// Update dots every 3 ticks
			if t.ticker%3 == 0 {
				t.dots = (t.dots + 1) % 4
			}
			
			// Change message every 15 ticks
			if t.ticker%15 == 0 {
				t.current = (t.current + 1) % len(t.messages)
			}
			
			// Only continue ticking if active
			if t.active {
				return t, t.tick()
			}
		}
	}
	return t, nil
}

// View renders the thinking indicator
func (t *ThinkingIndicator) View() string {
	theme := styles.CurrentTheme()
	
	// Get current message
	message := t.messages[t.current]
	
	// Add animated dots
	dots := strings.Repeat(".", t.dots)
	padding := strings.Repeat(" ", 3-t.dots)
	
	// Create gradient text for the message
	gradientMsg := styles.RenderThemeGradient(message, true)
	
	// Style the dots with subtle color
	dotsStyle := lipgloss.NewStyle().Foreground(theme.FgMuted)
	
	return gradientMsg + dotsStyle.Render(dots) + padding
}

// tick creates a command to advance the animation
func (t *ThinkingIndicator) tick() tea.Cmd {
	// Use 300ms for smoother performance
	return tea.Tick(300*time.Millisecond, func(time time.Time) tea.Msg {
		return thinkingTickMsg{id: t}
	})
}

// thinkingTickMsg is sent to advance the animation
type thinkingTickMsg struct {
	id *ThinkingIndicator
}

// Stop stops the animation
func (t *ThinkingIndicator) Stop() {
	t.active = false
}

// Start restarts the animation
func (t *ThinkingIndicator) Start() tea.Cmd {
	t.active = true
	return t.tick()
}