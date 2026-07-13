package siteconfig

import (
	"errors"
	"strings"
	"testing"
)

func TestNewBlockRegistryRejectsInvalidDefinitions(t *testing.T) {
	validator := func(string, map[string]any, *collector) {}

	tests := []struct {
		name string
		defs []BlockDefinition
		want string
	}{
		{
			name: "missing type",
			defs: []BlockDefinition{{
				Version:       BlockVersionV1,
				ValidateProps: validator,
			}},
			want: "type is required",
		},
		{
			name: "missing version",
			defs: []BlockDefinition{{
				Type:          "hero",
				ValidateProps: validator,
			}},
			want: "version is required",
		},
		{
			name: "missing props validator",
			defs: []BlockDefinition{{
				Type:    "hero",
				Version: BlockVersionV1,
			}},
			want: "props validator is required",
		},
		{
			name: "duplicate definition",
			defs: []BlockDefinition{
				{
					Type:          "hero",
					Version:       BlockVersionV1,
					ValidateProps: validator,
				},
				{
					Type:          "hero",
					Version:       BlockVersionV1,
					ValidateProps: validator,
				},
			},
			want: "duplicate definition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewBlockRegistry(tt.defs...)
			if err == nil {
				t.Fatal("expected registry build to fail")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestBlockRegistryValidatePropsRejectsUnknownVersion(t *testing.T) {
	registry := DefaultBlockRegistry()

	err := registry.ValidateProps("hero", "9.9.9", "props", map[string]any{
		"headline": "Launch faster",
	})
	if !errors.Is(err, ErrBlockVersionUnknown) {
		t.Fatalf("expected unknown version, got %v", err)
	}
}

func TestBlockRegistryValidatePropsRejectsInvalidHeroContract(t *testing.T) {
	registry := DefaultBlockRegistry()

	err := registry.ValidateProps("hero", BlockVersionV1, "props", map[string]any{
		"headline": "Launch faster",
		"layout":   "diagonal",
		"primaryCta": map[string]any{
			"label": "Run it",
			"href":  "javascript:alert(1)",
		},
		"script": "alert(1)",
	})
	if !hasValidationIssue(err, "invalid_value") {
		t.Fatalf("expected invalid layout issue, got %v", err)
	}
	if !hasValidationIssue(err, "unsafe_url") {
		t.Fatalf("expected unsafe url issue, got %v", err)
	}
	if !hasValidationIssue(err, "unknown_property") {
		t.Fatalf("expected unknown property issue, got %v", err)
	}
}

func TestBlockRegistryDefinitionsAreSorted(t *testing.T) {
	registry := DefaultBlockRegistry()

	definitions := registry.Definitions()
	want := []string{
		"collection_detail@1.0.0",
		"collection_index@1.0.0",
		"collection_list@1.0.0",
		"contact_form@1.0.0",
		"cta_band@1.0.0",
		"faq@1.0.0",
		"features_grid@1.0.0",
		"footer@1.0.0",
		"gallery@1.0.0",
		"hero@1.1.0",
		"image_text@1.0.0",
		"pricing_packages@1.0.0",
		"stats@1.0.0",
		"team_profile_cards@1.0.0",
		"testimonials@1.0.0",
		"text_section@1.0.0",
	}
	if len(definitions) != len(want) {
		t.Fatalf("expected %d definitions, got %d", len(want), len(definitions))
	}

	got := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		got = append(got, definition.Type+"@"+definition.Version)
	}

	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("expected sorted definitions %v, got %v", want, got)
		}
	}
}

func TestBlockRegistryHeroVersions(t *testing.T) {
	registry := DefaultBlockRegistry()

	// Stored drafts carry hero@1.0.0 and must keep resolving.
	legacy, err := registry.Lookup("hero", BlockVersionV1)
	if err != nil {
		t.Fatalf("lookup hero@1.0.0: %v", err)
	}
	if legacy.Version != BlockVersionV1 {
		t.Fatalf("expected legacy hero version %q, got %q", BlockVersionV1, legacy.Version)
	}

	latest, err := registry.Latest("hero")
	if err != nil {
		t.Fatalf("latest hero: %v", err)
	}
	if latest.Version != HeroBlockVersion {
		t.Fatalf("expected latest hero version %q, got %q", HeroBlockVersion, latest.Version)
	}
	if LatestBlockVersion("hero") != HeroBlockVersion {
		t.Fatalf("expected LatestBlockVersion(hero) %q, got %q", HeroBlockVersion, LatestBlockVersion("hero"))
	}
	if LatestBlockVersion("cta_band") != BlockVersionV1 {
		t.Fatalf("expected LatestBlockVersion(cta_band) %q, got %q", BlockVersionV1, LatestBlockVersion("cta_band"))
	}
	if _, err := registry.Latest("bogus"); err == nil {
		t.Fatal("expected unknown-type error for Latest(bogus)")
	}

	// The statement variant is additive: it validates on both registered versions.
	props := map[string]any{"variant": "statement", "headline": "Pipes fixed, promises kept"}
	for _, version := range []string{BlockVersionV1, HeroBlockVersion} {
		if err := registry.ValidateProps("hero", version, "props", props); err != nil {
			t.Fatalf("statement variant should validate on hero@%s: %v", version, err)
		}
	}
	if err := registry.ValidateProps("hero", HeroBlockVersion, "props", map[string]any{"variant": "poster", "headline": "x"}); err == nil {
		t.Fatal("expected unknown variant to fail validation")
	}
}

func TestCompareBlockVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.1.0", -1},
		{"1.1.0", "1.0.0", 1},
		{"1.1.0", "1.1.0", 0},
		{"1.10.0", "1.9.0", 1},
		{"2.0.0", "1.99.0", 1},
	}
	for _, c := range cases {
		if got := compareBlockVersions(c.a, c.b); got != c.want {
			t.Fatalf("compareBlockVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestBlockRegistryValidatePropsRejectsInvalidContactFormContract(t *testing.T) {
	registry := DefaultBlockRegistry()

	err := registry.ValidateProps("contact_form", BlockVersionV1, "props", map[string]any{
		"heading":     "Reach out",
		"submitLabel": "Send",
		"fields": []any{
			map[string]any{
				"name":     "contact_reason",
				"label":    "Reason",
				"type":     "select",
				"required": true,
			},
		},
		"notificationEmail": "not-an-email",
	})
	if !hasValidationIssue(err, "required") {
		t.Fatalf("expected select options issue, got %v", err)
	}
	if !hasValidationIssue(err, "invalid_email") {
		t.Fatalf("expected invalid notification email issue, got %v", err)
	}
}

func hasValidationIssue(err error, code string) bool {
	validationErr, ok := err.(ValidationError)
	return ok && validationErr.Has(code)
}
