package styles

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/charmbracelet/glamour/v2/ansi"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/rivo/uniseg"
)

// Semantic color names for consistency
type Theme struct {
	Name   string
	IsDark bool

	// Brand colors
	Primary   color.Color
	Secondary color.Color
	Tertiary  color.Color
	Accent    color.Color

	// Background colors
	BgBase        color.Color
	BgBaseLighter color.Color
	BgSubtle      color.Color
	BgOverlay     color.Color
	BgHighlight   color.Color

	// Foreground colors
	FgBase      color.Color
	FgMuted     color.Color
	FgSubtle    color.Color
	FgInverted  color.Color
	
	// Additional foreground colors
	FgHalfMuted color.Color
	FgSelected  color.Color

	// Border colors
	Border      color.Color
	BorderFocus color.Color

	// Semantic colors
	Success color.Color
	Error   color.Color
	Warning color.Color
	Info    color.Color

	// Special colors
	Blue      color.Color
	BlueLight color.Color
	Green     color.Color
	Yellow    color.Color
	Purple    color.Color
	Pink      color.Color
	Orange    color.Color
	Cyan      color.Color

	styles *Styles
}

type Styles struct {
	Base     lipgloss.Style
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Text     lipgloss.Style
	Muted    lipgloss.Style
	Subtle   lipgloss.Style
	Bold     lipgloss.Style

	Success lipgloss.Style
	Error   lipgloss.Style
	Warning lipgloss.Style
	Info    lipgloss.Style

	// Component styles
	Button         lipgloss.Style
	ButtonFocused  lipgloss.Style
	Input          lipgloss.Style
	InputFocused   lipgloss.Style
	Border         lipgloss.Style
	BorderFocused  lipgloss.Style
	Badge          lipgloss.Style
	CodeBlock      lipgloss.Style
	InlineCode     lipgloss.Style

	// Markdown & Chroma
	Markdown ansi.StyleConfig
}

func (t *Theme) S() *Styles {
	if t.styles == nil {
		t.styles = t.buildStyles()
	}
	return t.styles
}

func (t *Theme) buildStyles() *Styles {
	base := lipgloss.NewStyle().
		Foreground(t.FgBase)

	return &Styles{
		Base: base,

		Title: base.
			Foreground(t.Accent).
			Bold(true),

		Subtitle: base.
			Foreground(t.Secondary).
			Bold(true),

		Text: base,

		Muted: base.Foreground(t.FgMuted),

		Subtle: base.Foreground(t.FgSubtle),

		Bold: base.Bold(true),

		Success: base.Foreground(t.Success),

		Error: base.Foreground(t.Error),

		Warning: base.Foreground(t.Warning),

		Info: base.Foreground(t.Info),

		Button: base.
			Background(t.BgSubtle).
			Foreground(t.FgBase).
			Padding(0, 2),

		ButtonFocused: base.
			Background(t.Primary).
			Foreground(t.FgInverted).
			Padding(0, 2),

		Input: base.
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(t.Border),

		InputFocused: base.
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderFocus),

		Border: base.
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(t.Border),

		BorderFocused: base.
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderFocus),

		Badge: base.
			Background(t.BgSubtle).
			Foreground(t.FgBase).
			Padding(0, 1),

		CodeBlock: base.
			Background(t.BgSubtle).
			Foreground(t.FgBase).
			Padding(1, 2),

		InlineCode: base.
			Background(t.BgSubtle).
			Foreground(t.Accent).
			Padding(0, 1),

		Markdown: t.buildMarkdownStyles(),
	}
}

// Helper functions for style pointers
func boolPtr(b bool) *bool       { return &b }
func stringPtr(s string) *string { return &s }
func uintPtr(u uint) *uint       { return &u }

func (t *Theme) buildMarkdownStyles() ansi.StyleConfig {
	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(colorToHex(t.FgBase)),
			},
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr(colorToHex(t.FgMuted)),
			},
			Indent:      uintPtr(1),
			IndentToken: stringPtr("│ "),
		},
		List: ansi.StyleList{
			LevelIndent: 2,
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       stringPtr(colorToHex(t.Secondary)),
				Bold:        boolPtr(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           stringPtr(colorToHex(t.FgInverted)),
				BackgroundColor: stringPtr(colorToHex(t.Primary)),
				Bold:            boolPtr(true),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "## ",
				Color:  stringPtr(colorToHex(t.Accent)),
				Bold:   boolPtr(true),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "### ",
				Color:  stringPtr(colorToHex(t.Secondary)),
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "#### ",
				Color:  stringPtr(colorToHex(t.Tertiary)),
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "##### ",
				Color:  stringPtr(colorToHex(t.FgBase)),
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "###### ",
				Color:  stringPtr(colorToHex(t.FgMuted)),
			},
		},
		Text: ansi.StylePrimitive{
			Color: stringPtr(colorToHex(t.FgBase)),
		},
		Paragraph: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
			},
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
		},
		Task: ansi.StyleTask{
			StylePrimitive: ansi.StylePrimitive{},
			Ticked:         "✓ ",
			Unticked:       "☐ ",
		},
		Link: ansi.StylePrimitive{
			Color:     stringPtr(colorToHex(t.BlueLight)),
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: stringPtr(colorToHex(t.BlueLight)),
		},
		Image: ansi.StylePrimitive{
			Color: stringPtr(colorToHex(t.Purple)),
		},
		ImageText: ansi.StylePrimitive{
			Color:  stringPtr(colorToHex(t.Purple)),
			Format: "Image: {{.text}} {{.url}}",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           stringPtr(colorToHex(t.Accent)),
				BackgroundColor: stringPtr(colorToHex(t.BgSubtle)),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color:           stringPtr(colorToHex(t.FgBase)),
					BackgroundColor: stringPtr(colorToHex(t.BgSubtle)),
				},
				Margin: uintPtr(2),
			},
			Chroma: &ansi.Chroma{
				Text:                ansi.StylePrimitive{Color: stringPtr(colorToHex(t.FgBase))},
				Error:               ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Error))},
				Comment:             ansi.StylePrimitive{Color: stringPtr(colorToHex(t.FgMuted)), Italic: boolPtr(true)},
				CommentPreproc:      ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Warning))},
				Keyword:             ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Primary)), Bold: boolPtr(true)},
				KeywordReserved:     ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Accent)), Bold: boolPtr(true)},
				KeywordNamespace:    ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Purple))},
				KeywordType:         ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Blue))},
				Operator:            ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Orange))},
				Punctuation:         ansi.StylePrimitive{Color: stringPtr(colorToHex(t.FgSubtle))},
				Name:                ansi.StylePrimitive{Color: stringPtr(colorToHex(t.FgBase))},
				NameBuiltin:         ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Yellow))},
				NameTag:             ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Pink))},
				NameAttribute:       ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Cyan))},
				NameClass:           ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Secondary)), Bold: boolPtr(true)},
				NameConstant:        ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Accent))},
				NameDecorator:       ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Pink))},
				NameException:       ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Error))},
				NameFunction:        ansi.StylePrimitive{Color: stringPtr(colorToHex(t.BlueLight))},
				NameOther:           ansi.StylePrimitive{Color: stringPtr(colorToHex(t.FgBase))},
				Literal:             ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Green))},
				LiteralNumber:       ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Yellow))},
				LiteralDate:         ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Green))},
				LiteralString:       ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Green))},
				LiteralStringEscape: ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Orange))},
				GenericDeleted:      ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Error))},
				GenericEmph:         ansi.StylePrimitive{Color: stringPtr(colorToHex(t.FgBase)), Italic: boolPtr(true)},
				GenericInserted:     ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Success))},
				GenericStrong:       ansi.StylePrimitive{Color: stringPtr(colorToHex(t.FgBase)), Bold: boolPtr(true)},
				GenericSubheading:   ansi.StylePrimitive{Color: stringPtr(colorToHex(t.Secondary))},
				Background:          ansi.StylePrimitive{BackgroundColor: stringPtr(colorToHex(t.BgSubtle))},
			},
		},
		Table: ansi.StyleTable{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{},
			},
			CenterSeparator: stringPtr("┼"),
			ColumnSeparator: stringPtr("│"),
			RowSeparator:    stringPtr("─"),
		},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix: "\n🠶 ",
		},
	}
}

// Manager handles theme switching and registration
type Manager struct {
	themes  map[string]*Theme
	current *Theme
}

var defaultManager *Manager

func SetDefaultManager(m *Manager) {
	defaultManager = m
}

func DefaultManager() *Manager {
	if defaultManager == nil {
		defaultManager = NewManager("loco")
	}
	return defaultManager
}

func CurrentTheme() *Theme {
	if defaultManager == nil {
		defaultManager = NewManager("loco")
	}
	return defaultManager.Current()
}

func NewManager(defaultTheme string) *Manager {
	m := &Manager{
		themes: make(map[string]*Theme),
	}

	// Register the beautiful Loco theme
	m.Register(NewLocoTheme())

	m.current = m.themes[defaultTheme]
	if m.current == nil {
		m.current = m.themes["loco"]
	}

	return m
}

func (m *Manager) Register(theme *Theme) {
	m.themes[theme.Name] = theme
}

func (m *Manager) Current() *Theme {
	return m.current
}

func (m *Manager) SetTheme(name string) error {
	if theme, ok := m.themes[name]; ok {
		m.current = theme
		return nil
	}
	return fmt.Errorf("theme %s not found", name)
}

func (m *Manager) List() []string {
	names := make([]string, 0, len(m.themes))
	for name := range m.themes {
		names = append(names, name)
	}
	return names
}

// Color utility functions

// ParseHex converts hex string to color
func ParseHex(hex string) color.Color {
	var r, g, b uint8
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

// Alpha returns a color with transparency
func Alpha(c color.Color, alpha uint8) color.Color {
	r, g, b, _ := c.RGBA()
	return color.RGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: alpha,
	}
}

// Darken makes a color darker by percentage (0-100)
func Darken(c color.Color, percent float64) color.Color {
	r, g, b, a := c.RGBA()
	factor := 1.0 - percent/100.0
	return color.RGBA{
		R: uint8(float64(r>>8) * factor),
		G: uint8(float64(g>>8) * factor),
		B: uint8(float64(b>>8) * factor),
		A: uint8(a >> 8),
	}
}

// Lighten makes a color lighter by percentage (0-100)
func Lighten(c color.Color, percent float64) color.Color {
	r, g, b, a := c.RGBA()
	factor := percent / 100.0
	return color.RGBA{
		R: uint8(min(255, float64(r>>8)+255*factor)),
		G: uint8(min(255, float64(g>>8)+255*factor)),
		B: uint8(min(255, float64(b>>8)+255*factor)),
		A: uint8(a >> 8),
	}
}

// ApplyGradient renders text with a horizontal gradient
func ApplyGradient(text string, color1, color2 color.Color) string {
	if text == "" {
		return ""
	}

	var output strings.Builder
	if len(text) == 1 {
		return lipgloss.NewStyle().Foreground(color1).Render(text)
	}

	// Handle Unicode properly
	var clusters []string
	gr := uniseg.NewGraphemes(text)
	for gr.Next() {
		clusters = append(clusters, string(gr.Runes()))
	}

	colors := blendColors(len(clusters), color1, color2)
	for i, cluster := range clusters {
		style := lipgloss.NewStyle().Foreground(colors[i])
		fmt.Fprint(&output, style.Render(cluster))
	}

	return output.String()
}

// ApplyBoldGradient renders text with a bold horizontal gradient
func ApplyBoldGradient(text string, color1, color2 color.Color) string {
	if text == "" {
		return ""
	}

	var output strings.Builder
	if len(text) == 1 {
		return lipgloss.NewStyle().Foreground(color1).Bold(true).Render(text)
	}

	// Handle Unicode properly
	var clusters []string
	gr := uniseg.NewGraphemes(text)
	for gr.Next() {
		clusters = append(clusters, string(gr.Runes()))
	}

	colors := blendColors(len(clusters), color1, color2)
	for i, cluster := range clusters {
		style := lipgloss.NewStyle().Foreground(colors[i]).Bold(true)
		fmt.Fprint(&output, style.Render(cluster))
	}

	return output.String()
}

// blendColors creates a gradient between colors
func blendColors(steps int, color1, color2 color.Color) []color.Color {
	if steps <= 0 {
		return nil
	}
	if steps == 1 {
		return []color.Color{color1}
	}

	colors := make([]color.Color, steps)
	
	// Convert to colorful for better blending
	c1, _ := colorful.MakeColor(color1)
	c2, _ := colorful.MakeColor(color2)

	for i := 0; i < steps; i++ {
		t := float64(i) / float64(steps-1)
		// Use HCL color space for perceptually uniform blending
		colors[i] = c1.BlendHcl(c2, t)
	}

	return colors
}

// colorToHex converts color to hex string
func colorToHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("%02x%02x%02x", r>>8, g>>8, b>>8)
}