package siteconfig

import (
	"fmt"
	"maps"
	"math"
)

const (
	ThemePaletteCalmNordic      = "calm-nordic"
	ThemePaletteCleanLocal      = "clean-local"
	ThemePaletteBrightShopfront = "bright-shopfront"
	ThemePaletteEditorialStudio = "editorial-studio"
	ThemePaletteHeritageCraft   = "heritage-craft"
	ThemePaletteAfterHours      = "after-hours"

	ThemeFontBalanced      = "balanced"
	ThemeFontEditorial     = "editorial"
	ThemeFontStudioSans    = "studio-sans"
	ThemeFontModernGrotesk = "modern-grotesk"
	ThemeFontHumanist      = "humanist"
	ThemeFontHeritageSerif = "heritage-serif"

	ThemeTypeScaleCompact    = "compact"
	ThemeTypeScaleBalanced   = "balanced"
	ThemeTypeScaleExpressive = "expressive"

	ThemeSpacingSnug        = "snug"
	ThemeSpacingComfortable = "comfortable"
	ThemeSpacingAiry        = "airy"

	ThemeContentWidthFocused  = "focused"
	ThemeContentWidthStandard = "standard"
	ThemeContentWidthWide     = "wide"

	ThemeRadiusSharp    = "sharp"
	ThemeRadiusCrisp    = "crisp"
	ThemeRadiusTailored = "tailored"
	ThemeRadiusSoft     = "soft"
	ThemeRadiusPillowy  = "pillowy"

	ThemeButtonRibbonFill    = "ribbon-fill"
	ThemeButtonThreadOutline = "thread-outline"
	ThemeButtonInkSolid      = "ink-solid"

	ThemeImageSoftFrame = "soft-frame"
	ThemeImageWovenTint = "woven-tint"
	ThemeImagePaperCut  = "paper-cut"
)

type ThemeSelection struct {
	Palette        string `json:"palette"`
	FontPreset     string `json:"fontPreset"`
	TypeScale      string `json:"typeScale"`
	SectionSpacing string `json:"sectionSpacing"`
	ContentWidth   string `json:"contentWidth"`
	Radius         string `json:"radius"`
	ButtonStyle    string `json:"buttonStyle"`
	ImageStyle     string `json:"imageStyle"`
}

type ThemeOption struct {
	ID            string            `json:"id"`
	Label         string            `json:"label"`
	Description   string            `json:"description,omitempty"`
	PreviewColors map[string]string `json:"previewColors,omitempty"`
}

type ThemeEditorCatalog struct {
	Palettes        []ThemeOption `json:"palettes"`
	FontPresets     []ThemeOption `json:"fontPresets"`
	TypeScales      []ThemeOption `json:"typeScales"`
	SectionSpacings []ThemeOption `json:"sectionSpacings"`
	ContentWidths   []ThemeOption `json:"contentWidths"`
	Radii           []ThemeOption `json:"radii"`
	ButtonStyles    []ThemeOption `json:"buttonStyles"`
	ImageStyles     []ThemeOption `json:"imageStyles"`
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
		ThemePaletteCleanLocal: {
			"background":   "#f7f3ea",
			"foreground":   "#2c2721",
			"surface":      "#fffaf1",
			"surfaceMuted": "#ebe3d5",
			"primary":      "#426b5c",
			"secondary":    "#6f8f82",
			"accent":       "#b46a4d",
			"muted":        "#8c765c",
			"border":       "#d9cebd",
			"ring":         "#9b7f52",
		},
		ThemePaletteBrightShopfront: {
			"background":   "#fff3df",
			"foreground":   "#33231b",
			"surface":      "#fffaf0",
			"surfaceMuted": "#f5d9bd",
			"primary":      "#c65f32",
			"secondary":    "#2f9488",
			"accent":       "#d98b2b",
			"muted":        "#956b4b",
			"border":       "#e8c2a2",
			"ring":         "#d17938",
		},
		ThemePaletteEditorialStudio: {
			"background":   "#f2eee6",
			"foreground":   "#1f1d1b",
			"surface":      "#fbf8f1",
			"surfaceMuted": "#e2dbcf",
			"primary":      "#3a4e59",
			"secondary":    "#8b6f55",
			"accent":       "#9b4f4c",
			"muted":        "#72685e",
			"border":       "#cbc2b5",
			"ring":         "#92785f",
		},
		ThemePaletteHeritageCraft: {
			"background":   "#f5eadb",
			"foreground":   "#34251e",
			"surface":      "#fff6e8",
			"surfaceMuted": "#e7d1bc",
			"primary":      "#875c3d",
			"secondary":    "#6f7d54",
			"accent":       "#b25f45",
			"muted":        "#876e58",
			"border":       "#d7bda5",
			"ring":         "#a86f42",
		},
		ThemePaletteAfterHours: {
			"background":   "#151314",
			"foreground":   "#f1e8d8",
			"surface":      "#211d20",
			"surfaceMuted": "#2e2830",
			"primary":      "#d58f4f",
			"secondary":    "#7fb3a5",
			"accent":       "#b986d0",
			"muted":        "#c4a77e",
			"border":       "#514653",
			"ring":         "#e0b369",
		},
	}
	themeTypographyTokens = map[string]map[string]any{
		ThemeFontBalanced: {
			"heading":       "Literata",
			"body":          "Be Vietnam Pro",
			"headingFont":   "Literata",
			"bodyFont":      "Be Vietnam Pro",
			"headingWeight": 600,
			"bodyWeight":    400,
		},
		ThemeFontEditorial: {
			"heading":       "Literata",
			"body":          "Literata",
			"headingFont":   "Literata",
			"bodyFont":      "Literata",
			"headingWeight": 700,
			"bodyWeight":    400,
		},
		ThemeFontStudioSans: {
			"heading":       "Be Vietnam Pro",
			"body":          "Be Vietnam Pro",
			"headingFont":   "Be Vietnam Pro",
			"bodyFont":      "Be Vietnam Pro",
			"headingWeight": 700,
			"bodyWeight":    400,
		},
		ThemeFontModernGrotesk: {
			"heading":       "Helvetica Neue",
			"body":          "Helvetica Neue",
			"headingFont":   "Helvetica Neue",
			"bodyFont":      "Helvetica Neue",
			"headingWeight": 700,
			"bodyWeight":    400,
		},
		ThemeFontHumanist: {
			"heading":       "Trebuchet MS",
			"body":          "Trebuchet MS",
			"headingFont":   "Trebuchet MS",
			"bodyFont":      "Trebuchet MS",
			"headingWeight": 700,
			"bodyWeight":    400,
		},
		ThemeFontHeritageSerif: {
			"heading":       "Georgia",
			"body":          "Georgia",
			"headingFont":   "Georgia",
			"bodyFont":      "Georgia",
			"headingWeight": 700,
			"bodyWeight":    400,
		},
	}
	themeTypeScaleValues = map[string]string{
		ThemeTypeScaleCompact:    "compact",
		ThemeTypeScaleBalanced:   "balanced",
		ThemeTypeScaleExpressive: "expressive",
	}
	themeSectionSpacingValues = map[string]string{
		ThemeSpacingSnug:        "88px",
		ThemeSpacingComfortable: "96px",
		ThemeSpacingAiry:        "120px",
	}
	themeContentWidthValues = map[string]string{
		ThemeContentWidthFocused:  "640px",
		ThemeContentWidthStandard: "720px",
		ThemeContentWidthWide:     "860px",
	}
	themeRadiusValues = map[string]string{
		ThemeRadiusSharp:    "0px",
		ThemeRadiusCrisp:    "8px",
		ThemeRadiusTailored: "16px",
		ThemeRadiusSoft:     "24px",
		ThemeRadiusPillowy:  "32px",
	}
	themeButtonStyles = map[string]struct{}{
		ThemeButtonRibbonFill:    {},
		ThemeButtonThreadOutline: {},
		ThemeButtonInkSolid:      {},
	}
	themeImageStyles = map[string]struct{}{
		ThemeImageSoftFrame: {},
		ThemeImageWovenTint: {},
		ThemeImagePaperCut:  {},
	}
	themeCatalog = ThemeEditorCatalog{
		Palettes: []ThemeOption{
			{ID: ThemePaletteCleanLocal, Label: "Clean Local", Description: "Practical warm neutrals for services, clinics, trades, classes, and consultants."},
			{ID: ThemePaletteBrightShopfront, Label: "Bright Shopfront", Description: "Sunlit, friendly surfaces for cafes, shops, salons, studios, and bookable local offers."},
			{ID: ThemePaletteEditorialStudio, Label: "Editorial Studio", Description: "Image-led restraint with sharper contrast for photographers, florists, makers, and boutique portfolios."},
			{ID: ThemePaletteCalmNordic, Label: "Calm Nordic", Description: "Muted vellum surfaces for quiet, spacious sites that should feel steady rather than loud."},
			{ID: ThemePaletteHeritageCraft, Label: "Heritage Craft", Description: "Earthier paper and clay tones for handmade, workshop, food, and place-based businesses."},
			{ID: ThemePaletteAfterHours, Label: "After Hours", Description: "A warm dark direction for bars, studios, music, events, tattoo, and dramatic visual brands."},
		},
		FontPresets: []ThemeOption{
			{ID: ThemeFontBalanced, Label: "Literary Sans", Description: "Literata headings with clean Be Vietnam Pro body text."},
			{ID: ThemeFontEditorial, Label: "Editorial Serif", Description: "Literata throughout for a bookish, text-led character."},
			{ID: ThemeFontStudioSans, Label: "Studio Sans", Description: "Be Vietnam Pro throughout for a warm contemporary voice."},
			{ID: ThemeFontModernGrotesk, Label: "Modern Grotesk", Description: "A tighter Helvetica-led system stack with a direct commercial tone."},
			{ID: ThemeFontHumanist, Label: "Humanist Sans", Description: "Open, friendly Trebuchet forms suited to services and community sites."},
			{ID: ThemeFontHeritageSerif, Label: "Heritage Serif", Description: "Georgia throughout for a familiar, traditional editorial feel."},
		},
		TypeScales: []ThemeOption{
			{ID: ThemeTypeScaleCompact, Label: "Compact", Description: "Smaller headings for information-dense sites and longer titles."},
			{ID: ThemeTypeScaleBalanced, Label: "Balanced", Description: "A clear default hierarchy with practical headline scale."},
			{ID: ThemeTypeScaleExpressive, Label: "Expressive", Description: "Larger display moments for short, confident messaging."},
		},
		SectionSpacings: []ThemeOption{
			{ID: ThemeSpacingSnug, Label: "Snug", Description: "Tighter section rhythm for denser pages."},
			{ID: ThemeSpacingComfortable, Label: "Comfortable", Description: "The default prototype spacing balance."},
			{ID: ThemeSpacingAiry, Label: "Airy", Description: "More breathing room between major sections."},
		},
		ContentWidths: []ThemeOption{
			{ID: ThemeContentWidthFocused, Label: "Focused", Description: "Narrower prose and forms for calm, guided reading."},
			{ID: ThemeContentWidthStandard, Label: "Standard", Description: "Balanced line lengths for most small-business sites."},
			{ID: ThemeContentWidthWide, Label: "Wide", Description: "More room for menus, listings, and content-rich sections."},
		},
		Radii: []ThemeOption{
			{ID: ThemeRadiusSharp, Label: "Sharp", Description: "True square corners with no rounding."},
			{ID: ThemeRadiusCrisp, Label: "Crisp", Description: "A restrained 8px corner for practical interfaces."},
			{ID: ThemeRadiusTailored, Label: "Tailored", Description: "Moderate corners that keep structure visible."},
			{ID: ThemeRadiusSoft, Label: "Soft", Description: "Generous rounding aligned with the warm default."},
			{ID: ThemeRadiusPillowy, Label: "Pillowy", Description: "Extra rounded surfaces for the warmest feel."},
		},
		ButtonStyles: []ThemeOption{
			{ID: ThemeButtonRibbonFill, Label: "Ribbon Fill", Description: "Primary-colored buttons that read as the clearest next step."},
			{ID: ThemeButtonThreadOutline, Label: "Thread Outline", Description: "Lighter outlined actions that keep more of the page surface visible."},
			{ID: ThemeButtonInkSolid, Label: "Ink Solid", Description: "High-contrast buttons with the sharpest dark-versus-light cut."},
		},
		ImageStyles: []ThemeOption{
			{ID: ThemeImageSoftFrame, Label: "Soft Frame", Description: "Calm framed image slots with the least visual drama."},
			{ID: ThemeImageWovenTint, Label: "Woven Tint", Description: "Tinted image panels that pull the ribbon palette into media areas."},
			{ID: ThemeImagePaperCut, Label: "Paper Cut", Description: "Layered paper-like image surfaces with a warmer collage feel."},
		},
	}
)

func DefaultThemeSelection() ThemeSelection {
	return ThemeSelection{
		Palette:        ThemePaletteCleanLocal,
		FontPreset:     ThemeFontBalanced,
		TypeScale:      ThemeTypeScaleBalanced,
		SectionSpacing: ThemeSpacingComfortable,
		ContentWidth:   ThemeContentWidthStandard,
		Radius:         ThemeRadiusSoft,
		ButtonStyle:    ThemeButtonRibbonFill,
		ImageStyle:     ThemeImageSoftFrame,
	}
}

func DefaultThemeEditorCatalog() ThemeEditorCatalog {
	return themeCatalog
}

func ThemeEditorCatalogWithBrand(brand BrandConfig) ThemeEditorCatalog {
	catalog := themeCatalog
	catalog.Palettes = make([]ThemeOption, len(themeCatalog.Palettes))
	for index, option := range themeCatalog.Palettes {
		selection := DefaultThemeSelection()
		selection.Palette = option.ID
		option.PreviewColors = BuildThemeWithBrand(selection, brand).Tokens.Colors
		catalog.Palettes[index] = option
	}
	return catalog
}

func ThemePreset(name string) ThemeConfig {
	switch name {
	case ThemePaletteBrightShopfront:
		return BuildTheme(ThemeSelection{
			Palette:        ThemePaletteBrightShopfront,
			FontPreset:     ThemeFontStudioSans,
			TypeScale:      ThemeTypeScaleExpressive,
			SectionSpacing: ThemeSpacingSnug,
			ContentWidth:   ThemeContentWidthWide,
			Radius:         ThemeRadiusPillowy,
			ButtonStyle:    ThemeButtonRibbonFill,
			ImageStyle:     ThemeImagePaperCut,
		})
	case ThemePaletteEditorialStudio:
		return BuildTheme(ThemeSelection{
			Palette:        ThemePaletteEditorialStudio,
			FontPreset:     ThemeFontEditorial,
			TypeScale:      ThemeTypeScaleExpressive,
			SectionSpacing: ThemeSpacingAiry,
			ContentWidth:   ThemeContentWidthFocused,
			Radius:         ThemeRadiusTailored,
			ButtonStyle:    ThemeButtonInkSolid,
			ImageStyle:     ThemeImageSoftFrame,
		})
	case ThemePaletteHeritageCraft:
		return BuildTheme(ThemeSelection{
			Palette:        ThemePaletteHeritageCraft,
			FontPreset:     ThemeFontBalanced,
			TypeScale:      ThemeTypeScaleBalanced,
			SectionSpacing: ThemeSpacingComfortable,
			ContentWidth:   ThemeContentWidthStandard,
			Radius:         ThemeRadiusSoft,
			ButtonStyle:    ThemeButtonThreadOutline,
			ImageStyle:     ThemeImageWovenTint,
		})
	case ThemePaletteAfterHours:
		return BuildTheme(ThemeSelection{
			Palette:        ThemePaletteAfterHours,
			FontPreset:     ThemeFontEditorial,
			TypeScale:      ThemeTypeScaleExpressive,
			SectionSpacing: ThemeSpacingComfortable,
			ContentWidth:   ThemeContentWidthStandard,
			Radius:         ThemeRadiusTailored,
			ButtonStyle:    ThemeButtonRibbonFill,
			ImageStyle:     ThemeImageWovenTint,
		})
	case ThemePaletteCalmNordic:
		return BuildTheme(ThemeSelection{
			Palette:        ThemePaletteCalmNordic,
			FontPreset:     ThemeFontBalanced,
			TypeScale:      ThemeTypeScaleBalanced,
			SectionSpacing: ThemeSpacingAiry,
			ContentWidth:   ThemeContentWidthFocused,
			Radius:         ThemeRadiusSoft,
			ButtonStyle:    ThemeButtonThreadOutline,
			ImageStyle:     ThemeImageSoftFrame,
		})
	default:
		return BuildTheme(ThemeSelection{
			Palette:        ThemePaletteCleanLocal,
			FontPreset:     ThemeFontBalanced,
			TypeScale:      ThemeTypeScaleBalanced,
			SectionSpacing: ThemeSpacingComfortable,
			ContentWidth:   ThemeContentWidthStandard,
			Radius:         ThemeRadiusSoft,
			ButtonStyle:    ThemeButtonRibbonFill,
			ImageStyle:     ThemeImageSoftFrame,
		})
	}
}

func BuildTheme(selection ThemeSelection) ThemeConfig {
	return BuildThemeWithBrand(selection, BrandConfig{})
}

// BuildThemeWithBrand builds the theme tokens and, when brand.primaryColor is
// set, overrides the palette's primary token with the brand color. Per
// [Spec 11](../../specs/11-theme-navigation-and-assets.md), brand is the
// source of the theme palette: the platform chooses preset families and the
// brand's primary color authoritatively becomes the rendered primary.
func BuildThemeWithBrand(selection ThemeSelection, brand BrandConfig) ThemeConfig {
	normalized := normalizeThemeSelection(selection)
	colors := maps.Clone(themePaletteTokens[normalized.Palette])
	if brand.PrimaryColor != "" && hexColorPattern.MatchString(brand.PrimaryColor) {
		colors = deriveBrandPalette(colors, brand.PrimaryColor)
	}
	sectionPaddingY := themeSectionSpacingValues[normalized.SectionSpacing]
	typography := maps.Clone(themeTypographyTokens[normalized.FontPreset])
	typography["scale"] = themeTypeScaleValues[normalized.TypeScale]
	return ThemeConfig{
		Version: ThemeVersionV1,
		Tokens: ThemeTokens{
			Colors:     colors,
			Typography: typography,
			Layout: map[string]any{
				"maxWidth":        "1120px",
				"contentWidth":    themeContentWidthValues[normalized.ContentWidth],
				"sectionPaddingX": "24px",
				"sectionPaddingY": sectionPaddingY,
			},
			Shape: map[string]any{
				"radius":      themeRadiusValues[normalized.Radius],
				"shadow":      "soft",
				"buttonStyle": normalized.ButtonStyle,
				"imageStyle":  normalized.ImageStyle,
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
	if typeScale := detectThemeTypeScale(theme.Tokens.Typography); typeScale != "" {
		selection.TypeScale = typeScale
	}
	if spacing := detectThemeSectionSpacing(theme.Tokens.Layout); spacing != "" {
		selection.SectionSpacing = spacing
	}
	if contentWidth := detectThemeContentWidth(theme.Tokens.Layout); contentWidth != "" {
		selection.ContentWidth = contentWidth
	}
	if radius := detectThemeRadius(theme.Tokens.Shape); radius != "" {
		selection.Radius = radius
	}
	if buttonStyle := detectThemeButtonStyle(theme.Tokens.Shape); buttonStyle != "" {
		selection.ButtonStyle = buttonStyle
	}
	if imageStyle := detectThemeImageStyle(theme.Tokens.Shape); imageStyle != "" {
		selection.ImageStyle = imageStyle
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
	if _, ok := themeTypeScaleValues[normalized.TypeScale]; !ok {
		normalized.TypeScale = DefaultThemeSelection().TypeScale
	}
	if _, ok := themeSectionSpacingValues[normalized.SectionSpacing]; !ok {
		normalized.SectionSpacing = DefaultThemeSelection().SectionSpacing
	}
	if _, ok := themeContentWidthValues[normalized.ContentWidth]; !ok {
		normalized.ContentWidth = DefaultThemeSelection().ContentWidth
	}
	if _, ok := themeRadiusValues[normalized.Radius]; !ok {
		normalized.Radius = DefaultThemeSelection().Radius
	}
	if _, ok := themeButtonStyles[normalized.ButtonStyle]; !ok {
		normalized.ButtonStyle = DefaultThemeSelection().ButtonStyle
	}
	if _, ok := themeImageStyles[normalized.ImageStyle]; !ok {
		normalized.ImageStyle = DefaultThemeSelection().ImageStyle
	}
	return normalized
}

func detectThemePalette(colors map[string]string) string {
	for id, tokens := range themePaletteTokens {
		if sameStringMap(colors, tokens) {
			return id
		}
	}
	for id, tokens := range themePaletteTokens {
		if colors["background"] == tokens["background"] && colors["foreground"] == tokens["foreground"] {
			return id
		}
	}
	return ""
}

func detectThemeFontPreset(typography map[string]any) string {
	for id, tokens := range themeTypographyTokens {
		matches := true
		for key, value := range tokens {
			if typography[key] != value {
				matches = false
				break
			}
		}
		if matches {
			return id
		}
	}
	return ""
}

func detectThemeTypeScale(typography map[string]any) string {
	value, _ := typography["scale"].(string)
	for id, scale := range themeTypeScaleValues {
		if value == scale {
			return id
		}
	}
	return ""
}

func detectThemeSectionSpacing(layout map[string]any) string {
	value, _ := layout["sectionPaddingY"].(string)
	if value == "" {
		value, _ = layout["sectionSpacing"].(string)
	}
	for id, spacing := range themeSectionSpacingValues {
		if value == spacing {
			return id
		}
	}
	return ""
}

func detectThemeContentWidth(layout map[string]any) string {
	value, _ := layout["contentWidth"].(string)
	for id, width := range themeContentWidthValues {
		if value == width {
			return id
		}
	}
	return ""
}

func detectThemeRadius(shape map[string]any) string {
	value, _ := shape["radius"].(string)
	switch value {
	case "22px":
		return ThemeRadiusTailored
	case "28px":
		return ThemeRadiusSoft
	}
	for id, radius := range themeRadiusValues {
		if value == radius {
			return id
		}
	}
	return ""
}

func detectThemeButtonStyle(shape map[string]any) string {
	value, _ := shape["buttonStyle"].(string)
	if _, ok := themeButtonStyles[value]; ok {
		return value
	}
	return ""
}

func detectThemeImageStyle(shape map[string]any) string {
	value, _ := shape["imageStyle"].(string)
	if _, ok := themeImageStyles[value]; ok {
		return value
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

func deriveBrandPalette(base map[string]string, primaryHex string) map[string]string {
	primary, ok := parseHexColor(primaryHex)
	if !ok {
		return base
	}
	background, _ := parseHexColor(base["background"])
	foreground, _ := parseHexColor(base["foreground"])
	darkMode := luminance(background) < luminance(foreground)

	primaryHSL := rgbToHSL(primary)
	secondaryHSL := primaryHSL
	secondaryHSL.h = math.Mod(secondaryHSL.h+18+360, 360)
	secondaryHSL.s = clampFloat(secondaryHSL.s*0.82, 0.18, 0.88)
	secondaryHSL.l = clampFloat(primaryHSL.l+map[bool]float64{true: 0.10, false: -0.08}[darkMode], 0.16, 0.82)

	accentHSL := primaryHSL
	accentHSL.h = math.Mod(accentHSL.h-28+360, 360)
	accentHSL.s = clampFloat(primaryHSL.s*1.08, 0.22, 0.92)
	accentHSL.l = clampFloat(primaryHSL.l+map[bool]float64{true: 0.16, false: -0.04}[darkMode], 0.18, 0.86)

	surface := mixRGB(background, primary, map[bool]float64{true: 0.18, false: 0.08}[darkMode])
	surfaceMuted := mixRGB(background, primary, map[bool]float64{true: 0.28, false: 0.14}[darkMode])
	border := mixRGB(surfaceMuted, foreground, map[bool]float64{true: 0.24, false: 0.10}[darkMode])
	muted := mixRGB(primary, background, map[bool]float64{true: 0.42, false: 0.54}[darkMode])
	ring := mixRGB(primary, foreground, map[bool]float64{true: 0.28, false: 0.18}[darkMode])

	base["primary"] = formatHexColor(primary)
	base["secondary"] = formatHexColor(hslToRGB(secondaryHSL))
	base["accent"] = formatHexColor(hslToRGB(accentHSL))
	base["surface"] = formatHexColor(surface)
	base["surfaceMuted"] = formatHexColor(surfaceMuted)
	base["border"] = formatHexColor(border)
	base["muted"] = formatHexColor(muted)
	base["ring"] = formatHexColor(ring)
	return base
}

type rgbColor struct {
	r float64
	g float64
	b float64
}

type hslColor struct {
	h float64
	s float64
	l float64
}

func parseHexColor(value string) (rgbColor, bool) {
	var r int
	var g int
	var b int
	if _, err := fmt.Sscanf(value, "#%02x%02x%02x", &r, &g, &b); err == nil {
		return rgbColor{r: float64(r), g: float64(g), b: float64(b)}, true
	}
	return rgbColor{}, false
}

func formatHexColor(value rgbColor) string {
	return fmt.Sprintf("#%02x%02x%02x", clampChannel(value.r), clampChannel(value.g), clampChannel(value.b))
}

func mixRGB(left rgbColor, right rgbColor, weight float64) rgbColor {
	weight = clampFloat(weight, 0, 1)
	return rgbColor{
		r: left.r + ((right.r - left.r) * weight),
		g: left.g + ((right.g - left.g) * weight),
		b: left.b + ((right.b - left.b) * weight),
	}
}

func luminance(value rgbColor) float64 {
	return (0.2126 * value.r) + (0.7152 * value.g) + (0.0722 * value.b)
}

func rgbToHSL(value rgbColor) hslColor {
	r := value.r / 255
	g := value.g / 255
	b := value.b / 255
	maxValue := math.Max(r, math.Max(g, b))
	minValue := math.Min(r, math.Min(g, b))
	lightness := (maxValue + minValue) / 2
	if maxValue == minValue {
		return hslColor{l: lightness}
	}

	delta := maxValue - minValue
	saturation := delta / (1 - math.Abs((2*lightness)-1))
	var hue float64
	switch maxValue {
	case r:
		hue = math.Mod(((g - b) / delta), 6)
	case g:
		hue = ((b - r) / delta) + 2
	default:
		hue = ((r - g) / delta) + 4
	}
	return hslColor{h: 60 * hue, s: saturation, l: lightness}
}

func hslToRGB(value hslColor) rgbColor {
	chroma := (1 - math.Abs((2*value.l)-1)) * value.s
	segment := value.h / 60
	x := chroma * (1 - math.Abs(math.Mod(segment, 2)-1))

	var rPrime float64
	var gPrime float64
	var bPrime float64
	switch {
	case segment >= 0 && segment < 1:
		rPrime, gPrime = chroma, x
	case segment >= 1 && segment < 2:
		rPrime, gPrime = x, chroma
	case segment >= 2 && segment < 3:
		gPrime, bPrime = chroma, x
	case segment >= 3 && segment < 4:
		gPrime, bPrime = x, chroma
	case segment >= 4 && segment < 5:
		rPrime, bPrime = x, chroma
	default:
		rPrime, bPrime = chroma, x
	}

	match := value.l - (chroma / 2)
	return rgbColor{
		r: (rPrime + match) * 255,
		g: (gPrime + match) * 255,
		b: (bPrime + match) * 255,
	}
}

func clampFloat(value float64, minValue float64, maxValue float64) float64 {
	return math.Min(math.Max(value, minValue), maxValue)
}

func clampChannel(value float64) int {
	return int(math.Round(clampFloat(value, 0, 255)))
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
