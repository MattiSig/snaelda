package siteconfig

import "testing"

func TestFooterValidatesStructuredContact(t *testing.T) {
	registry := DefaultBlockRegistry()

	err := registry.ValidateProps("footer", BlockVersionV1, "props", map[string]any{
		"showBrand":    true,
		"showMadeWith": false,
		"copyright":    "Copyright 2026 Fléttan",
		"contact": map[string]any{
			"address": map[string]any{
				"street":     "Laugavegur 12",
				"city":       "Reykjavík",
				"postalCode": "101",
				"region":     "Höfuðborgarsvæðið",
				"country":    "Ísland",
			},
			"phone": "+354 555 1234",
			"email": "hallo@fléttan.is",
			"hours": []any{
				map[string]any{"day": "monday", "opens": "09:00", "closes": "17:00", "closed": false},
				map[string]any{"day": "sunday", "closed": true},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected structured footer contact to validate, got %v", err)
	}
}

func TestFooterRejectsMalformedStructuredContact(t *testing.T) {
	registry := DefaultBlockRegistry()

	err := registry.ValidateProps("footer", BlockVersionV1, "props", map[string]any{
		"copyright": "Copyright 2026 Fléttan",
		"contact": map[string]any{
			"address": map[string]any{
				"street":  "Laugavegur 12",
				"country": "Ísland",
				"planet":  "Earth",
			},
			"hours": []any{
				map[string]any{"day": "someday", "opens": "9am", "closes": "17:00"},
			},
		},
	})
	if !hasValidationIssue(err, "unknown_property") {
		t.Fatalf("expected unknown address property issue, got %v", err)
	}
	if !hasValidationIssue(err, "invalid_value") {
		t.Fatalf("expected invalid day/time issue, got %v", err)
	}
}

func TestFooterRejectsLegacyStringAddress(t *testing.T) {
	registry := DefaultBlockRegistry()

	err := registry.ValidateProps("footer", BlockVersionV1, "props", map[string]any{
		"copyright": "Copyright 2026 Fléttan",
		"contact": map[string]any{
			"address": "Laugavegur 12, 101 Reykjavík",
		},
	})
	if !hasValidationIssue(err, "invalid_type") {
		t.Fatalf("expected legacy free-text address to be rejected, got %v", err)
	}
}
