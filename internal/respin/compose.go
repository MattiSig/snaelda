package respin

import (
	"fmt"
	"strings"

	"github.com/MattiSig/snaelda/internal/generation"
)

// ComposeContext carries the non-analysis inputs the composer needs: the source
// URL (recorded in the brief for provenance and to anchor the re-spin framing)
// and the pre-allocated site id that brand/hero assets were ingested against, so
// generation builds the draft under the same id those assets are scoped to.
type ComposeContext struct {
	SourceURL string
	SiteID    string
	// SourceHero is the source site's hero, read deterministically from the home
	// page's DOM (Spec 21 step 7). The composer resolves its background image to a
	// pulled asset id and carries it into optionalHints.sourceHero.
	SourceHero SourceHero
}

// Composition is the composer's output. Its Input is exactly Spec 07's Minimum
// Input shape (Spec 21 "Hard Contract") — re-spin never introduces a second
// draft format, so everything downstream of composition is standard generation.
//
// Degraded/DegradationReason mirror onto the respin_imports record. A degraded
// composition is not a dead end: it is a thinner instance of the same contract,
// and PromptPrefill carries the salvaged brief the demo UI drops into the
// ordinary homepage prompt experience when the pipeline chooses to hand off to
// the prompt flow rather than generate unattended.
type Composition struct {
	Input             generation.GenerateInput
	Degraded          bool
	DegradationReason string
	PromptPrefill     string
}

// Salvage is the partial information available when the LLM analysis stages
// could not run at all (thin content, fetch failure, budget exhaustion).
// ComposeDegraded turns whatever was salvaged into a thin canonical input that
// pre-fills the prompt flow, so a broken read still lands the visitor somewhere
// useful (Spec 21 graceful degradation).
type Salvage struct {
	BusinessName string
	Vertical     string
	Locale       string
	// Snippet is any readable copy fragment recovered before the pipeline gave
	// up (e.g. the page title or meta description), folded into the prefill so
	// the visitor sees a head start rather than a blank prompt.
	Snippet string
}

// Compose assembles the canonical generation input from the LLM analysis and the
// pulled brand assets (Spec 21 pipeline step 10). It never fails and never
// dead-ends: with empty analysis it still yields a usable, thin input. When the
// analysis flagged a soft degradation (low classification confidence), the
// composition carries that flag through to the import record while still
// producing a full input the caller can generate from with the generic block
// set.
func Compose(analysis AnalysisResult, brand BrandResult, cctx ComposeContext) Composition {
	fields := analysis.Fields

	locale := normalizeStageLocale(analysis.TargetLocale)
	if locale == "" {
		locale = normalizeStageLocale(analysis.Classification.Locale)
	}
	if locale == "" {
		locale = "is"
	}

	name := firstNonEmptyString(fields.BusinessName, brand.Brand.BusinessName)

	// Brand identity is used verbatim (Spec 21): the composer only backfills the
	// business name onto the brand config when the extraction found one and the
	// pull did not, so downstream brand-aware blocks have a name to render.
	brandConfig := brand.Brand
	if strings.TrimSpace(brandConfig.BusinessName) == "" {
		brandConfig.BusinessName = name
	}

	input := generation.GenerateInput{
		Name:              name,
		Prompt:            composeBrief(analysis, cctx),
		PreferredLanguage: locale,
		OptionalHints:     composeHints(analysis),
		Brand:             brandConfig,
		SourceHero:        composeSourceHero(cctx.SourceHero, brand),
		// The site was reserved before the brand pull so its logo/hero assets are
		// already scoped to this id; generation reuses it verbatim. The pulled hero
		// photos seed the draft's image slots ahead of any stock imagery.
		SiteID:       strings.TrimSpace(cctx.SiteID),
		SeedAssetIDs: brand.HeroAssetIDs,
	}

	comp := Composition{
		Input:         input,
		PromptPrefill: composePrefill(name, analysis.Classification.Vertical, fields, ""),
	}
	if analysis.Degraded {
		comp.Degraded = true
		comp.DegradationReason = analysis.DegradationReason
	}
	return comp
}

// ComposeDegraded builds a thin canonical input from partial salvage when the
// analysis stages could not run. The result is always degraded and always
// carries a PromptPrefill so the demo UI can hand off to the prompt flow with a
// head start instead of a blank slate (Spec 21).
func ComposeDegraded(reason string, salvage Salvage, brand BrandResult, cctx ComposeContext) Composition {
	locale := normalizeStageLocale(salvage.Locale)
	if locale == "" {
		locale = "is"
	}
	name := firstNonEmptyString(salvage.BusinessName, brand.Brand.BusinessName)

	brandConfig := brand.Brand
	if strings.TrimSpace(brandConfig.BusinessName) == "" {
		brandConfig.BusinessName = name
	}

	prefill := composePrefill(name, salvage.Vertical, ExtractedFields{}, salvage.Snippet)

	hints := map[string]string{}
	if v := strings.TrimSpace(salvage.Vertical); v != "" {
		hints["industry"] = strings.ToLower(v)
	}
	if len(hints) == 0 {
		hints = nil
	}

	return Composition{
		Input: generation.GenerateInput{
			Name:              name,
			Prompt:            prefill,
			PreferredLanguage: locale,
			OptionalHints:     hints,
			Brand:             brandConfig,
		},
		Degraded:          true,
		DegradationReason: strings.TrimSpace(reason),
		PromptPrefill:     prefill,
	}
}

// composeSourceHero maps the deterministically-extracted source hero into the
// Spec 07 generation contract, resolving the background image URL to the asset id
// it was ingested as (when the brand pull pulled it). It returns nil when the
// extraction found no usable hero, so the hint is simply omitted.
func composeSourceHero(hero SourceHero, brand BrandResult) *generation.SourceHero {
	if hero.IsEmpty() {
		return nil
	}
	assetID := strings.TrimSpace(hero.ImageAssetID)
	if assetID == "" && hero.ImageURL != "" {
		assetID = brand.AssetIDByURL[hero.ImageURL]
	}
	return &generation.SourceHero{
		Headline:     strings.TrimSpace(hero.Headline),
		Subheadline:  strings.TrimSpace(hero.Subheadline),
		CTALabel:     strings.TrimSpace(hero.CTALabel),
		ImageAssetID: assetID,
		// A hero is text-only when the source carried no background image at all.
		TextOnly: strings.TrimSpace(hero.ImageURL) == "" && assetID == "",
	}
}

// composeHints builds the OptionalHints map: the classified industry and the
// page set the extracted content supports. Style is intentionally omitted — the
// theme is derived by generation, and re-spin never clones the source design
// (Spec 21 scope boundary).
func composeHints(analysis AnalysisResult) map[string]string {
	hints := map[string]string{}
	if v := strings.TrimSpace(analysis.Classification.Vertical); v != "" {
		hints["industry"] = strings.ToLower(v)
	}
	if pages := resolvePages(analysis.Fields); len(pages) > 0 {
		hints["pages"] = strings.Join(pages, ", ")
	}
	if len(hints) == 0 {
		return nil
	}
	return hints
}

// resolvePages derives the page set from what the extraction actually found, so
// generation only plans pages there is content for. Home is always present.
func resolvePages(fields ExtractedFields) []string {
	pages := []string{"home"}
	if len(fields.Services) > 0 {
		pages = append(pages, "services")
	}
	if strings.TrimSpace(fields.About) != "" {
		pages = append(pages, "about")
	}
	if len(fields.FAQs) > 0 {
		pages = append(pages, "faq")
	}
	if !fields.Contact.IsEmpty() {
		pages = append(pages, "contact")
	}
	return pages
}

// composePrefill builds the short, human-readable prompt the degrade-to-prompt
// UI pre-fills. It reads as something the visitor could have typed themselves,
// seeded from whatever was salvaged (business name, vertical, a copy fragment).
func composePrefill(name, vertical string, fields ExtractedFields, snippet string) string {
	name = strings.TrimSpace(name)
	vertical = strings.TrimSpace(vertical)

	var b strings.Builder
	if name != "" && vertical != "" {
		fmt.Fprintf(&b, "A website for %s, a %s business.", name, vertical)
	} else if name != "" {
		fmt.Fprintf(&b, "A website for %s.", name)
	} else if vertical != "" {
		fmt.Fprintf(&b, "A website for a %s business.", vertical)
	} else {
		b.WriteString("A website for my business.")
	}

	if tagline := strings.TrimSpace(fields.Tagline); tagline != "" {
		fmt.Fprintf(&b, " %s", ensureSentence(tagline))
	} else if about := firstSentence(fields.About); about != "" {
		fmt.Fprintf(&b, " %s", ensureSentence(about))
	} else if snippet := strings.TrimSpace(snippet); snippet != "" {
		fmt.Fprintf(&b, " %s", ensureSentence(firstSentence(snippet)))
	}

	return strings.TrimSpace(b.String())
}

// weekdayOrder and weekdayTitles render canonical weekday keys in calendar order
// for the brief. Labels are English because the brief is a model-facing
// instruction, not user copy — the generated output language is governed by
// PreferredLanguage, and the verbatim content it carries is already in the
// target language.
var weekdayOrder = []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}

var weekdayTitles = map[string]string{
	"monday":    "Monday",
	"tuesday":   "Tuesday",
	"wednesday": "Wednesday",
	"thursday":  "Thursday",
	"friday":    "Friday",
	"saturday":  "Saturday",
	"sunday":    "Sunday",
}

// composeBrief synthesizes the extracted content into the business brief that
// satisfies the generation job's prompt column and feeds the engine's normal
// intent-extraction and content stages (Spec 21). It renders only the sections
// the extraction populated and instructs the model to treat the facts as
// verbatim, so generation reproduces real services/prices/hours rather than
// inventing plausible ones.
func composeBrief(analysis AnalysisResult, cctx ComposeContext) string {
	fields := analysis.Fields
	name := firstNonEmptyString(fields.BusinessName, analysis.Classification.Vertical)

	var b strings.Builder
	b.WriteString("Re-spin an existing small business into a new website. ")
	b.WriteString("Use the business's own facts below verbatim — do not invent or embellish services, prices, opening hours, contact details, or testimonials.\n")

	if name != "" {
		fmt.Fprintf(&b, "\nBusiness name: %s", strings.TrimSpace(fields.BusinessName))
	}
	if v := strings.TrimSpace(analysis.Classification.Vertical); v != "" {
		fmt.Fprintf(&b, "\nIndustry: %s", v)
	}
	if tone := strings.TrimSpace(analysis.Classification.Tone); tone != "" {
		fmt.Fprintf(&b, "\nBrand tone: %s", tone)
	}
	if tagline := strings.TrimSpace(fields.Tagline); tagline != "" {
		fmt.Fprintf(&b, "\nTagline: %s", tagline)
	}

	if about := strings.TrimSpace(fields.About); about != "" {
		fmt.Fprintf(&b, "\n\nAbout:\n%s", about)
	}

	if len(fields.Services) > 0 {
		b.WriteString("\n\nServices:")
		for _, svc := range fields.Services {
			line := "\n- " + strings.TrimSpace(svc.Name)
			if desc := strings.TrimSpace(svc.Description); desc != "" {
				line += " — " + desc
			}
			if price := strings.TrimSpace(svc.Price); price != "" {
				line += " (" + price + ")"
			}
			b.WriteString(line)
		}
	}

	if hours := renderHours(fields.Hours); hours != "" {
		b.WriteString("\n\nOpening hours:")
		b.WriteString(hours)
	}

	if contact := renderContact(fields.Contact); contact != "" {
		b.WriteString("\n\nContact:")
		b.WriteString(contact)
	}

	if len(fields.Testimonials) > 0 {
		b.WriteString("\n\nTestimonials (quote verbatim, never fabricate):")
		for _, t := range fields.Testimonials {
			line := "\n- \"" + strings.TrimSpace(t.Quote) + "\""
			if author := strings.TrimSpace(t.Author); author != "" {
				line += " — " + author
			}
			b.WriteString(line)
		}
	}

	if len(fields.Offers) > 0 {
		b.WriteString("\n\nCurrent offers/announcements (keep the offer, do not invent new ones):")
		for _, o := range fields.Offers {
			line := "\n- " + strings.TrimSpace(o.Title)
			if desc := strings.TrimSpace(o.Description); desc != "" {
				line += " — " + desc
			}
			b.WriteString(line)
		}
	}

	if len(fields.ServiceAreas) > 0 {
		fmt.Fprintf(&b, "\n\nService areas (keep place names verbatim): %s", strings.Join(fields.ServiceAreas, ", "))
	}

	if len(fields.ClientTypes) > 0 {
		fmt.Fprintf(&b, "\n\nWho they serve: %s", strings.Join(fields.ClientTypes, ", "))
	}

	if len(fields.FAQs) > 0 {
		b.WriteString("\n\nFAQ (use these real questions and answers; never invent a Q&A):")
		for _, f := range fields.FAQs {
			fmt.Fprintf(&b, "\n- Q: %s", strings.TrimSpace(f.Question))
			if ans := strings.TrimSpace(f.Answer); ans != "" {
				fmt.Fprintf(&b, "\n  A: %s", ans)
			}
		}
	}

	if src := strings.TrimSpace(cctx.SourceURL); src != "" {
		fmt.Fprintf(&b, "\n\nSource site: %s", src)
	}

	return strings.TrimSpace(b.String())
}

// renderHours renders the opening-hours rows in calendar order, one line each.
func renderHours(hours []ExtractHours) string {
	if len(hours) == 0 {
		return ""
	}
	byDay := make(map[string]ExtractHours, len(hours))
	for _, h := range hours {
		byDay[h.Day] = h
	}
	var b strings.Builder
	for _, day := range weekdayOrder {
		h, ok := byDay[day]
		if !ok {
			continue
		}
		title := weekdayTitles[day]
		if h.Closed || (h.Opens == "" && h.Closes == "") {
			fmt.Fprintf(&b, "\n- %s: Closed", title)
			continue
		}
		fmt.Fprintf(&b, "\n- %s: %s–%s", title, h.Opens, h.Closes)
	}
	return b.String()
}

// renderContact renders the populated contact fields, one line each.
func renderContact(contact ContactDetails) string {
	var b strings.Builder
	if phone := strings.TrimSpace(contact.Phone); phone != "" {
		fmt.Fprintf(&b, "\n- Phone: %s", phone)
	}
	if email := strings.TrimSpace(contact.Email); email != "" {
		fmt.Fprintf(&b, "\n- Email: %s", email)
	}
	if address := strings.TrimSpace(contact.Address); address != "" {
		fmt.Fprintf(&b, "\n- Address: %s", address)
	}
	return b.String()
}

// firstSentence returns the first sentence of s (up to the first ., !, or ?),
// trimmed. It is used to keep the pre-filled prompt short.
func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if idx := strings.IndexAny(s, ".!?"); idx >= 0 {
		return strings.TrimSpace(s[:idx+1])
	}
	return s
}

// ensureSentence appends a period when s does not already end in sentence
// punctuation, so composed prefills read as complete sentences.
func ensureSentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	switch s[len(s)-1] {
	case '.', '!', '?':
		return s
	default:
		return s + "."
	}
}
