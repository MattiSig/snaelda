package siteconfig

import "maps"

const (
	ThemePaletteCalmNordic    = "calm-nordic"
	ThemePalettePlayfulRibbon = "playful-ribbon"
	ThemePaletteMeanerDark    = "meaner-dark"

	ThemeFontBalanced   = "balanced"
	ThemeFontEditorial  = "editorial"
	ThemeFontStudioSans = "studio-sans"

	ThemeSpacingSnug        = "snug"
	ThemeSpacingComfortable = "comfortable"
	ThemeSpacingAiry        = "airy"

	ThemeRadiusTailored = "tailored"
	ThemeRadiusSoft     = "soft"
	ThemeRadiusPillowy  = "pillowy"
)

type ThemeSelection struct {
	Palette        string `json:"palette"`
	FontPreset     string `json:"fontPreset"`
	SectionSpacing string `json:"sectionSpacing"`
	Radius         string `json:"radius"`
}

type ThemeOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type ThemeEditorCatalog struct {
	Palettes        []ThemeOption `json:"palettes"`
	FontPresets     []ThemeOption `json:"fontPresets"`
	SectionSpacings []ThemeOption `json:"sectionSpacings"`
	Radii           []ThemeOption `json:"radii"`
}

var (
	themePaletteTokens = map[string]map[string]string{
		ThemePaletteCalmNordic: {
			"background":   "#f4efe6",
			"foreground":   "#2d2426",
			"surface":      "#fff9f0",
			"surfaceMuted": "#eadfd2",
			"primary":      "#3c78ad",
			"secondary":    "#5bb7a7",
			"accent":       "#e9708c",
			"muted":        "#a8784d",
			"border":       "#decfbe",
			"ring":         "#d99633",
		},
		ThemePalettePlayfulRibbon: {
			"background":   "#fff7ee",
			"foreground":   "#2d2426",
			"surface":      "#fff0e8",
			"surfaceMuted": "#f6ddd5",
			"primary":      "#d95f7e",
			"secondary":    "#67bfb3",
			"accent":       "#4f84c8",
			"muted":        "#b88452",
			"border":       "#e8c7ba",
			"ring":         "#e0a23f",
		},
		ThemePaletteMeanerDark: {
			"background":   "#191119",
			"foreground":   "#f3ead8",
			"surface":      "#241a24",
			"surfaceMuted": "#302333",
			"primary":      "#86d8cf",
			"secondary":    "#89b9f0",
			"accent":       "#ff8a9d",
			"muted":        "#caa778",
			"border":       "#5a3e57",
			"ring":         "#f2bd63",
		},
	}
	themeTypographyTokens = map[string]map[string]any{
		ThemeFontBalanced: {
			"heading":     "Iowan Old Style",
			"body":        "Avenir Next",
			"headingFont": "Iowan Old Style",
			"bodyFont":    "Avenir Next",
			"scale":       "calm",
		},
		ThemeFontEditorial: {
			"heading":     "Iowan Old Style",
			"body":        "Avenir Next",
			"headingFont": "Iowan Old Style",
			"bodyFont":    "Avenir Next",
			"scale":       "editorial",
		},
		ThemeFontStudioSans: {
			"heading":     "Avenir Next",
			"body":        "Avenir Next",
			"headingFont": "Avenir Next",
			"bodyFont":    "Avenir Next",
			"scale":       "playful",
		},
	}
	themeSectionSpacingValues = map[string]string{
		ThemeSpacingSnug:        "88px",
		ThemeSpacingComfortable: "96px",
		ThemeSpacingAiry:        "120px",
	}
	themeRadiusValues = map[string]string{
		ThemeRadiusTailored: "22px",
		ThemeRadiusSoft:     "28px",
		ThemeRadiusPillowy:  "32px",
	}
	themeCatalog = ThemeEditorCatalog{
		Palettes: []ThemeOption{
			{ID: ThemePaletteCalmNordic, Label: "Calm Nordic", Description: "Vellum neutrals with blue, teal, and coral held in balance."},
			{ID: ThemePalettePlayfulRibbon, Label: "Playful Ribbon", Description: "Warm paper surfaces with coral leading the woven accents."},
			{ID: ThemePaletteMeanerDark, Label: "Meaner Dark", Description: "Mulberry-black surfaces with teal action and coral sparks."},
		},
		FontPresets: []ThemeOption{
			{ID: ThemeFontBalanced, Label: "Balanced", Description: "Serif headings with calm supporting sans text."},
			{ID: ThemeFontEditorial, Label: "Editorial", Description: "Sharper serif display with the highest contrast hierarchy."},
			{ID: ThemeFontStudioSans, Label: "Studio Sans", Description: "Unified sans treatment for a lighter, brisker tone."},
		},
		SectionSpacings: []ThemeOption{
			{ID: ThemeSpacingSnug, Label: "Snug", Description: "Tighter section rhythm for denser pages."},
			{ID: ThemeSpacingComfortable, Label: "Comfortable", Description: "The default prototype spacing balance."},
			{ID: ThemeSpacingAiry, Label: "Airy", Description: "More breathing room between major sections."},
		},
		Radii: []ThemeOption{
			{ID: ThemeRadiusTailored, Label: "Tailored", Description: "Sharper corners with a little craft left in."},
			{ID: ThemeRadiusSoft, Label: "Soft", Description: "Rounded panels aligned with the current prototype."},
			{ID: ThemeRadiusPillowy, Label: "Pillowy", Description: "Extra rounded surfaces for the warmest feel."},
		},
	}
)

func DefaultThemeSelection() ThemeSelection {
	return ThemeSelection{
		Palette:        ThemePaletteMeanerDark,
		FontPreset:     ThemeFontEditorial,
		SectionSpacing: ThemeSpacingComfortable,
		Radius:         ThemeRadiusSoft,
	}
}

func DefaultThemeEditorCatalog() ThemeEditorCatalog {
	return themeCatalog
}

func ThemePreset(name string) ThemeConfig {
	switch name {
	case ThemePalettePlayfulRibbon:
		return BuildTheme(ThemeSelection{
			Palette:        ThemePalettePlayfulRibbon,
			FontPreset:     ThemeFontStudioSans,
			SectionSpacing: ThemeSpacingSnug,
			Radius:         ThemeRadiusPillowy,
		})
	case ThemePaletteMeanerDark:
		return BuildTheme(ThemeSelection{
			Palette:        ThemePaletteMeanerDark,
			FontPreset:     ThemeFontEditorial,
			SectionSpacing: ThemeSpacingComfortable,
			Radius:         ThemeRadiusSoft,
		})
	default:
		return BuildTheme(ThemeSelection{
			Palette:        ThemePaletteCalmNordic,
			FontPreset:     ThemeFontBalanced,
			SectionSpacing: ThemeSpacingComfortable,
			Radius:         ThemeRadiusSoft,
		})
	}
}

func BuildTheme(selection ThemeSelection) ThemeConfig {
	normalized := normalizeThemeSelection(selection)
	return ThemeConfig{
		Version: ThemeVersionV1,
		Tokens: ThemeTokens{
			Colors:     maps.Clone(themePaletteTokens[normalized.Palette]),
			Typography: maps.Clone(themeTypographyTokens[normalized.FontPreset]),
			Layout: map[string]any{
				"maxWidth":       "1120px",
				"contentWidth":   "720px",
				"sectionSpacing": themeSectionSpacingValues[normalized.SectionSpacing],
			},
			Shape: map[string]any{
				"radius": themeRadiusValues[normalized.Radius],
				"shadow": "soft",
			},
		},
	}
}

func DetectThemeSelection(theme ThemeConfig) ThemeSelection {
	selection := DefaultThemeSelection()

	if palette := detectThemePalette(theme.Tokens.Colors); palette != "" {
		selection.Palette = palette
	}
	if fontPreset := detectThemeFontPreset(theme.Tokens.Typography); fontPreset != "" {
		selection.FontPreset = fontPreset
	}
	if spacing := detectThemeSectionSpacing(theme.Tokens.Layout); spacing != "" {
		selection.SectionSpacing = spacing
	}
	if radius := detectThemeRadius(theme.Tokens.Shape); radius != "" {
		selection.Radius = radius
	}

	return selection
}

func normalizeThemeSelection(selection ThemeSelection) ThemeSelection {
	normalized := selection
	if _, ok := themePaletteTokens[normalized.Palette]; !ok {
		normalized.Palette = DefaultThemeSelection().Palette
	}
	if _, ok := themeTypographyTokens[normalized.FontPreset]; !ok {
		normalized.FontPreset = DefaultThemeSelection().FontPreset
	}
	if _, ok := themeSectionSpacingValues[normalized.SectionSpacing]; !ok {
		normalized.SectionSpacing = DefaultThemeSelection().SectionSpacing
	}
	if _, ok := themeRadiusValues[normalized.Radius]; !ok {
		normalized.Radius = DefaultThemeSelection().Radius
	}
	return normalized
}

func detectThemePalette(colors map[string]string) string {
	for id, tokens := range themePaletteTokens {
		if sameStringMap(colors, tokens) {
			return id
		}
	}
	return ""
}

func detectThemeFontPreset(typography map[string]any) string {
	for id, tokens := range themeTypographyTokens {
		if sameAnyMap(typography, tokens) {
			return id
		}
	}
	return ""
}

func detectThemeSectionSpacing(layout map[string]any) string {
	value, _ := layout["sectionSpacing"].(string)
	for id, spacing := range themeSectionSpacingValues {
		if value == spacing {
			return id
		}
	}
	return ""
}

func detectThemeRadius(shape map[string]any) string {
	value, _ := shape["radius"].(string)
	for id, radius := range themeRadiusValues {
		if value == radius {
			return id
		}
	}
	return ""
}

func sameStringMap(left map[string]string, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range right {
		if left[key] != value {
			return false
		}
	}
	return true
}

func sameAnyMap(left map[string]any, right map[string]any) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range right {
		if left[key] != value {
			return false
		}
	}
	return true
}
