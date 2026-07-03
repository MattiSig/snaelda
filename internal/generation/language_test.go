package generation

import "testing"

func TestNormalizeLocale(t *testing.T) {
	cases := map[string]string{
		"is":       "is",
		"is-IS":    "is",
		"IS":       "is",
		"is_IS":    "is",
		"en":       "en",
		"en-US":    "en",
		"  is  ":   "is",
		"":         "",
		"   ":      "",
		"-":        "-",
		"pt-BR-x1": "pt",
	}
	for input, want := range cases {
		if got := normalizeLocale(input); got != want {
			t.Errorf("normalizeLocale(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLanguageDirectiveIcelandic(t *testing.T) {
	for _, locale := range []string{"is", "is-IS", "IS", "is_IS"} {
		got := languageDirective(locale)
		if got == "" {
			t.Fatalf("languageDirective(%q) returned empty; expected Icelandic directive", locale)
		}
		for _, marker := range []string{"íslenska", "thjonusta", "LANGUAGE CONTRACT"} {
			if !contains(got, marker) {
				t.Errorf("languageDirective(%q) missing %q", locale, marker)
			}
		}
	}
}

func TestLanguageDirectiveEnglishAndUnknownAreEmpty(t *testing.T) {
	for _, locale := range []string{"en", "en-US", "", "  ", "de", "sv"} {
		if got := languageDirective(locale); got != "" {
			t.Errorf("languageDirective(%q) = %q, want empty", locale, got)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
