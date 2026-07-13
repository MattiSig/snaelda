package generation

import (
	"encoding/json"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

type layoutBlockCatalogEntry struct {
	Type         string   `json:"type"`
	DisplayName  string   `json:"displayName"`
	Category     string   `json:"category"`
	Tags         []string `json:"tags"`
	Use          string   `json:"use"`
	Avoid        string   `json:"avoid"`
	VariantHints []string `json:"variantHints"`
}

func layoutBlockCatalog() []layoutBlockCatalogEntry {
	definitions := siteconfig.DefaultBlockRegistry().Definitions()
	catalog := make([]layoutBlockCatalogEntry, 0, len(definitions))
	for _, definition := range definitions {
		entry := layoutBlockCatalogEntry{
			Type:        definition.Type,
			DisplayName: definition.DisplayName,
			Category:    string(definition.Category),
			Tags:        []string{string(definition.Category)},
			Use:         definition.Tagline,
			Avoid:       "",
		}

		switch definition.Type {
		case "hero":
			entry.Tags = []string{"opener", "positioning", "first_impression", "cta"}
			entry.Use = "Open a page with the main promise, context, and primary action."
			entry.Avoid = "Deep detail, long prose, or repeated mid-page sections."
			entry.VariantHints = []string{"standard", "full-page", "statement", "centered", "split-left", "split-right"}
		case "text_section":
			entry.Tags = []string{"content", "narrative", "explanation", "about"}
			entry.Use = "Explain a focused idea in prose."
			entry.Avoid = "Lists, input forms, pricing, social proof, or image-led showcases."
			entry.VariantHints = []string{"narrow", "default", "wide", "left", "center"}
		case "image_text":
			entry.Tags = []string{"media", "story", "process", "feature"}
			entry.Use = "Pair one strong image with explanatory copy or a focused CTA."
			entry.Avoid = "Large galleries or purely text-only detail."
			entry.VariantHints = []string{"image-left", "image-right"}
		case "features_grid":
			entry.Tags = []string{"benefits", "services", "features", "overview"}
			entry.Use = "Summarize multiple benefits, services, or differentiators."
			entry.Avoid = "Long narrative, pricing tiers, or customer quotes."
			entry.VariantHints = []string{"2-columns", "3-columns", "4-columns"}
		case "cta_band":
			entry.Tags = []string{"conversion", "cta", "next_step"}
			entry.Use = "Invite the visitor into one clear next action."
			entry.Avoid = "Collecting visitor input; use contact_form for forms."
			entry.VariantHints = []string{"primary", "secondary", "accent"}
		case "contact_form":
			entry.Tags = []string{"contact", "lead_capture", "inquiry_form", "message_form", "conversion"}
			entry.Use = "Let visitors submit a message, request, booking inquiry, or lead form."
			entry.Avoid = "Only showing email, phone, address, hours, or footer details."
			entry.VariantHints = []string{"simple-inquiry", "booking-request", "wholesale-request"}
		case "gallery":
			entry.Tags = []string{"gallery", "portfolio", "visual_showcase", "media"}
			entry.Use = "Show multiple images, portfolio items, products, spaces, or visual proof."
			entry.Avoid = "Single supporting image or text-only content."
			entry.VariantHints = []string{"grid", "masonry", "spotlight"}
		case "testimonials":
			entry.Tags = []string{"social_proof", "quotes", "trust", "reviews"}
			entry.Use = "Show customer/client quotes and attribution."
			entry.Avoid = "Input forms, contact capture, or generic section headings without quotes."
		case "pricing_packages":
			entry.Tags = []string{"pricing", "packages", "offers", "conversion"}
			entry.Use = "Present purchasable packages, subscriptions, tiers, or menu-like offer groups."
			entry.Avoid = "Generic feature lists without prices or package distinction."
		case "faq":
			entry.Tags = []string{"faq", "questions", "objections", "expectations"}
			entry.Use = "Answer common visitor questions and reduce uncertainty."
			entry.Avoid = "Primary page opener or broad service overview."
		case "team_profile_cards":
			entry.Tags = []string{"team", "people", "founder", "trust"}
			entry.Use = "Introduce founders, staff, makers, or named practitioners."
			entry.Avoid = "Anonymous brands with no people to introduce."
		case "stats":
			entry.Tags = []string{"metrics", "proof", "numbers", "credibility"}
			entry.Use = "Show quantified proof, milestones, capacity, or outcomes."
			entry.Avoid = "Inventing numbers when the prompt gives no basis."
		case "footer":
			entry.Tags = []string{"footer", "navigation", "contact_details", "legal"}
			entry.Use = "Close the page/site with navigation, contact details, socials, and copyright."
			entry.Avoid = "Main content or visitor input forms."
		case "collection_list":
			entry.Tags = []string{"collections", "entries", "featured_list", "dynamic_content"}
			entry.Use = "Show selected entries from a collection."
			entry.Avoid = "When no collection exists or static content is enough."
		case "collection_index":
			entry.Tags = []string{"collections", "index", "listing", "dynamic_content"}
			entry.Use = "List all entries for a collection index page."
			entry.Avoid = "Static pages not backed by a collection."
		case "collection_detail":
			entry.Tags = []string{"collections", "detail", "template", "dynamic_content"}
			entry.Use = "Render one active collection entry on a detail template."
			entry.Avoid = "Static pages or manually authored one-off content."
		}
		catalog = append(catalog, entry)
	}
	return catalog
}

func layoutBlockCatalogContext() string {
	bytes, err := json.Marshal(map[string]any{
		"layoutBlockCatalog": layoutBlockCatalog(),
	})
	if err != nil {
		return ""
	}
	return "Snaelda page layout catalog:\n" + string(bytes)
}
