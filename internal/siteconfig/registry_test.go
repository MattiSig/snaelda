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
	if len(definitions) != 5 {
		t.Fatalf("expected five definitions, got %d", len(definitions))
	}

	got := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		got = append(got, definition.Type+"@"+definition.Version)
	}

	want := []string{
		"cta_band@1.0.0",
		"features_grid@1.0.0",
		"hero@1.0.0",
		"image_text@1.0.0",
		"text_section@1.0.0",
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("expected sorted definitions %v, got %v", want, got)
		}
	}
}

func hasValidationIssue(err error, code string) bool {
	validationErr, ok := err.(ValidationError)
	return ok && validationErr.Has(code)
}
