package styles

// NewLocoTheme creates the default Loco theme with fire gradient (matches 🚂 emoji!)
func NewLocoTheme() *Theme {
	return &Theme{
		Name:   "loco",
		IsDark: true,

		// Brand colors - fire gradient to match the locomotive emoji
		Primary:   ParseHex("#C0392B"), // Fire red
		Secondary: ParseHex("#F4D03F"), // Bright yellow
		Tertiary:  ParseHex("#E67E22"), // Orange
		Accent:    ParseHex("#F39C12"), // Golden orange

		// Background colors - slate gray theme
		BgBase:        ParseHex("#2C3E50"), // Slate gray base
		BgBaseLighter: ParseHex("#34495E"), // Slightly lighter
		BgSubtle:      ParseHex("#3D566E"), // Subtle contrast
		BgOverlay:     ParseHex("#4A6278"), // For overlays
		BgHighlight:   ParseHex("#5D6D7E"), // Highlighted items

		// Foreground colors
		FgBase:      ParseHex("#f5f6fa"), // Lynx white
		FgMuted:     ParseHex("#a0a0a0"), // Muted text
		FgHalfMuted: ParseHex("#7A7A7A"), // Between muted and subtle
		FgSubtle:    ParseHex("#6F6F70"), // Subtle text
		FgSelected:  ParseHex("#ffffff"), // Selected text (pure white)
		FgInverted:  ParseHex("#1e1e1e"), // For light backgrounds

		// Border colors
		Border:      ParseHex("#5D6D7E"), // Subtle slate border
		BorderFocus: ParseHex("#F39C12"), // Golden focus (fire theme!)

		// Semantic colors
		Success: ParseHex("#27AE60"), // Emerald green
		Error:   ParseHex("#E74C3C"), // Bright red
		Warning: ParseHex("#F39C12"), // Orange (matches accent)
		Info:    ParseHex("#3498DB"), // Sky blue

		// Special colors
		Blue:      ParseHex("#008ae5"),
		BlueLight: ParseHex("#5EB3F6"),
		Green:     ParseHex("#3DCC91"),
		Yellow:    ParseHex("#F4D03F"),
		Purple:    ParseHex("#7c3aed"), // Violet
		Pink:      ParseHex("#EC4899"),
		Orange:    ParseHex("#F97316"),
		Cyan:      ParseHex("#00CED1"),
	}
}

// NewDarkTheme creates a professional dark theme
func NewDarkTheme() *Theme {
	return &Theme{
		Name:   "dark",
		IsDark: true,

		// Brand colors
		Primary:   ParseHex("#60a5fa"), // Sky blue
		Secondary: ParseHex("#a78bfa"), // Violet
		Tertiary:  ParseHex("#f472b6"), // Pink
		Accent:    ParseHex("#34d399"), // Emerald

		// Background colors
		BgBase:        ParseHex("#0f172a"), // Slate 900
		BgBaseLighter: ParseHex("#1e293b"), // Slate 800
		BgSubtle:      ParseHex("#334155"), // Slate 700
		BgOverlay:     ParseHex("#475569"), // Slate 600
		BgHighlight:   ParseHex("#64748b"), // Slate 500

		// Foreground colors
		FgBase:     ParseHex("#f8fafc"), // Slate 50
		FgMuted:    ParseHex("#cbd5e1"), // Slate 300
		FgSubtle:   ParseHex("#94a3b8"), // Slate 400
		FgInverted: ParseHex("#0f172a"), // Slate 900

		// Border colors
		Border:      ParseHex("#334155"), // Slate 700
		BorderFocus: ParseHex("#60a5fa"), // Sky 400

		// Semantic colors
		Success: ParseHex("#34d399"), // Emerald 400
		Error:   ParseHex("#f87171"), // Red 400
		Warning: ParseHex("#fbbf24"), // Amber 400
		Info:    ParseHex("#60a5fa"), // Sky 400

		// Special colors
		Blue:      ParseHex("#60a5fa"),
		BlueLight: ParseHex("#93c5fd"),
		Green:     ParseHex("#34d399"),
		Yellow:    ParseHex("#fbbf24"),
		Purple:    ParseHex("#a78bfa"),
		Pink:      ParseHex("#f472b6"),
		Orange:    ParseHex("#fb923c"),
		Cyan:      ParseHex("#67e8f9"),
	}
}

// NewAuroraTheme creates a purple->blue gradient theme
func NewAuroraTheme() *Theme {
	return &Theme{
		Name:   "aurora",
		IsDark: true,

		// Brand colors - purple to blue gradient
		Primary:   ParseHex("#7c3aed"), // Violet
		Secondary: ParseHex("#60a5fa"), // Light blue
		Tertiary:  ParseHex("#8b5cf6"), // Purple
		Accent:    ParseHex("#a78bfa"), // Light purple

		// Background colors
		BgBase:        ParseHex("#1e40af"), // Blue 800
		BgBaseLighter: ParseHex("#1e3a8a"), // Blue 900
		BgSubtle:      ParseHex("#312e81"), // Indigo 900
		BgOverlay:     ParseHex("#4c1d95"), // Purple 900
		BgHighlight:   ParseHex("#5b21b6"), // Purple 800

		// Foreground colors
		FgBase:     ParseHex("#f5f3ff"), // Violet 50
		FgMuted:    ParseHex("#c4b5fd"), // Violet 300
		FgSubtle:   ParseHex("#a78bfa"), // Violet 400
		FgInverted: ParseHex("#1e1b4b"), // Indigo 950

		// Border colors
		Border:      ParseHex("#6366f1"), // Indigo 500
		BorderFocus: ParseHex("#a78bfa"), // Violet 400

		// Semantic colors
		Success: ParseHex("#34d399"), // Emerald
		Error:   ParseHex("#f87171"), // Red
		Warning: ParseHex("#fbbf24"), // Amber
		Info:    ParseHex("#60a5fa"), // Sky

		// Special colors
		Blue:      ParseHex("#60a5fa"),
		BlueLight: ParseHex("#93c5fd"),
		Green:     ParseHex("#34d399"),
		Yellow:    ParseHex("#fbbf24"),
		Purple:    ParseHex("#a78bfa"),
		Pink:      ParseHex("#f472b6"),
		Orange:    ParseHex("#fb923c"),
		Cyan:      ParseHex("#67e8f9"),
	}
}

// NewSunsetTheme creates a red->yellow gradient theme
func NewSunsetTheme() *Theme {
	return &Theme{
		Name:   "sunset",
		IsDark: true,

		// Brand colors - sunset gradient
		Primary:   ParseHex("#C0392B"), // Pomegranate red
		Secondary: ParseHex("#F4D03F"), // Sunflower yellow
		Tertiary:  ParseHex("#E67E22"), // Carrot orange
		Accent:    ParseHex("#F39C12"), // Orange

		// Background colors
		BgBase:        ParseHex("#2E4053"), // Dark blue-gray
		BgBaseLighter: ParseHex("#34495E"), // Lighter blue-gray
		BgSubtle:      ParseHex("#5D6D7E"), // Subtle blue-gray
		BgOverlay:     ParseHex("#85929E"), // Overlay gray
		BgHighlight:   ParseHex("#ABB2B9"), // Highlight gray

		// Foreground colors
		FgBase:     ParseHex("#FDFEFE"), // Almost white
		FgMuted:    ParseHex("#D5D8DC"), // Light gray
		FgSubtle:   ParseHex("#AEB6BF"), // Subtle gray
		FgInverted: ParseHex("#2E4053"), // Dark for light bg

		// Border colors
		Border:      ParseHex("#5D6D7E"), // Subtle border
		BorderFocus: ParseHex("#F39C12"), // Orange focus

		// Semantic colors
		Success: ParseHex("#27AE60"), // Emerald
		Error:   ParseHex("#E74C3C"), // Alizarin
		Warning: ParseHex("#F39C12"), // Orange
		Info:    ParseHex("#3498DB"), // Peter river

		// Special colors
		Blue:      ParseHex("#3498DB"),
		BlueLight: ParseHex("#5DADE2"),
		Green:     ParseHex("#27AE60"),
		Yellow:    ParseHex("#F4D03F"),
		Purple:    ParseHex("#8E44AD"),
		Pink:      ParseHex("#EC7063"),
		Orange:    ParseHex("#E67E22"),
		Cyan:      ParseHex("#48C9B0"),
	}
}

// NewFireTheme creates a fire-inspired red->yellow gradient theme
func NewFireTheme() *Theme {
	return &Theme{
		Name:   "fire",
		IsDark: true,

		// Brand colors - fire gradient
		Primary:   ParseHex("#C0392B"), // Deep red
		Secondary: ParseHex("#F4D03F"), // Bright yellow
		Tertiary:  ParseHex("#E74C3C"), // Bright red
		Accent:    ParseHex("#F39C12"), // Orange

		// Background colors
		BgBase:        ParseHex("#708090"), // Slate gray
		BgBaseLighter: ParseHex("#778899"), // Light slate gray
		BgSubtle:      ParseHex("#696969"), // Dim gray
		BgOverlay:     ParseHex("#A9A9A9"), // Dark gray
		BgHighlight:   ParseHex("#C0C0C0"), // Silver

		// Foreground colors
		FgBase:     ParseHex("#FFFFFF"), // White
		FgMuted:    ParseHex("#F5F5F5"), // White smoke
		FgSubtle:   ParseHex("#DCDCDC"), // Gainsboro
		FgInverted: ParseHex("#000000"), // Black

		// Border colors
		Border:      ParseHex("#A9A9A9"), // Dark gray
		BorderFocus: ParseHex("#F39C12"), // Orange

		// Semantic colors
		Success: ParseHex("#2ECC71"), // Emerald
		Error:   ParseHex("#E74C3C"), // Alizarin
		Warning: ParseHex("#F1C40F"), // Sunflower
		Info:    ParseHex("#3498DB"), // Peter river

		// Special colors
		Blue:      ParseHex("#3498DB"),
		BlueLight: ParseHex("#5DADE2"),
		Green:     ParseHex("#2ECC71"),
		Yellow:    ParseHex("#F1C40F"),
		Purple:    ParseHex("#9B59B6"),
		Pink:      ParseHex("#EC7063"),
		Orange:    ParseHex("#E67E22"),
		Cyan:      ParseHex("#1ABC9C"),
	}
}