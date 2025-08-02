package styles

import (
	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/glamour/v2/ansi"
)

func chromaStyle(style ansi.StylePrimitive) string {
	var s string

	if style.Color != nil {
		s = *style.Color
	}
	if style.BackgroundColor != nil {
		if s != "" {
			s += " "
		}
		s += "bg:" + *style.BackgroundColor
	}
	if style.Italic != nil && *style.Italic {
		if s != "" {
			s += " "
		}
		s += "italic"
	}
	if style.Bold != nil && *style.Bold {
		if s != "" {
			s += " "
		}
		s += "bold"
	}
	if style.Underline != nil && *style.Underline {
		if s != "" {
			s += " "
		}
		s += "underline"
	}

	return s
}

func GetChromaTheme() chroma.StyleEntries {
	t := CurrentTheme()
	
	// Create syntax highlighting rules matching our theme
	return chroma.StyleEntries{
		chroma.Text:                "#" + colorToHex(t.FgBase),
		chroma.Error:               "#" + colorToHex(t.Error),
		chroma.Comment:             "#" + colorToHex(t.FgMuted) + " italic",
		chroma.CommentPreproc:      "#" + colorToHex(t.Warning),
		chroma.Keyword:             "#" + colorToHex(t.Primary) + " bold",
		chroma.KeywordReserved:     "#" + colorToHex(t.Accent) + " bold",
		chroma.KeywordNamespace:    "#" + colorToHex(t.Purple),
		chroma.KeywordType:         "#" + colorToHex(t.Blue),
		chroma.Operator:            "#" + colorToHex(t.Orange),
		chroma.Punctuation:         "#" + colorToHex(t.FgSubtle),
		chroma.Name:                "#" + colorToHex(t.FgBase),
		chroma.NameBuiltin:         "#" + colorToHex(t.Yellow),
		chroma.NameTag:             "#" + colorToHex(t.Pink),
		chroma.NameAttribute:       "#" + colorToHex(t.Cyan),
		chroma.NameClass:           "#" + colorToHex(t.Secondary) + " bold",
		chroma.NameConstant:        "#" + colorToHex(t.Accent),
		chroma.NameDecorator:       "#" + colorToHex(t.Pink),
		chroma.NameException:       "#" + colorToHex(t.Error),
		chroma.NameFunction:        "#" + colorToHex(t.BlueLight),
		chroma.NameOther:           "#" + colorToHex(t.FgBase),
		chroma.Literal:             "#" + colorToHex(t.Green),
		chroma.LiteralNumber:       "#" + colorToHex(t.Yellow),
		chroma.LiteralDate:         "#" + colorToHex(t.Green),
		chroma.LiteralString:       "#" + colorToHex(t.Green),
		chroma.LiteralStringEscape: "#" + colorToHex(t.Orange),
		chroma.GenericDeleted:      "#" + colorToHex(t.Error),
		chroma.GenericEmph:         "#" + colorToHex(t.FgBase) + " italic",
		chroma.GenericInserted:     "#" + colorToHex(t.Success),
		chroma.GenericStrong:       "#" + colorToHex(t.FgBase) + " bold",
		chroma.GenericSubheading:   "#" + colorToHex(t.Secondary),
		chroma.Background:          "#" + colorToHex(t.BgSubtle),
	}
}

