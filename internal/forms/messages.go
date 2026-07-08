package forms

import (
	"fmt"
	"strings"
)

// formCopy holds the locale-specific, visitor-facing strings a published site
// emits when someone submits a contact form: the success confirmation and every
// validation message. These strings surface to end visitors of the published
// site, so they must follow the site's own content locale (Spec 22: English
// must never leak into an Icelandic site), not the builder's UI locale.
//
// Validation messages interpolate the form field's label, which is authored by
// the site owner and already conforms to the site locale, so the Icelandic
// variants wrap the label in Icelandic quotation marks and phrase the sentence
// to sidestep grammatical gender agreement with an arbitrary label.
type formCopy struct {
	successMessage string
	required       func(label string) string
	invalidType    func(label string) string
	lengthRange    func(label string, min, max int) string
	invalidEmail   func(label string) string
	invalidPhone   func(label string) string
	invalidOption  func(label string) string
	unsupported    func(label string) string
	unknownField   string
}

var formCopyEN = formCopy{
	successMessage: "Thanks. Your message is on its way.",
	required:       func(label string) string { return label + " is required" },
	invalidType:    func(label string) string { return label + " must be a string" },
	lengthRange: func(label string, min, max int) string {
		return fmt.Sprintf("%s must be between %d and %d characters", label, min, max)
	},
	invalidEmail:  func(label string) string { return label + " must be a valid email address" },
	invalidPhone:  func(label string) string { return label + " must be a valid phone number" },
	invalidOption: func(label string) string { return label + " must use one of the configured options" },
	unsupported:   func(label string) string { return label + " is not supported" },
	unknownField:  "field is not supported by this form",
}

var formCopyIS = formCopy{
	successMessage: "Takk fyrir. Skilaboðin þín eru á leiðinni.",
	required:       func(label string) string { return "Vinsamlegast fylltu út „" + label + "“." },
	invalidType:    func(label string) string { return "„" + label + "“ verður að vera texti." },
	lengthRange: func(label string, min, max int) string {
		return fmt.Sprintf("„%s“ verður að vera á milli %d og %d stafa.", label, min, max)
	},
	invalidEmail: func(label string) string { return "„" + label + "“ verður að vera gilt netfang." },
	invalidPhone: func(label string) string { return "„" + label + "“ verður að vera gilt símanúmer." },
	invalidOption: func(label string) string {
		return "„" + label + "“ verður að nota einn af uppgefnum valmöguleikum."
	},
	unsupported:  func(label string) string { return "„" + label + "“ er ekki stutt." },
	unknownField: "reiturinn er ekki studdur af þessu formi.",
}

// formCopyForLocale resolves the message catalog for a published site's default
// locale. Locales are reduced to their primary subtag ("is-IS" -> "is"), and
// anything outside the supported set falls back to English.
func formCopyForLocale(locale string) formCopy {
	switch normalizeFormLocale(locale) {
	case "is":
		return formCopyIS
	default:
		return formCopyEN
	}
}

func normalizeFormLocale(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if idx := strings.IndexAny(trimmed, "-_"); idx > 0 {
		trimmed = trimmed[:idx]
	}
	return strings.ToLower(trimmed)
}
