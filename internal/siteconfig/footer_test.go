package siteconfig

import (
	"strings"
	"testing"
)

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

func TestFooterToleratesLegacyContactShapes(t *testing.T) {
	registry := DefaultBlockRegistry()

	// Drafts generated before the structured contact contract stored the
	// address as one free-text line and hours as free-text entries. Rejecting
	// those shapes bricked every pre-contract site on load (Kaffi Krús
	// incident, 2026-07-13): the renderer tolerates them, so validation must.
	err := registry.ValidateProps("footer", BlockVersionV1, "props", map[string]any{
		"copyright": "Copyright 2026 Fléttan",
		"contact": map[string]any{
			"address": "Laugavegur 12, 101 Reykjavík",
			"phone":   "+354 555 0123",
			"email":   "postur@kaffikrus.is",
			"hours": []any{
				"Mán–Fös 08:00–17:00",
				"Lau 09:00–15:00",
				"Sun 10:00–14:00",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected legacy contact shapes to validate, got %v", err)
	}

	// Legacy tolerance is bounded: absurdly long free-text still fails.
	err = registry.ValidateProps("footer", BlockVersionV1, "props", map[string]any{
		"copyright": "Copyright 2026 Fléttan",
		"contact": map[string]any{
			"address": strings.Repeat("x", 301),
		},
	})
	if !hasValidationIssue(err, "invalid_length") {
		t.Fatalf("expected over-long legacy address to be rejected, got %v", err)
	}
}
