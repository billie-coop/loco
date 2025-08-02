package completions

import (
	"strings"

	"github.com/billie-coop/loco/internal/tui/components/core"
	"github.com/billie-coop/loco/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

const maxCompletionsHeight = 8

// Command represents a slash command with description
type Command struct {
	Name        string
	Description string
}

// Messages for completion handling
type (
	OpenCompletionsMsg struct {
		Commands []Command
		X        int // X position for the completions popup
		Y        int // Y position for the completions popup
	}
	
	FilterCompletionsMsg struct {
		Query string
	}
	
	SelectCompletionMsg struct {
		Value any // The value of the selected completion item (typically string)
	}
	
	CloseCompletionsMsg struct{}
)

// CompletionsModel manages the completions popup
type CompletionsModel struct {
	width  int
	height int
	x      int
	y      int
	open   bool
	
	commands         []Command
	filteredCommands []Command
	selectedIndex    int
	query            string
}

// Ensure CompletionsModel implements required interfaces
var _ core.Component = (*CompletionsModel)(nil)

// NewCompletions creates a new completions component
func NewCompletions() *CompletionsModel {
	return &CompletionsModel{
		commands:         []Command{},
		filteredCommands: []Command{},
		selectedIndex:    0,
	}
}

// Init initializes the completions component
func (c *CompletionsModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the completions component
func (c *CompletionsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case OpenCompletionsMsg:
		c.open = true
		c.x = msg.X
		c.y = msg.Y
		c.commands = msg.Commands
		c.filteredCommands = msg.Commands
		c.selectedIndex = 0
		c.query = ""
		c.updateDimensions()
		return c, nil
		
	case FilterCompletionsMsg:
		c.query = msg.Query
		c.filterCommands()
		c.updateDimensions()
		if len(c.filteredCommands) == 0 {
			c.open = false
		}
		return c, nil
		
	case CloseCompletionsMsg:
		c.open = false
		c.filteredCommands = []Command{}
		c.selectedIndex = 0
		return c, nil
		
	case tea.KeyPressMsg:
		if !c.open {
			return c, nil
		}
		
		switch msg.String() {
		case "up", "shift+tab":
			if c.selectedIndex > 0 {
				c.selectedIndex--
			} else {
				c.selectedIndex = len(c.filteredCommands) - 1
			}
			return c, nil
			
		case "down":
			if c.selectedIndex < len(c.filteredCommands)-1 {
				c.selectedIndex++
			} else {
				c.selectedIndex = 0
			}
			return c, nil
			
		case "tab":
			// Tab should complete the selected item
			if c.selectedIndex < len(c.filteredCommands) {
				selected := c.filteredCommands[c.selectedIndex]
				c.open = false
				return c, func() tea.Msg {
					return SelectCompletionMsg{Value: selected.Name}
				}
			}
			return c, nil
			
		case "enter":
			if c.selectedIndex < len(c.filteredCommands) {
				selected := c.filteredCommands[c.selectedIndex]
				c.open = false
				return c, func() tea.Msg {
					return SelectCompletionMsg{Value: selected.Name}
				}
			}
			return c, nil
			
		case "esc":
			c.open = false
			return c, func() tea.Msg {
				return CloseCompletionsMsg{}
			}
		}
	}
	
	return c, nil
}

// View renders the completions popup
func (c *CompletionsModel) View() string {
	if !c.open || len(c.filteredCommands) == 0 {
		return ""
	}
	
	theme := styles.CurrentTheme()
	
	// Build the completion list
	var items []string
	for i, cmd := range c.filteredCommands {
		// Highlight the selected item
		itemStyle := theme.S().Subtle
		if i == c.selectedIndex {
			itemStyle = theme.S().Base.
				Background(theme.Accent).
				Foreground(theme.BgBase)
		}
		
		// Format: "/command - description"
		item := itemStyle.
			Width(c.width - 2).
			PaddingLeft(1).
			PaddingRight(1).
			Render(cmd.Name + " - " + cmd.Description)
		
		items = append(items, item)
	}
	
	// Create the popup box
	popupStyle := theme.S().Base.
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.BorderFocus).
		Width(c.width).
		Height(c.height).
		Background(theme.BgBase)
	
	content := strings.Join(items, "\n")
	return popupStyle.Render(content)
}

// Helper methods

func (c *CompletionsModel) filterCommands() {
	c.filteredCommands = []Command{}
	c.selectedIndex = 0
	
	for _, cmd := range c.commands {
		if strings.HasPrefix(strings.ToLower(cmd.Name), strings.ToLower(c.query)) {
			c.filteredCommands = append(c.filteredCommands, cmd)
		}
	}
}

func (c *CompletionsModel) updateDimensions() {
	// Calculate width based on longest command
	maxWidth := 0
	for _, cmd := range c.filteredCommands {
		width := len(cmd.Name) + 3 + len(cmd.Description) + 4 // padding + border
		if width > maxWidth {
			maxWidth = width
		}
	}
	c.width = min(maxWidth, 60) // Cap at 60 chars
	
	// Calculate height
	c.height = min(len(c.filteredCommands)+2, maxCompletionsHeight) // +2 for borders
}

// Public getters

func (c *CompletionsModel) IsOpen() bool {
	return c.open
}

func (c *CompletionsModel) Position() (int, int) {
	// Return position adjusted for popup height
	return c.x, c.y - c.height
}

func (c *CompletionsModel) Dimensions() (int, int) {
	return c.width, c.height
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}