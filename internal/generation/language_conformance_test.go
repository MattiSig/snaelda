package generation

import (
	"context"
	"strings"
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// icelandicPlan is a small, fully-Icelandic plan used as the clean baseline the
// detector must pass without complaint.
func icelandicPlan() generationPlan {
	return generationPlan{
		SiteName: "Norðurljós Stúdíó",
		SiteGoal: "Hjálpa gestum að skilja stúdíóið og senda fyrirspurn.",
		Pages: []generationPagePlan{
			{
				Title: "Heim",
				Slug:  "/",
				SEO: siteconfig.SEOConfig{
					Title:       "Norðurljós Stúdíó",
					Description: "Róleg ljósmyndastofa í hjarta Reykjavíkur.",
				},
				Blocks: []generationBlockPlan{
					{
						Type: "hero",
						Props: map[string]any{
							"headline":    "Kyrrlát ljósmyndun fyrir alvöru heimili",
							"subheadline": "Við föngum augnablikin sem skipta máli.",
							"primaryCta":  map[string]any{"label": "Hafðu samband", "href": "/hafa-samband"},
						},
					},
					{
						Type: "features_grid",
						Props: map[string]any{
							"heading": "Þjónusta okkar",
							"items": []any{
								map[string]any{"title": "Brúðkaup", "body": "Heilsdags myndataka með rólegri nálgun."},
								map[string]any{"title": "Fjölskyldur", "body": "Hlýlegar myndir heima eða úti í náttúrunni."},
							},
						},
					},
				},
			},
		},
	}
}

func TestDetectLanguageConformancePassesCleanIcelandicPlan(t *testing.T) {
	issues := detectLanguageConformanceIssues(icelandicPlan(), "is", verbatimExemption{})
	if len(issues) != 0 {
		t.Fatalf("expected no issues for a native Icelandic plan, got %#v", issues)
	}
}

func TestDetectLanguageConformanceFlagsEnglishLeaks(t *testing.T) {
	plan := icelandicPlan()
	plan.SiteGoal = "Help visitors understand the studio and send an inquiry."
	plan.Pages[0].Blocks[0].Props["subheadline"] = "We capture the moments that matter."
	plan.Pages[0].SEO.Description = "A calm photography studio in the heart of Reykjavík."

	issues := detectLanguageConformanceIssues(plan, "is-IS", verbatimExemption{})
	if len(issues) < 3 {
		t.Fatalf("expected at least 3 leaks flagged, got %d: %#v", len(issues), issues)
	}

	paths := map[string]bool{}
	for _, issue := range issues {
		if issue.Code != languageConformanceIssueCode {
			t.Fatalf("expected code %q, got %q", languageConformanceIssueCode, issue.Code)
		}
		paths[issue.Path] = true
	}
	for _, want := range []string{
		"siteGoal",
		"pages[0].seo.description",
		"pages[0].blocks[0].props.subheadline",
	} {
		if !paths[want] {
			t.Errorf("expected a leak at %q, got paths %#v", want, paths)
		}
	}
}

func TestDetectLanguageConformanceExemptsVerbatimUserText(t *testing.T) {
	plan := icelandicPlan()
	// The user asked, verbatim, to keep an English tagline. It must not be flagged.
	plan.Pages[0].Blocks[0].Props["subheadline"] = "Made with love and light"

	exempt := verbatimExemptionFromInput(generationInputContext{
		Prompt: "Icelandic photography studio, but keep the slogan 'Made with love and light' exactly.",
	})
	issues := detectLanguageConformanceIssues(plan, "is", exempt)
	if len(issues) != 0 {
		t.Fatalf("expected verbatim user text to be exempt, got %#v", issues)
	}

	// Without the user having supplied it, the same English slips get flagged.
	if got := detectLanguageConformanceIssues(plan, "is", verbatimExemption{}); len(got) == 0 {
		t.Fatalf("expected the English tagline to be flagged when not user-supplied")
	}
}

func TestDetectLanguageConformancePassesProperNouns(t *testing.T) {
	plan := icelandicPlan()
	// A brand name made of English-looking words but carrying no function words.
	plan.Pages[0].Blocks[0].Props["headline"] = "Reykjavik Northern Lights Studio"
	plan.Pages[0].Blocks[0].Props["subheadline"] = "Blue Lagoon"

	issues := detectLanguageConformanceIssues(plan, "is", verbatimExemption{})
	if len(issues) != 0 {
		t.Fatalf("expected proper nouns to pass, got %#v", issues)
	}
}

func TestDetectLanguageConformanceSkipsStructuralProps(t *testing.T) {
	plan := icelandicPlan()
	// English-looking values under structural keys must never be scanned.
	plan.Pages[0].Blocks[0].Props["primaryCta"] = map[string]any{
		"label": "Hafðu samband",
		"href":  "https://example.com/learn-and-book-with-the-team",
	}
	plan.Pages[0].Blocks = append(plan.Pages[0].Blocks, generationBlockPlan{
		Type: "footer",
		Props: map[string]any{
			"tagline": "Handverk og hlýja",
			"contact": map[string]any{
				"email": "hello-and-welcome-with-you@example.com",
			},
		},
	})

	issues := detectLanguageConformanceIssues(plan, "is", verbatimExemption{})
	for _, issue := range issues {
		if strings.Contains(issue.Path, "href") {
			t.Fatalf("structural href must not be scanned, got %#v", issue)
		}
	}
	// email is copy-ish but the address carries no whole-word English function
	// word (tokens are joined by hyphens inside one field), so it should pass.
	if len(issues) != 0 {
		t.Fatalf("expected structural/url props to be skipped, got %#v", issues)
	}
}

func TestDetectLanguageConformanceNoOpOutsideIcelandic(t *testing.T) {
	plan := icelandicPlan()
	plan.SiteGoal = "Help visitors and book with the team."
	for _, locale := range []string{"en", "en-US", "", "  ", "de", "sv"} {
		if issues := detectLanguageConformanceIssues(plan, locale, verbatimExemption{}); issues != nil {
			t.Errorf("detectLanguageConformanceIssues(locale=%q) = %#v, want nil", locale, issues)
		}
	}
}

// TestGenerateRetriesOnLanguageConformanceLeak drives the legacy retry loop end
// to end: a planner that leaks English on the first Icelandic attempt and writes
// native copy on the second must run twice, with the language issues threaded
// into the second attempt's feedback.
func TestGenerateRetriesOnLanguageConformanceLeak(t *testing.T) {
	store := newFakeGenerationStore()
	feedbacks := []generationPlanFeedback{}

	service := Service{
		db:     store,
		writer: store,
		planner: func(_ context.Context, input generationInputContext, feedback generationPlanFeedback) (generationPlan, error) {
			feedbacks = append(feedbacks, feedback)
			if feedback.Attempt == 1 {
				plan := icelandicPlan()
				plan.Pages[0].Blocks[0].Props["headline"] = "Welcome to the calm photography studio for your family"
				return plan, nil
			}
			return icelandicPlan(), nil
		},
	}

	result, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:              "Norðurljós Stúdíó",
		Prompt:            "Róleg ljósmyndastofa sem þarf myndasafn og fyrirspurnarform.",
		PreferredLanguage: "is",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if result.Draft.Site.ID == "" {
		t.Fatalf("expected a saved draft after the language retry, got %#v", result.Draft.Site)
	}
	if len(feedbacks) != 2 {
		t.Fatalf("expected planner to run twice, got %d", len(feedbacks))
	}
	if len(feedbacks[0].ValidationIssues) != 0 {
		t.Fatalf("expected first attempt to start with no feedback, got %#v", feedbacks[0].ValidationIssues)
	}
	if len(feedbacks[1].ValidationIssues) == 0 {
		t.Fatalf("expected the language leak to be fed back on the second attempt")
	}
	for _, issue := range feedbacks[1].ValidationIssues {
		if issue.Code != languageConformanceIssueCode {
			t.Fatalf("expected forwarded issue code %q, got %#v", languageConformanceIssueCode, issue)
		}
	}
}
