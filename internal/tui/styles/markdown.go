package styles

import (
	"github.com/charmbracelet/glamour/v2"
)

// GetMarkdownRenderer returns a glamour TermRenderer configured with the current theme
func GetMarkdownRenderer(width int) *glamour.TermRenderer {
	t := CurrentTheme()
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(t.S().Markdown),
		glamour.WithWordWrap(width),
	)
	return r
}