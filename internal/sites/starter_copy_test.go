package sites

import (
	"strings"
	"testing"
)

// TestStarterDraftLocalized confirms the deterministic starter draft renders in
// the requested content locale rather than always in English (Spec 22).
func TestStarterDraftLocalized(t *testing.T) {
	draft, err := starterDraft("Snælda", "snaelda", "", "is")
	if err != nil {
		t.Fatalf("starterDraft: %v", err)
	}

	if draft.Site.DefaultLocale != "is" {
		t.Fatalf("default locale = %q, want is", draft.Site.DefaultLocale)
	}
	if got := draft.Navigation.Primary[0].Label; got != "Heim" {
		t.Fatalf("nav label = %q, want Heim", got)
	}
	if got := draft.Pages[0].Title; got != "Heim" {
		t.Fatalf("page title = %q, want Heim", got)
	}

	hero := draft.Pages[0].Blocks[0]
	headline, _ := hero.Props["headline"].(string)
	if !strings.Contains(headline, "Hlýleg fyrstu drög fyrir Snælda") {
		t.Fatalf("hero headline not localized: %q", headline)
	}
	subheadline, _ := hero.Props["subheadline"].(string)
	if !strings.Contains(subheadline, "skipulögðum drögum") {
		t.Fatalf("subheadline not localized: %q", subheadline)
	}
}

// TestStarterDraftEnglishDefault confirms unsupported/empty locales fall back to
// English copy, preserving existing behavior.
func TestStarterDraftEnglishDefault(t *testing.T) {
	draft, err := starterDraft("Studio", "studio", "", "")
	if err != nil {
		t.Fatalf("starterDraft: %v", err)
	}
	if draft.Site.DefaultLocale != "en" {
		t.Fatalf("default locale = %q, want en", draft.Site.DefaultLocale)
	}
	if got := draft.Pages[0].Title; got != "Home" {
		t.Fatalf("page title = %q, want Home", got)
	}
}
