package chat

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
)

// getMarkdownRenderer returns a glamour renderer configured for the given width
func getMarkdownRenderer(width int) *glamour.TermRenderer {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(getMarkdownStyle()),
		glamour.WithWordWrap(width),
	)
	return r
}

// renderMarkdown converts markdown text to styled terminal output with proper wrapping
func renderMarkdown(content string, width int) string {
	r := getMarkdownRenderer(width)
	rendered, _ := r.Render(content)
	return strings.TrimSuffix(rendered, "\n")
}

// getMarkdownStyle returns a simple markdown style configuration
func getMarkdownStyle() ansi.StyleConfig {
	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#FAFAFA"),
			},
		},
		Paragraph: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{},
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           stringPtr("#FF79C6"),
				BackgroundColor: stringPtr("#282A36"),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: stringPtr("#F8F8F2"),
				},
				Margin: uintPtr(1),
			},
		},
		Emph: ansi.StylePrimitive{
			Italic: boolPtr(true),
		},
		Strong: ansi.StylePrimitive{
			Bold: boolPtr(true),
		},
		Link: ansi.StylePrimitive{
			Color:     stringPtr("#8BE9FD"),
			Underline: boolPtr(true),
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:  boolPtr(true),
				Color: stringPtr("#FF79C6"),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:  boolPtr(true),
				Color: stringPtr("#BD93F9"),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:  boolPtr(true),
				Color: stringPtr("#50FA7B"),
			},
		},
	}
}

// Helper functions for creating pointers
func boolPtr(b bool) *bool       { return &b }
func stringPtr(s string) *string { return &s }
func uintPtr(u uint) *uint       { return &u }