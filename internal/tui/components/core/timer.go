package core

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
)

// TickMsg is sent periodically to update time-based displays.
type TickMsg struct {
	Time    time.Time
	Elapsed time.Duration
	ID      string // Optional ID to distinguish multiple timers
}

// Timer provides reusable timer functionality for TUI components.
type Timer struct {
	id           string
	startTime    time.Time
	isRunning    bool
	tickInterval time.Duration
	elapsed      time.Duration // For paused timers
}

// NewTimer creates a new timer with the specified tick interval.
func NewTimer(id string, interval time.Duration) *Timer {
	if interval <= 0 {
		interval = 100 * time.Millisecond // Default to 100ms
	}
	return &Timer{
		id:           id,
		tickInterval: interval,
	}
}

// Start begins the timer.
func (t *Timer) Start() tea.Cmd {
	t.startTime = time.Now()
	t.isRunning = true
	t.elapsed = 0
	return t.tick()
}

// Stop halts the timer and preserves elapsed time.
func (t *Timer) Stop() {
	if t.isRunning {
		t.elapsed = time.Since(t.startTime)
		t.isRunning = false
	}
}

// Reset clears the timer.
func (t *Timer) Reset() {
	t.startTime = time.Time{}
	t.isRunning = false
	t.elapsed = 0
}

// IsRunning returns whether the timer is currently running.
func (t *Timer) IsRunning() bool {
	return t.isRunning
}

// Elapsed returns the elapsed duration.
func (t *Timer) Elapsed() time.Duration {
	if t.isRunning {
		return time.Since(t.startTime)
	}
	return t.elapsed
}

// Update handles tick messages and continues the timer.
func (t *Timer) Update(msg tea.Msg) tea.Cmd {
	if tick, ok := msg.(TickMsg); ok && tick.ID == t.id {
		if t.isRunning {
			// Continue ticking
			return t.tick()
		}
	}
	return nil
}

// tick returns a command that sends a tick message after the interval.
func (t *Timer) tick() tea.Cmd {
	return tea.Tick(t.tickInterval, func(tm time.Time) tea.Msg {
		return TickMsg{
			Time:    tm,
			Elapsed: t.Elapsed(),
			ID:      t.id,
		}
	})
}

// Common format functions for timers.

// FormatSeconds formats duration as "1.2s".
func FormatSeconds(d time.Duration) string {
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// FormatMinutesSeconds formats duration as "1m 23s".
func FormatMinutesSeconds(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// FormatHMS formats duration as "01:23:45".
func FormatHMS(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}