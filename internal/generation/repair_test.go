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

	repaired := repairGenerationPlan(plan, "")

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

	repaired := repairGenerationPlan(plan, "")

	if blockIndex(repaired.Pages[1].Blocks, "contact_form") != -1 {
		t.Fatalf("did not expect contact page to gain mandatory contact_form, got %#v", repaired.Pages[1].Blocks)
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
