package slugs

import "testing"

func TestGenerateNormalizesHumanText(t *testing.T) {
	tests := map[string]string{
		"  Nordic Studio!  ": "nordic-studio",
		"Snælda & Þing":      "snaelda-thing",
		"already-valid":      "already-valid",
		"---":                "untitled",
	}

	for input, expected := range tests {
		if got := Generate(input); got != expected {
			t.Fatalf("Generate(%q) = %q, expected %q", input, got, expected)
		}
	}
}

func TestGenerateLimitsLengthWithoutTrailingDash(t *testing.T) {
	input := "this is a very long title with enough repeated words to exceed the supported slug limit"
	slug := Generate(input)

	if len(slug) > MaxLength {
		t.Fatalf("expected slug length at most %d, got %d", MaxLength, len(slug))
	}
	if slug[len(slug)-1] == '-' {
		t.Fatalf("expected slug without trailing dash, got %q", slug)
	}
}

func TestEnsureUniqueAddsNumericSuffix(t *testing.T) {
	taken := map[string]bool{
		"nordic-studio":   true,
		"nordic-studio-2": true,
	}

	slug, err := EnsureUnique("Nordic Studio", func(candidate string) (bool, error) {
		return taken[candidate], nil
	})
	if err != nil {
		t.Fatalf("ensure unique: %v", err)
	}
	if slug != "nordic-studio-3" {
		t.Fatalf("expected nordic-studio-3, got %q", slug)
	}
}

func TestPagePathUsesRootForHome(t *testing.T) {
	if got := PagePath("Home"); got != "/" {
		t.Fatalf("expected root path, got %q", got)
	}
	if got := PagePath("About Us"); got != "/about-us" {
		t.Fatalf("expected about path, got %q", got)
	}
}
