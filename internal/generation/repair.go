package generation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/MattiSig/snaelda/internal/platform/ids"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

var (
	generatedDangerousBlockPattern = regexp.MustCompile(`(?is)<\s*(script|style|iframe|object|embed|svg|form|input|button|textarea|select)\b[^>]*>.*?(?:<\s*/\s*[a-z]+\s*>|$)`)
	generatedHTMLTagPattern        = regexp.MustCompile(`(?is)</?[a-z][^>]*>`)
	generatedWhitespacePattern     = regexp.MustCompile(`\s+`)
	generatedSlugPattern           = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	generatedSlugReplacer          = regexp.MustCompile(`[^a-z0-9]+`)
)

// repairGenerationPlan normalizes a generated plan into a valid draft.
//
// dropEmptyRepeaters controls how repeater blocks (testimonials, features,
// gallery, pricing, faq, team) with no real items are handled: on a fresh
// first-generation or re-spin draft (true) the empty block is dropped entirely
// rather than shipping a meta-instruction placeholder the visitor would see; on
// an in-builder reprompt (false) the placeholder is kept so the owner sees the
// section exists and can fill it in.
func repairGenerationPlan(plan generationPlan, locale string, dropEmptyRepeaters bool) generationPlan {
	themeSelection := siteconfig.ThemeSelection{}
	if hasThemeSelection(plan.ThemeSelection) {
		themeSelection = normalizeThemeSelection(plan.ThemeSelection)
	}

	repaired := generationPlan{
		SiteName:       firstNonEmpty(cleanGeneratedText(plan.SiteName, 120), "Small Good Studio"),
		SiteGoal:       firstNonEmpty(cleanGeneratedText(plan.SiteGoal, 180), siteGoalForCategory(categoryBusiness, locale)),
		ThemePreset:    normalizeGeneratedThemePreset(firstNonEmpty(plan.ThemeSelection.Palette, plan.ThemePreset)),
		ThemeSelection: themeSelection,
		AssetsNeeded:   repairAssetsNeeded(plan.AssetsNeeded),
		Assumptions:    repairAssumptions(plan.Assumptions),
	}

	repaired.Theme = repairTheme(repaired.ThemePreset, repaired.ThemeSelection, plan.Theme)
	repaired.Pages = repairPages(repaired.SiteName, repaired.SiteGoal, plan.Pages, dropEmptyRepeaters)
	if dropEmptyRepeaters {
		// dropEmptyRepeaters marks a fresh first-generation or re-spin draft,
		// where the whole page set is present. That is the only safe moment to
		// run the whole-site copy-polish pass: an in-builder reprompt carries a
		// partial plan, so link validation would false-positive.
		repaired.Pages = polishGeneratedPages(repaired.Pages)
	}
	return repaired
}

// polishGeneratedPages runs deterministic, whole-site copy hygiene on a freshly
// generated plan. It removes short supporting strings that repeat verbatim
// across the site (a hero subheadline reused as the footer tagline reads as
// filler), and rewrites internal CTA links that point at a page or section
// anchor that does not exist so a "Get in touch" button can never dead-end.
func polishGeneratedPages(pages []generationPagePlan) []generationPagePlan {
	slugs := make(map[string]bool, len(pages))
	for _, page := range pages {
		slug := strings.TrimSpace(page.Slug)
		if slug == "" {
			slug = "/"
		}
		slugs[slug] = true
	}
	fallback := ctaFallbackHref(slugs)

	seenSupporting := map[string]bool{}
	for pageIndex := range pages {
		for _, block := range pages[pageIndex].Blocks {
			// block is a copy, but block.Props is a shared map reference, so
			// mutating and deleting entries updates the underlying plan.
			retargetBlockCTAs(block.Props, slugs, fallback)
			dedupeSupportingCopy(block, seenSupporting)
		}
	}
	return pages
}

// ctaFallbackHref picks the best real destination for a broken CTA, preferring a
// contact page (English or Icelandic slug) and falling back to the home page,
// which always exists.
func ctaFallbackHref(slugs map[string]bool) string {
	for _, candidate := range []string{"/contact", "/hafa-samband"} {
		if slugs[candidate] {
			return candidate
		}
	}
	return "/"
}

// retargetBlockCTAs repoints every CTA on a block whose href cannot resolve.
func retargetBlockCTAs(props map[string]any, slugs map[string]bool, fallback string) {
	if props == nil {
		return
	}
	for _, key := range []string{"primaryCta", "secondaryCta", "cta"} {
		cta, ok := props[key].(map[string]any)
		if !ok {
			continue
		}
		href, ok := cta["href"].(string)
		if !ok {
			continue
		}
		if resolved, changed := resolveInternalHref(href, slugs, fallback); changed {
			cta["href"] = resolved
		}
	}
}

// resolveInternalHref rewrites an internal CTA href that cannot reach a real
// destination. External, mailto:, and tel: links pass through untouched. A link
// to a page that does not exist, or to a section anchor (the renderer only ever
// emits page-level anchors, so "#contact"/"/#contact" never resolve), is
// redirected to the fallback page so the button always lands somewhere real.
func resolveInternalHref(href string, slugs map[string]bool, fallback string) (string, bool) {
	trimmed := strings.TrimSpace(href)
	if trimmed == "" {
		return fallback, fallback != ""
	}
	if !strings.HasPrefix(trimmed, "/") && !strings.HasPrefix(trimmed, "#") {
		return trimmed, false
	}
	if strings.Contains(trimmed, "#") {
		if trimmed == fallback {
			return trimmed, false
		}
		return fallback, true
	}
	if slugs[trimmed] {
		return trimmed, false
	}
	if trimmed == fallback {
		return trimmed, false
	}
	return fallback, true
}

// dedupeSupportingCopy drops a hero subheadline or footer tagline that repeats a
// supporting line already used elsewhere on the site. A unique footer tagline is
// intentionally not recorded as "seen": the footer repeats on every page and its
// tagline is allowed to stand as the site's one closing line.
func dedupeSupportingCopy(block generationBlockPlan, seen map[string]bool) {
	switch block.Type {
	case "hero":
		key := normalizeSupportingCopy(readString(block.Props, "subheadline"))
		if key == "" {
			return
		}
		if seen[key] {
			delete(block.Props, "subheadline")
			return
		}
		seen[key] = true
	case "footer":
		key := normalizeSupportingCopy(readString(block.Props, "tagline"))
		if key == "" {
			return
		}
		if seen[key] {
			delete(block.Props, "tagline")
		}
	}
}

func normalizeSupportingCopy(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

func readString(props map[string]any, key string) string {
	if props == nil {
		return ""
	}
	value, _ := props[key].(string)
	return value
}

func normalizeGeneratedThemePreset(value string) string {
	switch strings.TrimSpace(value) {
	case siteconfig.ThemePaletteCalmNordic:
		return siteconfig.ThemePaletteCalmNordic
	case siteconfig.ThemePaletteCleanLocal:
		return siteconfig.ThemePaletteCleanLocal
	case siteconfig.ThemePaletteBrightShopfront:
		return siteconfig.ThemePaletteBrightShopfront
	case siteconfig.ThemePaletteEditorialStudio:
		return siteconfig.ThemePaletteEditorialStudio
	case siteconfig.ThemePaletteHeritageCraft:
		return siteconfig.ThemePaletteHeritageCraft
	case siteconfig.ThemePaletteAfterHours:
		return siteconfig.ThemePaletteAfterHours
	default:
		return siteconfig.ThemePaletteCleanLocal
	}
}

func repairTheme(preset string, selection siteconfig.ThemeSelection, theme siteconfig.ThemeConfig) siteconfig.ThemeConfig {
	if selection.Palette != "" {
		return siteconfig.BuildTheme(selection)
	}
	if generatedThemeIsValid(theme) {
		return theme
	}
	return siteconfig.ThemePreset(preset)
}

func normalizeThemeSelection(selection siteconfig.ThemeSelection) siteconfig.ThemeSelection {
	return siteconfig.DetectThemeSelection(siteconfig.BuildTheme(selection))
}

func hasThemeSelection(selection siteconfig.ThemeSelection) bool {
	return selection.Palette != "" ||
		selection.FontPreset != "" ||
		selection.TypeScale != "" ||
		selection.SectionSpacing != "" ||
		selection.ContentWidth != "" ||
		selection.Radius != "" ||
		selection.ButtonStyle != "" ||
		selection.ImageStyle != ""
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

func repairPages(siteName string, siteGoal string, pages []generationPagePlan, dropEmptyRepeaters bool) []generationPagePlan {
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
	repaired = append(repaired, repairHomePage(siteName, siteGoal, homeSource, dropEmptyRepeaters))

	for index, page := range pages {
		if len(repaired) == siteconfig.MaxPagesPerSite {
			break
		}
		if index == homeIndex {
			continue
		}
		repaired = append(repaired, repairSecondaryPage(siteName, siteGoal, page, usedSlugs, dropEmptyRepeaters))
	}

	return repaired
}

func repairHomePage(siteName string, siteGoal string, page generationPagePlan, dropEmptyRepeaters bool) generationPagePlan {
	title := firstNonEmpty(cleanGeneratedText(page.Title, 120), "Home")
	blocks := repairBlocks(title, siteGoal, "/", page.Blocks, dropEmptyRepeaters)
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

func repairSecondaryPage(siteName string, siteGoal string, page generationPagePlan, usedSlugs map[string]bool, dropEmptyRepeaters bool) generationPagePlan {
	title := firstNonEmpty(cleanGeneratedText(page.Title, 120), "Page")
	slug := uniqueGeneratedPageSlug(page.Slug, title, usedSlugs)
	blocks := repairBlocks(title, siteGoal, slug, page.Blocks, dropEmptyRepeaters)
	blocks = repairIntendedContactFormBlock(title, page.Goal, blocks)
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

func repairBlocks(pageTitle string, pageGoal string, pageSlug string, blocks []generationBlockPlan, dropEmptyRepeaters bool) []generationBlockPlan {
	registry := siteconfig.DefaultBlockRegistry()
	repaired := make([]generationBlockPlan, 0, len(blocks))

	for _, block := range blocks {
		next, ok := repairBlockPlan(block, pageTitle, pageGoal, pageSlug, dropEmptyRepeaters)
		if !ok {
			continue
		}
		if err := registry.ValidateProps(next.Type, siteconfig.LatestBlockVersion(next.Type), "props", next.Props); err != nil {
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

func repairIntendedContactFormBlock(pageTitle string, pageGoal string, blocks []generationBlockPlan) []generationBlockPlan {
	if generatedBlocksIncludeType(blocks, "contact_form") {
		return blocks
	}

	for index, block := range blocks {
		if !generatedBlockLooksLikeContactFormIntent(block) {
			continue
		}
		next := append([]generationBlockPlan(nil), blocks...)
		next[index] = contactFormBlockFromIntent(block, pageTitle)
		return next
	}

	if !generatedTextMentionsContactForm(pageGoal) {
		return blocks
	}

	form := contactFormBlockFromIntent(generationBlockPlan{}, pageTitle)
	insertAt := len(blocks)
	for index, block := range blocks {
		if block.Type == "footer" {
			insertAt = index
			break
		}
	}
	next := make([]generationBlockPlan, 0, len(blocks)+1)
	next = append(next, blocks[:insertAt]...)
	next = append(next, form)
	next = append(next, blocks[insertAt:]...)
	return next
}

func contactFormBlockFromIntent(block generationBlockPlan, pageTitle string) generationBlockPlan {
	props := map[string]any{
		"heading":     firstNonEmpty(readGeneratedText(block.Props, "heading", 120), "Send a quick inquiry"),
		"intro":       firstNonEmpty(readGeneratedText(block.Props, "intro", 500), readGeneratedText(block.Props, "body", 500), "Share a few details and we will get back to you shortly."),
		"submitLabel": "Send inquiry",
	}
	return generationBlockPlan{
		Type:    "contact_form",
		Purpose: firstNonEmpty(cleanGeneratedText(block.Purpose, 280), "Give ready visitors a direct inquiry form."),
		Props:   repairContactFormProps(props, pageTitle),
	}
}

func generatedBlockLooksLikeContactFormIntent(block generationBlockPlan) bool {
	if block.Type == "contact_form" || block.Type == "footer" {
		return false
	}
	if generatedTextMentionsContactForm(block.Purpose) {
		return true
	}
	for _, key := range []string{"heading", "title", "intro", "body"} {
		if generatedTextMentionsContactForm(readGeneratedText(block.Props, key, 500)) {
			return true
		}
	}
	return false
}

func generatedTextMentionsContactForm(value string) bool {
	text := strings.ToLower(strings.TrimSpace(value))
	for _, marker := range contactFormIntentMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// contactFormIntentMarkers are phrases that signal a block is standing in for a
// real contact form. Kept to explicit form-action language ("use this form",
// "request service") rather than generic navigation CTAs ("get in touch",
// "contact us") so a cta_band that merely links to a contact page is not
// rewritten into an inline form. Includes Icelandic form phrasing for the
// Iceland-first re-spin path.
var contactFormIntentMarkers = []string{
	"contact form",
	"kontaktform",
	"inquiry form",
	"enquiry form",
	"message form",
	"use this form",
	"use the form",
	"fill out the form",
	"fill out this form",
	"fill in the form",
	"request service",
	"request a quote",
	"request a callback",
	"send us a message",
	"send a message",
	"eyðublað",
	"fylltu út",
}

func generatedBlocksIncludeType(blocks []generationBlockPlan, blockType string) bool {
	for _, block := range blocks {
		if block.Type == blockType {
			return true
		}
	}
	return false
}

func repairBlockPlan(block generationBlockPlan, pageTitle string, pageGoal string, pageSlug string, dropEmptyRepeaters bool) (generationBlockPlan, bool) {
	// On a fresh generation an empty repeater block should be dropped rather
	// than shipped with a meta-instruction placeholder; on an in-builder
	// reprompt the placeholder is kept so the section stays visible to edit.
	allowPlaceholder := !dropEmptyRepeaters
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
		props, keep := repairFeaturesGridProps(block.Props, pageTitle, pageGoal, allowPlaceholder)
		if !keep {
			return generationBlockPlan{}, false
		}
		return generationBlockPlan{Type: "features_grid", Purpose: cleanGeneratedText(block.Purpose, 280), Props: props}, true
	case "gallery":
		props, keep := repairGalleryProps(block.Props, pageTitle, pageGoal, allowPlaceholder)
		if !keep {
			return generationBlockPlan{}, false
		}
		return generationBlockPlan{Type: "gallery", Purpose: cleanGeneratedText(block.Purpose, 280), Props: props}, true
	case "testimonials":
		props, keep := repairTestimonialsProps(block.Props, pageTitle, allowPlaceholder)
		if !keep {
			return generationBlockPlan{}, false
		}
		return generationBlockPlan{Type: "testimonials", Purpose: cleanGeneratedText(block.Purpose, 280), Props: props}, true
	case "pricing_packages":
		props, keep := repairPricingPackagesProps(block.Props, pageTitle, pageSlug, allowPlaceholder)
		if !keep {
			return generationBlockPlan{}, false
		}
		return generationBlockPlan{Type: "pricing_packages", Purpose: cleanGeneratedText(block.Purpose, 280), Props: props}, true
	case "faq":
		props, keep := repairFAQProps(block.Props, pageTitle, allowPlaceholder)
		if !keep {
			return generationBlockPlan{}, false
		}
		return generationBlockPlan{Type: "faq", Purpose: cleanGeneratedText(block.Purpose, 280), Props: props}, true
	case "team_profile_cards":
		props, keep := repairTeamProfileCardsProps(block.Props, pageTitle, allowPlaceholder)
		if !keep {
			return generationBlockPlan{}, false
		}
		return generationBlockPlan{Type: "team_profile_cards", Purpose: cleanGeneratedText(block.Purpose, 280), Props: props}, true
	case "contact_form":
		props := repairContactFormProps(block.Props, pageTitle)
		return generationBlockPlan{Type: "contact_form", Purpose: cleanGeneratedText(block.Purpose, 280), Props: props}, true
	case "footer":
		props := repairFooterProps(block.Props, pageTitle)
		return generationBlockPlan{Type: "footer", Purpose: cleanGeneratedText(block.Purpose, 280), Props: props}, true
	case "cta_band":
		props := repairCTABandProps(block.Props, pageTitle, pageSlug)
		return generationBlockPlan{Type: "cta_band", Purpose: cleanGeneratedText(block.Purpose, 280), Props: props}, true
	default:
		return generationBlockPlan{}, false
	}
}

func repairHeroProps(props map[string]any, pageTitle string, pageSlug string) map[string]any {
	repaired := map[string]any{
		"variant":  readEnum(props, "variant", "standard", "standard", "full-page", "statement"),
		"headline": firstNonEmpty(readGeneratedText(props, "headline", 120), pageTitle),
		"layout":   readEnum(props, "layout", "split-left", "centered", "split-left", "split-right"),
	}
	if eyebrow := readGeneratedText(props, "eyebrow", 80); eyebrow != "" {
		repaired["eyebrow"] = eyebrow
	}
	headline, _ := repaired["headline"].(string)
	if subheadline := readGeneratedText(props, "subheadline", 280); subheadline != "" &&
		normalizeSupportingCopy(subheadline) != normalizeSupportingCopy(headline) {
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

func repairFeaturesGridProps(props map[string]any, pageTitle string, pageGoal string, allowPlaceholder bool) (map[string]any, bool) {
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
		if !allowPlaceholder {
			return nil, false
		}
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
	}, true
}

func repairGalleryProps(props map[string]any, pageTitle string, pageGoal string, allowPlaceholder bool) (map[string]any, bool) {
	images := make([]any, 0)
	if rawItems, ok := props["images"].([]any); ok {
		for _, raw := range rawItems {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			title := readGeneratedText(item, "title", 80)
			caption := readGeneratedText(item, "caption", 240)
			if title == "" {
				title = firstNonEmpty(caption, pageTitle)
			}
			if title == "" {
				continue
			}
			next := map[string]any{
				"title": title,
			}
			if caption != "" {
				next["caption"] = caption
			}
			if image := repairImage(item["image"]); image != nil {
				next["image"] = image
			}
			images = append(images, next)
			if len(images) == 12 {
				break
			}
		}
	}
	if len(images) == 0 {
		if !allowPlaceholder {
			return nil, false
		}
		images = append(images, map[string]any{
			"title":   firstNonEmpty(pageTitle, "Gallery highlight"),
			"caption": firstNonEmpty(pageGoal, "Add a short note describing the image or work shown here."),
		})
	}

	return map[string]any{
		"heading": firstNonEmpty(readGeneratedText(props, "heading", 120), pageTitle),
		"intro":   readGeneratedText(props, "intro", 500),
		"images":  images,
		"layout":  readEnum(props, "layout", "grid", "grid", "masonry", "spotlight"),
	}, true
}

func repairTestimonialsProps(props map[string]any, pageTitle string, allowPlaceholder bool) (map[string]any, bool) {
	items := make([]any, 0)
	if rawItems, ok := props["items"].([]any); ok {
		for _, raw := range rawItems {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			quote := readGeneratedText(item, "quote", 320)
			name := readGeneratedText(item, "name", 80)
			if quote == "" || name == "" {
				continue
			}
			next := map[string]any{
				"quote": quote,
				"name":  name,
			}
			if role := readGeneratedText(item, "role", 120); role != "" {
				next["role"] = role
			}
			if avatar := repairImage(item["avatar"]); avatar != nil {
				next["avatar"] = avatar
			}
			items = append(items, next)
			if len(items) == 6 {
				break
			}
		}
	}
	if len(items) == 0 {
		if !allowPlaceholder {
			return nil, false
		}
		items = append(items, map[string]any{
			"quote": "Add one concise testimonial that speaks to the actual experience of working together.",
			"name":  "Client name",
			"role":  "Client role",
		})
	}

	return map[string]any{
		"heading": firstNonEmpty(readGeneratedText(props, "heading", 120), pageTitle),
		"intro":   readGeneratedText(props, "intro", 500),
		"items":   items,
	}, true
}

func repairPricingPackagesProps(props map[string]any, pageTitle string, pageSlug string, allowPlaceholder bool) (map[string]any, bool) {
	plans := make([]any, 0)
	if rawPlans, ok := props["plans"].([]any); ok {
		for _, raw := range rawPlans {
			plan, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			name := readGeneratedText(plan, "name", 80)
			price := readGeneratedText(plan, "price", 40)
			description := readGeneratedText(plan, "description", 240)
			if name == "" || price == "" || description == "" {
				continue
			}
			next := map[string]any{
				"name":        name,
				"price":       price,
				"description": description,
			}
			features := repairFeatureList(plan["features"])
			if len(features) == 0 {
				features = []any{map[string]any{"text": "Add one focused feature"}}
			}
			next["features"] = features
			if cta := repairCTA(plan["cta"], "Get in touch", fallbackGeneratedCTAHref(pageSlug)); cta != nil {
				next["cta"] = cta
			}
			plans = append(plans, next)
			if len(plans) == 4 {
				break
			}
		}
	}
	if len(plans) == 0 {
		if !allowPlaceholder {
			return nil, false
		}
		plans = append(plans, map[string]any{
			"name":        firstNonEmpty(pageTitle, "Starter"),
			"price":       "From $350",
			"description": "Add a clear starting point so visitors can understand scope before they reach out.",
			"features":    []any{map[string]any{"text": "Focused scope"}},
			"cta":         map[string]any{"label": "Ask about fit", "href": fallbackGeneratedCTAHref(pageSlug)},
		})
	}

	return map[string]any{
		"heading": firstNonEmpty(readGeneratedText(props, "heading", 120), pageTitle),
		"intro":   readGeneratedText(props, "intro", 500),
		"plans":   plans,
	}, true
}

func repairFAQProps(props map[string]any, pageTitle string, allowPlaceholder bool) (map[string]any, bool) {
	items := make([]any, 0)
	if rawItems, ok := props["items"].([]any); ok {
		for _, raw := range rawItems {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			question := readGeneratedText(item, "question", 140)
			answer := readGeneratedText(item, "answer", 400)
			if question == "" || answer == "" {
				continue
			}
			items = append(items, map[string]any{
				"question": question,
				"answer":   answer,
			})
			if len(items) == 10 {
				break
			}
		}
	}
	if len(items) == 0 {
		if !allowPlaceholder {
			return nil, false
		}
		items = append(items, map[string]any{
			"question": "What should someone know before reaching out?",
			"answer":   "Add one practical answer that reduces hesitation and points toward the next step.",
		})
	}

	return map[string]any{
		"heading": firstNonEmpty(readGeneratedText(props, "heading", 120), pageTitle),
		"intro":   readGeneratedText(props, "intro", 500),
		"items":   items,
	}, true
}

func repairTeamProfileCardsProps(props map[string]any, pageTitle string, allowPlaceholder bool) (map[string]any, bool) {
	people := make([]any, 0)
	if rawPeople, ok := props["people"].([]any); ok {
		for _, raw := range rawPeople {
			person, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			name := readGeneratedText(person, "name", 80)
			role := readGeneratedText(person, "role", 80)
			bio := readGeneratedText(person, "bio", 400)
			if name == "" || role == "" || bio == "" {
				continue
			}
			next := map[string]any{
				"name": name,
				"role": role,
				"bio":  bio,
			}
			if photo := repairImage(person["photo"]); photo != nil {
				next["photo"] = photo
			}
			if links := repairLinkList(person["links"], 3); len(links) > 0 {
				next["links"] = links
			}
			people = append(people, next)
			if len(people) == 8 {
				break
			}
		}
	}
	if len(people) == 0 {
		if !allowPlaceholder {
			return nil, false
		}
		people = append(people, map[string]any{
			"name": "Founder",
			"role": "Lead contact",
			"bio":  "Add a short bio that explains who this person is and what they bring to the work.",
			"links": []any{
				map[string]any{
					"label": "Get in touch",
					"href":  "/contact",
				},
			},
		})
	}

	return map[string]any{
		"heading": firstNonEmpty(readGeneratedText(props, "heading", 120), pageTitle),
		"intro":   readGeneratedText(props, "intro", 500),
		"people":  people,
	}, true
}

func repairFooterProps(props map[string]any, pageTitle string) map[string]any {
	return map[string]any{
		"showBrand":    footerFlag(props, "showBrand"),
		"showMadeWith": footerFlag(props, "showMadeWith"),
		"tagline":      readGeneratedText(props, "tagline", 240),
		"contact":      repairFooterContact(props),
		"copyright":    firstNonEmpty(readGeneratedText(props, "copyright", 120), "Copyright 2026 "+pageTitle),
		"socialLinks":  repairLinkList(props["socialLinks"], 6),
	}
}

func repairFooterContact(props map[string]any) map[string]any {
	contact, _ := props["contact"].(map[string]any)
	result := map[string]any{}

	if address := repairFooterAddress(contact["address"]); len(address) > 0 {
		result["address"] = address
	}
	if phone := readGeneratedText(contact, "phone", 40); phone != "" {
		result["phone"] = phone
	}
	if email := readGeneratedText(contact, "email", 160); email != "" {
		result["email"] = email
	} else if contactLine := readGeneratedText(props, "contactLine", 180); contactLine != "" {
		result["email"] = contactLine
	}
	if hours := repairFooterHours(contact["hours"]); len(hours) > 0 {
		result["hours"] = hours
	}

	return result
}

// repairFooterAddress coerces the model's address into the structured
// { street, city, postalCode, region, country } shape (Spec 04). A legacy or
// stray single-line string is folded into `street` so no signal is lost.
func repairFooterAddress(value any) map[string]any {
	if text, ok := value.(string); ok {
		if street := cleanGeneratedText(text, 160); street != "" {
			return map[string]any{"street": street}
		}
		return nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	result := map[string]any{}
	for field, limit := range map[string]int{"street": 160, "city": 120, "postalCode": 40, "region": 120, "country": 120} {
		if text := readGeneratedText(object, field, limit); text != "" {
			result[field] = text
		}
	}
	return result
}

var footerWeekdaySet = map[string]bool{
	"monday": true, "tuesday": true, "wednesday": true, "thursday": true,
	"friday": true, "saturday": true, "sunday": true,
}

// repairFooterHours coerces the model's opening-hours array into the structured
// { day, opens, closes, closed } shape (Spec 04), keeping at most one entry per
// weekday and dropping anything without a recognizable day.
func repairFooterHours(value any) []any {
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	items := make([]any, 0, len(values))
	for _, raw := range values {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		day := strings.ToLower(strings.TrimSpace(readGeneratedText(entry, "day", 20)))
		if !footerWeekdaySet[day] || seen[day] {
			continue
		}
		seen[day] = true
		item := map[string]any{"day": day, "closed": readBool(entry, "closed")}
		if opens := repairClockTime(readGeneratedText(entry, "opens", 5)); opens != "" {
			item["opens"] = opens
		}
		if closes := repairClockTime(readGeneratedText(entry, "closes", 5)); closes != "" {
			item["closes"] = closes
		}
		items = append(items, item)
		if len(items) == 7 {
			break
		}
	}
	return items
}

// repairClockTime keeps a value only when it is a valid HH:MM clock time, so the
// structured hours never carry free-form text the validator would reject.
func repairClockTime(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) != 5 || trimmed[2] != ':' {
		return ""
	}
	for i, r := range trimmed {
		if i == 2 {
			continue
		}
		if r < '0' || r > '9' {
			return ""
		}
	}
	hours := int(trimmed[0]-'0')*10 + int(trimmed[1]-'0')
	minutes := int(trimmed[3]-'0')*10 + int(trimmed[4]-'0')
	if hours > 23 || minutes > 59 {
		return ""
	}
	return trimmed
}

func footerFlag(props map[string]any, key string) bool {
	if _, ok := props[key]; !ok {
		return true
	}
	return readBool(props, key)
}

func repairContactFormProps(props map[string]any, pageTitle string) map[string]any {
	fields := repairContactFormFields(props["fields"])
	if len(fields) == 0 {
		fields = []any{
			map[string]any{"name": "name", "label": "Name", "type": "name", "required": true},
			map[string]any{"name": "email", "label": "Email", "type": "email", "required": true},
			map[string]any{"name": "message", "label": "Message", "type": "message", "required": true},
		}
	}

	return map[string]any{
		"heading":     firstNonEmpty(readGeneratedText(props, "heading", 120), pageTitle),
		"intro":       readGeneratedText(props, "intro", 500),
		"submitLabel": firstNonEmpty(readGeneratedText(props, "submitLabel", 40), "Send message"),
		"fields":      fields,
	}
}

func repairContactFormFields(value any) []any {
	values, ok := value.([]any)
	if !ok {
		return nil
	}

	fields := make([]any, 0, len(values))
	seenNames := map[string]bool{}
	for _, raw := range values {
		field, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		name := readGeneratedText(field, "name", 40)
		label := readGeneratedText(field, "label", 80)
		fieldType := readEnum(field, "type", "", "name", "email", "phone", "message", "select")
		if name == "" || label == "" || fieldType == "" || seenNames[name] {
			continue
		}

		next := map[string]any{
			"name":     name,
			"label":    label,
			"type":     fieldType,
			"required": readBool(field, "required"),
		}
		if fieldType == "select" {
			options := repairStringList(field["options"], 8)
			if len(options) == 0 {
				continue
			}
			next["options"] = options
		}
		fields = append(fields, next)
		seenNames[name] = true
		if len(fields) == 6 {
			break
		}
	}

	return fields
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
	// The model is forced by the strict schema to emit an assetId string and
	// sometimes invents one ("hero-image"). Real asset ids are UUIDs; anything
	// else must be dropped so the imagery pass fills the slot with a real
	// asset instead of the writer rejecting the whole draft.
	if !ids.IsValid(assetID) {
		return nil
	}
	repaired := map[string]any{"assetId": assetID}
	if alt := readGeneratedText(object, "alt", 180); alt != "" {
		repaired["alt"] = alt
		return repaired
	}
	repaired["alt"] = "Descriptive image"
	return repaired
}

func repairFeatureList(value any) []any {
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	features := make([]any, 0, len(values))
	for _, raw := range values {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		text := readGeneratedText(item, "text", 120)
		if text == "" {
			continue
		}
		features = append(features, map[string]any{"text": text})
		if len(features) == 6 {
			break
		}
	}
	return features
}

func repairLinkList(value any, limit int) []any {
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	links := make([]any, 0, len(values))
	for _, raw := range values {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		label := readGeneratedText(item, "label", 40)
		href := readSafeURL(item, "href", "")
		if label == "" || href == "" {
			continue
		}
		links = append(links, map[string]any{
			"label": label,
			"href":  href,
		})
		if len(links) == limit {
			break
		}
	}
	return links
}

func repairStringList(value any, limit int) []any {
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	items := make([]any, 0, len(values))
	for _, raw := range values {
		text, ok := raw.(string)
		if !ok {
			continue
		}
		clean := cleanGeneratedText(text, 80)
		if clean == "" {
			continue
		}
		items = append(items, clean)
		if len(items) == limit {
			break
		}
	}
	return items
}

func readBool(props map[string]any, key string) bool {
	value, ok := props[key]
	if !ok {
		return false
	}
	parsed, ok := value.(bool)
	return ok && parsed
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
		case "gallery", "testimonials", "pricing_packages", "faq", "team_profile_cards", "contact_form", "footer":
			if text := readGeneratedText(block.Props, "intro", 180); text != "" {
				return text
			}
			if text := readGeneratedText(block.Props, "tagline", 180); text != "" {
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
