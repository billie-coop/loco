package dialog

import (
	"fmt"
	"strings"

	"github.com/billie-coop/loco/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea/v2"
)

// ThemeSwitcherDialog allows the user to switch themes
type ThemeSwitcherDialog struct {
	*BaseDialog
	themes        []string
	selectedIndex int
	previewTheme  string
}

// NewThemeSwitcher creates a new theme switcher dialog
func NewThemeSwitcher() *ThemeSwitcherDialog {
	base := NewBaseDialog("🎨 Theme Switcher")

	manager := styles.DefaultManager()
	themes := manager.List()
	currentTheme := manager.Current().Name

	// Find current theme index
	selectedIndex := 0
	for i, theme := range themes {
		if theme == currentTheme {
			selectedIndex = i
			break
		}
	}

	return &ThemeSwitcherDialog{
		BaseDialog:    base,
		themes:        themes,
		selectedIndex: selectedIndex,
		previewTheme:  currentTheme,
	}
}

// Init initializes the dialog
func (d *ThemeSwitcherDialog) Init() tea.Cmd {
	return nil
}

// Update handles input
func (d *ThemeSwitcherDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if d.selectedIndex > 0 {
				d.selectedIndex--
				d.previewTheme = d.themes[d.selectedIndex]
				// Apply preview
				styles.DefaultManager().SetTheme(d.previewTheme)
			}
			return d, nil

		case "down", "j":
			if d.selectedIndex < len(d.themes)-1 {
				d.selectedIndex++
				d.previewTheme = d.themes[d.selectedIndex]
				// Apply preview
				styles.DefaultManager().SetTheme(d.previewTheme)
			}
			return d, nil

		case "enter":
			// Theme already applied as preview
			d.SetResult(d.themes[d.selectedIndex])
			return d, d.Close()

		case "esc", "ctrl+c":
			// Revert to original theme
			originalTheme := d.themes[0] // We should store this properly
			styles.DefaultManager().SetTheme(originalTheme)
			d.SetResult("")
			return d, d.Close()
		}
	}

	return d, nil
}

// View renders the dialog
func (d *ThemeSwitcherDialog) View() string {
	content := d.renderContent()
	return d.BaseDialog.RenderDialog(content)
}

func (d *ThemeSwitcherDialog) renderContent() string {
	theme := styles.CurrentTheme()
	var lines []string

	// Instructions
	lines = append(lines, theme.S().Subtle.Render("Select a theme with ↑/↓ arrows, Enter to apply"))
	lines = append(lines, "")

	// Theme list
	for i, themeName := range d.themes {
		line := fmt.Sprintf("  %s", themeName)
		
		if i == d.selectedIndex {
			// Highlight selected theme with gradient
			arrow := styles.RenderThemeGradient("→", false)
			name := styles.RenderThemeGradient(themeName, true)
			line = fmt.Sprintf("%s %s", arrow, name)
			
			// Show theme preview
			if themeName == d.previewTheme {
				preview := d.getThemePreview(themeName)
				lines = append(lines, line)
				lines = append(lines, "")
				lines = append(lines, theme.S().Subtle.Render("  Preview:"))
				lines = append(lines, preview)
				lines = append(lines, "")
				continue
			}
		} else {
			style := theme.S().Text
			if themeName == styles.DefaultManager().Current().Name {
				// Mark current theme
				line = fmt.Sprintf("  %s (current)", themeName)
				style = theme.S().Muted
			}
			line = style.Render(line)
		}
		
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (d *ThemeSwitcherDialog) getThemePreview(themeName string) string {
	t := styles.CurrentTheme()
	
	var preview strings.Builder
	
	// Color swatches
	preview.WriteString("    ")
	
	// Primary gradient
	preview.WriteString(styles.RenderGradientBar(10, 1.0))
	preview.WriteString(" ")
	
	// Show some styled text samples
	samples := []string{
		t.S().Success.Render("Success"),
		t.S().Warning.Render("Warning"),
		t.S().Error.Render("Error"),
		t.S().Info.Render("Info"),
	}
	
	preview.WriteString("\n    ")
	preview.WriteString(strings.Join(samples, " "))
	
	return preview.String()
}

// SetFocus sets the dialog focus state
func (d *ThemeSwitcherDialog) SetFocus(focused bool) {
	if focused {
		d.Focus()
	} else {
		d.Blur()
	}
}