package generation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

var (
	generatedDangerousBlockPattern = regexp.MustCompile(`(?is)<\s*(script|style|iframe|object|embed|svg|form|input|button|textarea|select)\b[^>]*>.*?(?:<\s*/\s*[a-z]+\s*>|$)`)
	generatedHTMLTagPattern        = regexp.MustCompile(`(?is)</?[a-z][^>]*>`)
	generatedWhitespacePattern     = regexp.MustCompile(`\s+`)
	generatedSlugPattern           = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	generatedSlugReplacer          = regexp.MustCompile(`[^a-z0-9]+`)
)

func repairGenerationPlan(plan generationPlan) generationPlan {
	repaired := generationPlan{
		SiteName:     firstNonEmpty(cleanGeneratedText(plan.SiteName, 120), "Small Good Studio"),
		SiteGoal:     firstNonEmpty(cleanGeneratedText(plan.SiteGoal, 180), siteGoalForCategory("business")),
		ThemePreset:  normalizeGeneratedThemePreset(plan.ThemePreset),
		AssetsNeeded: repairAssetsNeeded(plan.AssetsNeeded),
		Assumptions:  repairAssumptions(plan.Assumptions),
	}

	repaired.Theme = repairTheme(repaired.ThemePreset, plan.Theme)
	repaired.Pages = repairPages(repaired.SiteName, repaired.SiteGoal, plan.Pages)
	return repaired
}

func normalizeGeneratedThemePreset(value string) string {
	switch strings.TrimSpace(value) {
	case siteconfig.ThemePalettePlayfulRibbon:
		return siteconfig.ThemePalettePlayfulRibbon
	case siteconfig.ThemePaletteMeanerDark:
		return siteconfig.ThemePaletteMeanerDark
	default:
		return siteconfig.ThemePaletteCalmNordic
	}
}

func repairTheme(preset string, theme siteconfig.ThemeConfig) siteconfig.ThemeConfig {
	if generatedThemeIsValid(theme) {
		return theme
	}
	return siteconfig.ThemePreset(preset)
}

func generatedThemeIsValid(theme siteconfig.ThemeConfig) bool {
	draft := siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:            "site_generation_guardrails",
			Name:          "Generation Guardrails",
			Slug:          "generation-guardrails",
			Status:        "draft",
			DefaultLocale: "en",
		},
		Theme: theme,
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page_home"}},
		},
		Pages: []siteconfig.PageDraft{
			{
				ID:     "page_home",
				Title:  "Home",
				Slug:   "/",
				Blocks: []siteconfig.BlockInstance{},
			},
		},
	}
	return siteconfig.ValidateDraft(draft) == nil
}

func repairAssetsNeeded(values []string) []string {
	allowed := map[string]bool{
		"hero-image":       true,
		"supporting-image": true,
	}
	seen := map[string]bool{}
	repaired := make([]string, 0, len(values))
	for _, value := range values {
		clean := strings.TrimSpace(value)
		if !allowed[clean] || seen[clean] {
			continue
		}
		seen[clean] = true
		repaired = append(repaired, clean)
	}
	return repaired
}

func repairAssumptions(values []string) []string {
	repaired := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		clean := cleanGeneratedText(value, 240)
		if clean == "" || seen[clean] {
			continue
		}
		seen[clean] = true
		repaired = append(repaired, clean)
		if len(repaired) == 8 {
			break
		}
	}
	return repaired
}

func repairPages(siteName string, siteGoal string, pages []generationPagePlan) []generationPagePlan {
	if len(pages) == 0 {
		return []generationPagePlan{fallbackHomePage(siteName, siteGoal)}
	}

	repaired := make([]generationPagePlan, 0, min(len(pages), siteconfig.MaxPagesPerSite))
	usedSlugs := map[string]bool{"/": true}

	homeSource := pages[0]
	homeIndex := 0
	for index, page := range pages {
		if strings.TrimSpace(page.Slug) == "/" {
			homeSource = page
			homeIndex = index
			break
		}
	}
	repaired = append(repaired, repairHomePage(siteName, siteGoal, homeSource))

	for index, page := range pages {
		if len(repaired) == siteconfig.MaxPagesPerSite {
			break
		}
		if index == homeIndex {
			continue
		}
		repaired = append(repaired, repairSecondaryPage(siteName, siteGoal, page, usedSlugs))
	}

	return repaired
}

func repairHomePage(siteName string, siteGoal string, page generationPagePlan) generationPagePlan {
	title := firstNonEmpty(cleanGeneratedText(page.Title, 120), "Home")
	blocks := repairBlocks(title, siteGoal, "/", page.Blocks)
	description := firstNonEmpty(
		cleanGeneratedText(page.SEO.Description, 180),
		cleanGeneratedText(page.Goal, 180),
		extractPageDescription(blocks),
		siteGoal,
	)

	return generationPagePlan{
		Title: "Home",
		Slug:  "/",
		Goal:  firstNonEmpty(cleanGeneratedText(page.Goal, 180), siteGoal),
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence(firstNonEmpty(cleanGeneratedText(page.SEO.Title, 70), siteName), 70),
			Description: clampSentence(description, 180),
		},
		Blocks: blocks,
	}
}

func repairSecondaryPage(siteName string, siteGoal string, page generationPagePlan, usedSlugs map[string]bool) generationPagePlan {
	title := firstNonEmpty(cleanGeneratedText(page.Title, 120), "Page")
	slug := uniqueGeneratedPageSlug(page.Slug, title, usedSlugs)
	blocks := repairBlocks(title, siteGoal, slug, page.Blocks)
	description := firstNonEmpty(
		cleanGeneratedText(page.SEO.Description, 180),
		cleanGeneratedText(page.Goal, 180),
		extractPageDescription(blocks),
		siteGoal,
	)

	return generationPagePlan{
		Title: title,
		Slug:  slug,
		Goal:  firstNonEmpty(cleanGeneratedText(page.Goal, 180), siteGoal),
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence(firstNonEmpty(cleanGeneratedText(page.SEO.Title, 70), fmt.Sprintf("%s | %s", title, siteName)), 70),
			Description: clampSentence(description, 180),
		},
		Blocks: blocks,
	}
}

func uniqueGeneratedPageSlug(raw string, title string, used map[string]bool) string {
	base := normalizedGeneratedPageSlug(raw, title)
	slug := base
	index := 2
	for used[slug] {
		slug = fmt.Sprintf("%s-%d", base, index)
		index++
	}
	used[slug] = true
	return slug
}

func normalizedGeneratedPageSlug(raw string, title string) string {
	slug := strings.TrimSpace(raw)
	if slug == "/" {
		slug = ""
	}
	slug = strings.TrimPrefix(slug, "/")
	if slug == "" || !isGeneratedSlugSafe(slug) {
		slug = slugsCandidateFromTitle(title)
	}
	if slug == "" {
		slug = "page"
	}
	return "/" + slug
}

func isGeneratedSlugSafe(value string) bool {
	if value == "" || strings.Contains(value, "/") {
		return false
	}
	return generatedSlugPattern.MatchString(value)
}

func slugsCandidateFromTitle(title string) string {
	text := strings.ToLower(cleanGeneratedText(title, 120))
	text = generatedSlugReplacer.ReplaceAllString(text, "-")
	text = strings.Trim(text, "-")
	return text
}

func repairBlocks(pageTitle string, pageGoal string, pageSlug string, blocks []generationBlockPlan) []generationBlockPlan {
	registry := siteconfig.DefaultBlockRegistry()
	repaired := make([]generationBlockPlan, 0, len(blocks))

	for _, block := range blocks {
		next, ok := repairBlockPlan(block, pageTitle, pageGoal, pageSlug)
		if !ok {
			continue
		}
		if err := registry.ValidateProps(next.Type, siteconfig.BlockVersionV1, "props", next.Props); err != nil {
			continue
		}
		repaired = append(repaired, next)
	}

	if len(repaired) > 0 {
		return repaired
	}

	fallback := generationBlockPlan{
		Type:    "text_section",
		Purpose: "Fallback content block",
		Props: map[string]any{
			"heading":   pageTitle,
			"body":      firstNonEmpty(pageGoal, "Add focused page content here."),
			"alignment": "left",
			"width":     "default",
		},
	}
	return []generationBlockPlan{fallback}
}

func repairBlockPlan(block generationBlockPlan, pageTitle string, pageGoal string, pageSlug string) (generationBlockPlan, bool) {
	switch strings.TrimSpace(block.Type) {
	case "hero":
		props := repairHeroProps(block.Props, pageTitle, pageSlug)
		return generationBlockPlan{Type: "hero", Purpose: cleanGeneratedText(block.Purpose, 280), Props: props}, true
	case "text_section":
		props := repairTextSectionProps(block.Props, pageTitle, pageGoal)
		return generationBlockPlan{Type: "text_section", Purpose: cleanGeneratedText(block.Purpose, 280), Props: props}, true
	case "image_text":
		props := repairImageTextProps(block.Props, pageTitle, pageGoal)
		return generationBlockPlan{Type: "image_text", Purpose: cleanGeneratedText(block.Purpose, 280), Props: props}, true
	case "features_grid":
		props := repairFeaturesGridProps(block.Props, pageTitle, pageGoal)
		return generationBlockPlan{Type: "features_grid", Purpose: cleanGeneratedText(block.Purpose, 280), Props: props}, true
	case "cta_band":
		props := repairCTABandProps(block.Props, pageTitle, pageSlug)
		return generationBlockPlan{Type: "cta_band", Purpose: cleanGeneratedText(block.Purpose, 280), Props: props}, true
	default:
		return generationBlockPlan{}, false
	}
}

func repairHeroProps(props map[string]any, pageTitle string, pageSlug string) map[string]any {
	repaired := map[string]any{
		"headline": firstNonEmpty(readGeneratedText(props, "headline", 120), pageTitle),
		"layout":   readEnum(props, "layout", "split-left", "centered", "split-left", "split-right"),
	}
	if eyebrow := readGeneratedText(props, "eyebrow", 80); eyebrow != "" {
		repaired["eyebrow"] = eyebrow
	}
	if subheadline := readGeneratedText(props, "subheadline", 280); subheadline != "" {
		repaired["subheadline"] = subheadline
	}
	if cta := repairCTA(props["primaryCta"], "Get in touch", fallbackGeneratedCTAHref(pageSlug)); cta != nil {
		repaired["primaryCta"] = cta
	}
	if cta := repairCTA(props["secondaryCta"], "Learn more", "/"); cta != nil {
		repaired["secondaryCta"] = cta
	}
	if image := repairImage(props["image"]); image != nil {
		repaired["image"] = image
	}
	return repaired
}

func repairTextSectionProps(props map[string]any, pageTitle string, pageGoal string) map[string]any {
	return map[string]any{
		"heading":   firstNonEmpty(readGeneratedText(props, "heading", 120), pageTitle),
		"body":      firstNonEmpty(readGeneratedText(props, "body", 4000), pageGoal, "Add focused supporting copy here."),
		"alignment": readEnum(props, "alignment", "left", "left", "center", "right"),
		"width":     readEnum(props, "width", "default", "narrow", "default", "wide"),
	}
}

func repairImageTextProps(props map[string]any, pageTitle string, pageGoal string) map[string]any {
	repaired := map[string]any{
		"heading":       firstNonEmpty(readGeneratedText(props, "heading", 120), pageTitle),
		"body":          firstNonEmpty(readGeneratedText(props, "body", 2500), pageGoal, "Pair a short message with a supporting image."),
		"imagePosition": readEnum(props, "imagePosition", "right", "left", "right"),
	}
	if image := repairImage(props["image"]); image != nil {
		repaired["image"] = image
	}
	if cta := repairCTA(props["cta"], "Get in touch", "/contact"); cta != nil {
		repaired["cta"] = cta
	}
	return repaired
}

func repairFeaturesGridProps(props map[string]any, pageTitle string, pageGoal string) map[string]any {
	items := make([]any, 0)
	if rawItems, ok := props["items"].([]any); ok {
		for _, raw := range rawItems {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			title := readGeneratedText(item, "title", 80)
			body := readGeneratedText(item, "body", 280)
			if title == "" || body == "" {
				continue
			}
			next := map[string]any{
				"title": title,
				"body":  body,
			}
			if icon := readGeneratedText(item, "icon", 40); icon != "" {
				next["icon"] = icon
			}
			items = append(items, next)
			if len(items) == 12 {
				break
			}
		}
	}
	if len(items) == 0 {
		items = append(items, map[string]any{
			"title": firstNonEmpty(pageTitle, "What you get"),
			"body":  firstNonEmpty(pageGoal, "Add a concise benefit statement."),
		})
	}

	return map[string]any{
		"heading": firstNonEmpty(readGeneratedText(props, "heading", 120), pageTitle),
		"intro":   readGeneratedText(props, "intro", 500),
		"items":   items,
		"columns": readIntEnum(props, "columns", 3, 2, 3, 4),
	}
}

func repairCTABandProps(props map[string]any, pageTitle string, pageSlug string) map[string]any {
	repaired := map[string]any{
		"heading": firstNonEmpty(readGeneratedText(props, "heading", 120), pageTitle),
		"body":    firstNonEmpty(readGeneratedText(props, "body", 600), "Invite the visitor into the clearest next step."),
		"variant": readEnum(props, "variant", "primary", "primary", "secondary", "accent"),
	}
	if cta := repairCTA(props["cta"], "Get in touch", fallbackGeneratedCTAHref(pageSlug)); cta != nil {
		repaired["cta"] = cta
	}
	return repaired
}

func repairCTA(value any, fallbackLabel string, fallbackHref string) map[string]any {
	object, ok := value.(map[string]any)
	if !ok {
		if fallbackLabel == "" || fallbackHref == "" {
			return nil
		}
		return map[string]any{
			"label": fallbackLabel,
			"href":  fallbackHref,
		}
	}

	label := firstNonEmpty(readGeneratedText(object, "label", 40), fallbackLabel)
	href := readSafeURL(object, "href", fallbackHref)
	if label == "" || href == "" {
		return nil
	}
	return map[string]any{
		"label": label,
		"href":  href,
	}
}

func repairImage(value any) map[string]any {
	object, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	assetID := readGeneratedText(object, "assetId", 120)
	if assetID == "" {
		return nil
	}
	repaired := map[string]any{"assetId": assetID}
	if alt := readGeneratedText(object, "alt", 180); alt != "" {
		repaired["alt"] = alt
	}
	return repaired
}

func fallbackGeneratedCTAHref(pageSlug string) string {
	if pageSlug == "/contact" {
		return "mailto:hello@example.com"
	}
	return "/contact"
}

func fallbackHomePage(siteName string, siteGoal string) generationPagePlan {
	return generationPagePlan{
		Title: "Home",
		Slug:  "/",
		Goal:  firstNonEmpty(siteGoal, "Turn visitors into clear, low-friction inquiries."),
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence(siteName, 70),
			Description: clampSentence(siteGoal, 180),
		},
		Blocks: []generationBlockPlan{
			{
				Type:    "hero",
				Purpose: "Fallback homepage block",
				Props: map[string]any{
					"headline": firstNonEmpty(siteName, "Small Good Studio"),
					"layout":   "centered",
					"primaryCta": map[string]any{
						"label": "Get in touch",
						"href":  "/contact",
					},
				},
			},
			{
				Type:    "text_section",
				Purpose: "Fallback homepage copy",
				Props: map[string]any{
					"heading":   "A focused first draft",
					"body":      firstNonEmpty(siteGoal, "Add concise supporting copy here."),
					"alignment": "left",
					"width":     "default",
				},
			},
		},
	}
}

func extractPageDescription(blocks []generationBlockPlan) string {
	for _, block := range blocks {
		switch block.Type {
		case "hero":
			if text := readGeneratedText(block.Props, "subheadline", 180); text != "" {
				return text
			}
			if text := readGeneratedText(block.Props, "headline", 180); text != "" {
				return text
			}
		case "text_section", "image_text", "cta_band":
			if text := readGeneratedText(block.Props, "body", 180); text != "" {
				return text
			}
		case "features_grid":
			if text := readGeneratedText(block.Props, "intro", 180); text != "" {
				return text
			}
		}
	}
	return ""
}

func readGeneratedText(props map[string]any, key string, limit int) string {
	if props == nil {
		return ""
	}
	value, _ := props[key].(string)
	return cleanGeneratedText(value, limit)
}

func readEnum(props map[string]any, key string, fallback string, allowed ...string) string {
	if props != nil {
		if value, ok := props[key].(string); ok {
			value = strings.TrimSpace(value)
			for _, candidate := range allowed {
				if value == candidate {
					return value
				}
			}
		}
	}
	return fallback
}

func readIntEnum(props map[string]any, key string, fallback int, allowed ...int) int {
	if props != nil {
		switch value := props[key].(type) {
		case int:
			for _, candidate := range allowed {
				if value == candidate {
					return value
				}
			}
		case float64:
			integer := int(value)
			if value == float64(integer) {
				for _, candidate := range allowed {
					if integer == candidate {
						return integer
					}
				}
			}
		}
	}
	return fallback
}

func readSafeURL(props map[string]any, key string, fallback string) string {
	if props != nil {
		if value, ok := props[key].(string); ok {
			value = strings.TrimSpace(value)
			if siteconfig.ValidateURL(value) == nil {
				return value
			}
		}
	}
	if siteconfig.ValidateURL(fallback) == nil {
		return fallback
	}
	return ""
}

func cleanGeneratedText(value string, limit int) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	text = generatedDangerousBlockPattern.ReplaceAllString(text, " ")
	text = generatedHTMLTagPattern.ReplaceAllString(text, " ")
	text = generatedWhitespacePattern.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	return clampSentence(text, limit)
}

func min(left int, right int) int {
	if left < right {
		return left
	}
	return right
}
