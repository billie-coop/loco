package styles

import (
	"image/color"
	"strings"

	"github.com/charmbracelet/lipgloss/v2"
)

// GradientType defines different gradient styles
type GradientType int

const (
	GradientHorizontal GradientType = iota
	GradientVertical
	GradientDiagonal
)

// RenderGradientText applies a gradient to text
func RenderGradientText(text string, startColor, endColor color.Color, bold bool) string {
	if bold {
		return ApplyBoldGradient(text, startColor, endColor)
	}
	return ApplyGradient(text, startColor, endColor)
}

// RenderThemeGradient renders text with the current theme's primary gradient
func RenderThemeGradient(text string, bold bool) string {
	theme := CurrentTheme()
	return RenderGradientText(text, theme.Primary, theme.Secondary, bold)
}

// RenderAccentGradient renders text with the current theme's accent gradient
func RenderAccentGradient(text string, bold bool) string {
	theme := CurrentTheme()
	return RenderGradientText(text, theme.Tertiary, theme.Accent, bold)
}

// GetGradientColors returns a slice of colors for creating gradients
func GetGradientColors(numColors int) []color.Color {
	theme := CurrentTheme()
	return blendColors(numColors, theme.Primary, theme.Secondary)
}

// GetAccentGradientColors returns accent gradient colors
func GetAccentGradientColors(numColors int) []color.Color {
	theme := CurrentTheme()
	return blendColors(numColors, theme.Tertiary, theme.Accent)
}

// ColorForIndex returns a color from the gradient based on index
func ColorForIndex(index, total int) color.Color {
	if total <= 0 {
		return CurrentTheme().Primary
	}
	colors := GetGradientColors(total)
	if index >= len(colors) {
		return colors[len(colors)-1]
	}
	return colors[index]
}

// RenderGradientBar creates a gradient progress bar
func RenderGradientBar(width int, filled float64) string {
	if width <= 0 {
		return ""
	}

	theme := CurrentTheme()
	filledWidth := int(float64(width) * filled)
	if filledWidth <= 0 {
		return strings.Repeat(" ", width)
	}

	var bar strings.Builder
	colors := blendColors(filledWidth, theme.Primary, theme.Secondary)
	
	for i := 0; i < filledWidth; i++ {
		style := lipgloss.NewStyle().Foreground(colors[i])
		bar.WriteString(style.Render("█"))
	}
	
	// Add empty space for unfilled portion
	if filledWidth < width {
		bar.WriteString(strings.Repeat(" ", width-filledWidth))
	}
	
	return bar.String()
}

// RenderGradientBorder creates a border with gradient colors
func RenderGradientBorder(content string, width, height int) string {
	theme := CurrentTheme()
	
	// Create a gradient border style
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary)
	
	return style.Render(content)
}

// RenderGradientBox creates a box with gradient background
func RenderGradientBox(content string, width, height int) string {
	theme := CurrentTheme()
	
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Background(theme.BgSubtle).
		Foreground(theme.FgBase).
		Padding(1, 2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary)
	
	return style.Render(content)
}

// AnimatedGradientText creates text that can animate through gradient colors
type AnimatedGradientText struct {
	text   string
	colors []color.Color
	offset int
}

// NewAnimatedGradientText creates a new animated gradient text
func NewAnimatedGradientText(text string) *AnimatedGradientText {
	theme := CurrentTheme()
	// Create more colors than needed for smooth animation
	colors := blendColors(len(text)*2, theme.Primary, theme.Secondary)
	return &AnimatedGradientText{
		text:   text,
		colors: colors,
		offset: 0,
	}
}

// Tick advances the animation
func (a *AnimatedGradientText) Tick() {
	a.offset = (a.offset + 1) % len(a.colors)
}

// Render returns the current frame of the animated text
func (a *AnimatedGradientText) Render() string {
	if a.text == "" {
		return ""
	}

	var output strings.Builder
	
	for i, ch := range a.text {
		colorIndex := (i + a.offset) % len(a.colors)
		style := lipgloss.NewStyle().Foreground(a.colors[colorIndex])
		output.WriteString(style.Render(string(ch)))
	}
	
	return output.String()
}