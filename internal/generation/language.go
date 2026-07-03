package generation

import "strings"

// normalizeLocale reduces a BCP-47-ish locale string to its lowercase primary
// subtag, so "is-IS", "IS", and "is" all resolve to "is". An empty or malformed
// value yields "".
func normalizeLocale(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if idx := strings.IndexAny(trimmed, "-_"); idx > 0 {
		trimmed = trimmed[:idx]
	}
	return strings.ToLower(trimmed)
}

// languageDirective returns a system-prompt fragment that makes the site's
// content locale a hard output contract for every copy-producing generation
// stage (Spec 22). Callers append the result to the stage's system prompt; the
// leading blank line keeps it visually separated from the base prompt.
//
// English needs no reinforcement — it is the model's default — so "en" and
// unknown/empty locales yield an empty string and leave the prompt untouched.
func languageDirective(preferredLanguage string) string {
	switch normalizeLocale(preferredLanguage) {
	case "is":
		return "\n\n" + icelandicLanguageDirective
	default:
		return ""
	}
}

// icelandicLanguageDirective is the per-locale tone-and-language fragment for
// Icelandic sites. It covers the copy contract, the small-business register from
// Spec 22, and the deterministic slug transliteration table (so slug-producing
// stages — outline, collection, and entry drafters — emit ASCII slugs that match
// the backend slugifier in internal/platform/slugs).
const icelandicLanguageDirective = `LANGUAGE CONTRACT (hard requirement — not a hint):
Write EVERY user-visible string in natural, native Icelandic (íslenska): headlines, subheadlines, body copy, button and link labels, navigation labels, FAQ questions and answers, feature/service/pricing text, testimonials, SEO titles and meta descriptions, image alt text, and any changeSummary you emit.
Tone: the direct, warm, unpretentious small-business register a Reykjavík salon, café, or contractor would actually use. Never translated-English phrasing, no hype or superlatives, and no anglicism where a natural Icelandic word exists.
Never leave English text in the output. The only exceptions are proper nouns, brand names, and verbatim text the user supplied.
Slugs and other URL fields stay ASCII: transliterate Icelandic letters deterministically (þ→th, ð→d, æ→ae, ö→o, and á/é/í/ó/ú/ý→a/e/i/o/u/y). For example "Þjónusta" becomes "thjonusta" and "Verkefni í Reykjavík" becomes "verkefni-i-reykjavik".`
