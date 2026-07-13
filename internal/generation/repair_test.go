package generation

import (
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

func TestRepairGenerationPlanConvertsWrongBlockWhenModelIntendedContactForm(t *testing.T) {
	plan := generationPlan{
		SiteName: "North Light Studio",
		SiteGoal: "Help visitors understand the studio and send inquiries.",
		Theme:    siteconfig.ThemePreset(siteconfig.ThemePaletteCleanLocal),
		Pages: []generationPagePlan{
			{
				Title: "Home",
				Slug:  "/",
				Blocks: []generationBlockPlan{{
					Type: "hero",
					Props: map[string]any{
						"headline": "Quiet photography for real homes",
					},
				}},
			},
			{
				Title: "Contact",
				Slug:  "/contact",
				Goal:  "Convert a ready visitor into an inquiry.",
				Blocks: []generationBlockPlan{
					{
						Type: "hero",
						Props: map[string]any{
							"headline": "Start a conversation",
						},
					},
					{
						Type: "testimonials",
						Props: map[string]any{
							"heading": "Contact form",
							"intro":   "Tell us about your request and how we can help.",
							"items": []any{
								map[string]any{
									"quote": "Add one concise testimonial that speaks to the actual experience.",
									"name":  "Client name",
								},
							},
						},
					},
					{
						Type: "footer",
						Props: map[string]any{
							"copyright": "North Light Studio",
						},
					},
				},
			},
		},
	}

	repaired := repairGenerationPlan(plan, "", true)

	contactPage := repaired.Pages[1]
	formIndex := blockIndex(contactPage.Blocks, "contact_form")
	if formIndex == -1 {
		t.Fatalf("expected contact page to gain contact_form, got %#v", contactPage.Blocks)
	}
	if blockIndex(contactPage.Blocks, "testimonials") != -1 {
		t.Fatalf("expected intended form block to replace testimonials, got %#v", contactPage.Blocks)
	}
	footerIndex := blockIndex(contactPage.Blocks, "footer")
	if footerIndex != -1 && formIndex > footerIndex {
		t.Fatalf("expected contact_form before footer, got form=%d footer=%d blocks=%#v", formIndex, footerIndex, contactPage.Blocks)
	}
	if err := siteconfig.DefaultBlockRegistry().ValidateProps("contact_form", siteconfig.BlockVersionV1, "props", contactPage.Blocks[formIndex].Props); err != nil {
		t.Fatalf("expected generated contact_form props to be valid, got %v", err)
	}
}

func TestRepairGenerationPlanDoesNotMakeContactPageFormMandatory(t *testing.T) {
	plan := generationPlan{
		SiteName: "North Light Studio",
		SiteGoal: "Help visitors understand the studio and send inquiries.",
		Theme:    siteconfig.ThemePreset(siteconfig.ThemePaletteCleanLocal),
		Pages: []generationPagePlan{
			{
				Title: "Home",
				Slug:  "/",
				Blocks: []generationBlockPlan{{
					Type: "hero",
					Props: map[string]any{
						"headline": "Quiet photography for real homes",
					},
				}},
			},
			{
				Title: "Contact",
				Slug:  "/contact",
				Goal:  "Explain how to reach the studio.",
				Blocks: []generationBlockPlan{
					{
						Type: "text_section",
						Props: map[string]any{
							"heading": "Visit the studio",
							"body":    "Email or call us to arrange a time that works.",
						},
					},
					{
						Type: "cta_band",
						Props: map[string]any{
							"heading": "Prefer email?",
							"body":    "Send a short note when you know the rough timing.",
						},
					},
				},
			},
		},
	}

	repaired := repairGenerationPlan(plan, "", true)

	if blockIndex(repaired.Pages[1].Blocks, "contact_form") != -1 {
		t.Fatalf("did not expect contact page to gain mandatory contact_form, got %#v", repaired.Pages[1].Blocks)
	}
}

func TestRepairFooterPropsProducesStructuredContact(t *testing.T) {
	props := repairFooterProps(map[string]any{
		"showMadeWith": false,
		"tagline":      "Handmade in Reykjavík.",
		"copyright":    "",
		"contact": map[string]any{
			"address": map[string]any{
				"street":     "Laugavegur 12",
				"city":       "Reykjavík",
				"postalCode": "101",
			},
			"phone": "+354 555 1234",
			"email": "hallo@example.is",
			"hours": []any{
				map[string]any{"day": "Monday", "opens": "09:00", "closes": "17:00"},
				map[string]any{"day": "sunday", "closed": true, "opens": "bad"},
				map[string]any{"day": "monday", "opens": "10:00"}, // duplicate weekday dropped
				map[string]any{"day": "someday"},                  // unknown weekday dropped
			},
		},
	}, "Fléttan")

	if props["showMadeWith"] != false {
		t.Fatalf("expected showMadeWith preserved as false, got %v", props["showMadeWith"])
	}
	if props["showBrand"] != true {
		t.Fatalf("expected showBrand to default true, got %v", props["showBrand"])
	}

	contact := props["contact"].(map[string]any)
	address := contact["address"].(map[string]any)
	if address["street"] != "Laugavegur 12" || address["city"] != "Reykjavík" {
		t.Fatalf("unexpected structured address: %#v", address)
	}
	hours := contact["hours"].([]any)
	if len(hours) != 2 {
		t.Fatalf("expected 2 valid hours entries (dedup + drop unknown), got %#v", hours)
	}
	sunday := hours[1].(map[string]any)
	if sunday["closed"] != true {
		t.Fatalf("expected sunday closed, got %#v", sunday)
	}
	if _, ok := sunday["opens"]; ok {
		t.Fatalf("expected invalid opens time dropped, got %#v", sunday)
	}

	if err := siteconfig.DefaultBlockRegistry().ValidateProps("footer", siteconfig.BlockVersionV1, "props", props); err != nil {
		t.Fatalf("repaired footer props failed validation: %v", err)
	}
}

func blockIndex(blocks []generationBlockPlan, blockType string) int {
	for index, block := range blocks {
		if block.Type == blockType {
			return index
		}
	}
	return -1
}

// planWithEmptyRepeaters returns a home page with a hero followed by repeater
// blocks that carry no real items — the shape a model produces when it plans a
// section it has no facts to fill.
func planWithEmptyRepeaters() generationPlan {
	return generationPlan{
		SiteName: "North Light Studio",
		SiteGoal: "Help visitors understand the studio and get in touch.",
		Theme:    siteconfig.ThemePreset(siteconfig.ThemePaletteCleanLocal),
		Pages: []generationPagePlan{
			{
				Title: "Home",
				Slug:  "/",
				Blocks: []generationBlockPlan{
					{
						Type:  "hero",
						Props: map[string]any{"headline": "Quiet photography for real homes"},
					},
					{
						Type:  "testimonials",
						Props: map[string]any{"heading": "Kind words", "items": []any{}},
					},
					{
						Type:  "faq",
						Props: map[string]any{"heading": "Questions", "items": []any{}},
					},
				},
			},
		},
	}
}

func TestRepairGenerationPlanDropsEmptyRepeatersOnFreshGeneration(t *testing.T) {
	repaired := repairGenerationPlan(planWithEmptyRepeaters(), "", true)

	home := repaired.Pages[0]
	if blockIndex(home.Blocks, "testimonials") != -1 {
		t.Fatalf("expected empty testimonials block dropped on fresh generation, got %#v", home.Blocks)
	}
	if blockIndex(home.Blocks, "faq") != -1 {
		t.Fatalf("expected empty faq block dropped on fresh generation, got %#v", home.Blocks)
	}
	if blockIndex(home.Blocks, "hero") == -1 {
		t.Fatalf("expected hero to survive, got %#v", home.Blocks)
	}
}

func TestRepairGenerationPlanKeepsPlaceholderRepeatersOnReprompt(t *testing.T) {
	repaired := repairGenerationPlan(planWithEmptyRepeaters(), "", false)

	home := repaired.Pages[0]
	testimonialsIndex := blockIndex(home.Blocks, "testimonials")
	if testimonialsIndex == -1 {
		t.Fatalf("expected testimonials placeholder kept on reprompt, got %#v", home.Blocks)
	}
	items, ok := home.Blocks[testimonialsIndex].Props["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one placeholder testimonial item on reprompt, got %#v", home.Blocks[testimonialsIndex].Props["items"])
	}
	if blockIndex(home.Blocks, "faq") == -1 {
		t.Fatalf("expected faq placeholder kept on reprompt, got %#v", home.Blocks)
	}
}

func TestGeneratedTextMentionsContactForm(t *testing.T) {
	forms := []string{
		"Contact form",
		"Use this form to reach us",
		"Fill out the form below",
		"Request service now",
		"Send us a message and we'll reply",
		"Request a quote",
		"Fylltu út eyðublaðið",
	}
	for _, value := range forms {
		if !generatedTextMentionsContactForm(value) {
			t.Errorf("expected %q to read as contact-form intent", value)
		}
	}

	// Generic navigation CTAs must NOT be rewritten into an inline form.
	notForms := []string{
		"Get in touch",
		"Contact us",
		"Learn more about the studio",
		"",
	}
	for _, value := range notForms {
		if generatedTextMentionsContactForm(value) {
			t.Errorf("did not expect %q to read as contact-form intent", value)
		}
	}
}
