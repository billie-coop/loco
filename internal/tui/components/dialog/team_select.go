package dialog

import (
	"fmt"
	"strings"

	"github.com/billie-coop/loco/internal/session"
	"github.com/billie-coop/loco/internal/tui/events"
	"github.com/billie-coop/loco/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// TeamSelectDialog allows selecting a model team
type TeamSelectDialog struct {
	*BaseDialog

	teams           []*session.ModelTeam
	selectedIndex   int
	eventBroker     *events.Broker
	
	// Styling
	itemStyle       lipgloss.Style
	selectedStyle   lipgloss.Style
	tierStyle       lipgloss.Style
	modelStyle      lipgloss.Style
}

// NewTeamSelectDialog creates a new team selection dialog
func NewTeamSelectDialog(eventBroker *events.Broker) *TeamSelectDialog {
	theme := styles.CurrentTheme()
	
	d := &TeamSelectDialog{
		BaseDialog:     NewBaseDialog("Select Model Team"),
		teams:          []*session.ModelTeam{},
		selectedIndex:  0,
		eventBroker:    eventBroker,

		itemStyle: lipgloss.NewStyle().
			PaddingLeft(2),

		selectedStyle: lipgloss.NewStyle().
			PaddingLeft(1).
			Foreground(theme.Accent).
			Bold(true),

		tierStyle: lipgloss.NewStyle().
			Foreground(theme.Primary).
			Bold(true),

		modelStyle: lipgloss.NewStyle().
			Foreground(theme.FgMuted),
	}
	return d
}

// SetTeams sets the available teams
func (d *TeamSelectDialog) SetTeams(teams []*session.ModelTeam) {
	d.teams = teams
	// Try to maintain selection
	if d.selectedIndex >= len(d.teams) {
		d.selectedIndex = 0
	}
}

// Init initializes the dialog
func (d *TeamSelectDialog) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (d *TeamSelectDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !d.isOpen {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return d, d.HandleEscape()
		case "q":
			if d.isOpen {
				return d, d.Cancel()
			}
		case "up", "k":
			if d.selectedIndex > 0 {
				d.selectedIndex--
			}
		case "down", "j":
			if d.selectedIndex < len(d.teams)-1 {
				d.selectedIndex++
			}
		case "home", "g":
			d.selectedIndex = 0
		case "end", "G":
			d.selectedIndex = len(d.teams) - 1
		case "enter":
			if d.selectedIndex < len(d.teams) {
				selected := d.teams[d.selectedIndex]
				d.SetResult(selected)
				
				// Publish team selected event
				if d.eventBroker != nil {
					d.eventBroker.PublishAsync(events.Event{
						Type: events.TeamSelectedEvent,
						Payload: events.TeamSelectedPayload{
							Team: selected,
						},
					})
				}
				
				return d, d.Close()
			}
		}
	}

	return d, nil
}

// View renders the dialog
func (d *TeamSelectDialog) View() string {
	if !d.isOpen {
		return ""
	}

	// Build team list
	var items []string
	for i, team := range d.teams {
		var item string
		
		// Selection indicator
		if i == d.selectedIndex {
			item = d.selectedStyle.Render("▶ ")
		} else {
			item = d.itemStyle.Render("  ")
		}
		
		// Team name
		item += team.Name + "\n"
		
		// Team models
		tiers := []struct {
			name  string
			model string
		}{
			{"Small", team.Small},
			{"Medium", team.Medium},
			{"Large", team.Large},
		}
		
		for _, tier := range tiers {
			if tier.model != "" {
				tierLine := d.itemStyle.Render("  ") + 
					d.tierStyle.Render(fmt.Sprintf("%-7s", tier.name+":")) + " " +
					d.modelStyle.Render(tier.model)
				item += tierLine + "\n"
			}
		}
		
		items = append(items, strings.TrimRight(item, "\n"))
	}
	
	// Add instructions
	instructions := d.modelStyle.Render("\n\n↑/↓ Navigate • Enter Select • Esc Cancel")
	
	content := strings.Join(items, "\n\n") + instructions
	
	return d.RenderDialog(content)
}