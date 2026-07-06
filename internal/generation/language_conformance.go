package generation

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// languageConformanceIssueCode tags the validation issues the language-
// conformance detector produces. It routes through the same
// feedback.ValidationIssues → planner-feedback loop as structural validation,
// so the model sees the offending strings on the next attempt.
const languageConformanceIssueCode = "language_conformance"

// englishFunctionWords is the marker set the detector matches against. Every
// entry is a common English grammatical word (article, pronoun, conjunction,
// preposition, auxiliary/modal, or wh-word) that never occurs in Icelandic.
// Because these words carry no meaning of their own, they are absent from brand
// names, loanwords, and the short deterministic CTA labels the repair pass
// injects — so a single whole-word match is a high-precision signal that a
// phrase was written in English rather than Icelandic (Spec 22). Content words
// are intentionally excluded to keep false positives near zero; the trade-off
// is that a bare English label like "Home" slips through, which the deterministic
// localized-copy path handles separately.
var englishFunctionWords = map[string]bool{
	"the": true, "and": true, "with": true, "your": true, "you": true,
	"our": true, "for": true, "are": true, "was": true, "were": true,
	"been": true, "this": true, "that": true, "these": true, "those": true,
	"they": true, "them": true, "their": true, "will": true, "would": true,
	"should": true, "could": true, "about": true, "please": true, "have": true,
	"has": true, "had": true, "what": true, "when": true, "where": true,
	"which": true, "who": true, "whom": true, "whose": true, "why": true,
	"how": true, "into": true, "over": true, "than": true, "then": true,
	"because": true, "before": true, "after": true, "while": true, "from": true,
}

// nonCopyPropKeys are block-prop keys whose values are structural (URLs, asset
// references, enum tokens, identifiers) rather than user-visible copy. The
// walker skips them so the detector never scans a slug or href for English.
var nonCopyPropKeys = map[string]bool{
	"href": true, "url": true, "src": true, "assetId": true, "slug": true,
	"icon": true, "type": true, "variant": true, "layout": true,
	"alignment": true, "imagePosition": true, "width": true, "columns": true,
	"target": true, "rel": true, "id": true, "primaryColor": true, "color": true,
	"email": true, "phone": true,
}

// verbatimExemption holds the lowercased corpus of text the visitor supplied, so
// the detector can exempt copy the user asked for verbatim (Spec 22 exempts
// user-provided text alongside proper nouns). A generation input rarely reaches
// the draft unchanged, so this is a safety net rather than a common path.
type verbatimExemption struct {
	corpus string
}

// exempt reports whether the candidate string should be skipped: empty strings
// and any text contained verbatim in the user's own input are never flagged.
func (v verbatimExemption) exempt(candidate string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(candidate))
	if trimmed == "" {
		return true
	}
	return v.corpus != "" && strings.Contains(v.corpus, trimmed)
}

// verbatimExemptionFromInput builds the exemption corpus from every field the
// visitor authored: the prompt, name/slug hints, brand name, optional hints, and
// interview answers.
func verbatimExemptionFromInput(input generationInputContext) verbatimExemption {
	var b strings.Builder
	write := func(value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		b.WriteString(strings.ToLower(value))
		b.WriteByte('\n')
	}
	write(input.Prompt)
	write(input.NameHint)
	write(input.SlugHint)
	write(input.Brand.BusinessName)
	for _, value := range input.OptionalHints {
		write(value)
	}
	for _, answer := range input.InterviewAnswers {
		write(answer.Prompt)
		write(answer.Text)
		for _, option := range answer.SelectedOptions {
			write(option)
		}
	}
	return verbatimExemption{corpus: b.String()}
}

// detectLanguageConformanceIssues walks the user-visible copy in a generation
// plan and returns a validation issue for every string that leaked English into
// a draft whose content locale requires another language. Per Spec 22 only the
// Icelandic phase is enforced; English and unknown/empty locales return nil so
// the detector is a no-op everywhere else. Proper nouns are skipped implicitly
// (they carry no function words) and verbatim user text is exempted.
func detectLanguageConformanceIssues(plan generationPlan, preferredLanguage string, exempt verbatimExemption) []siteconfig.Issue {
	if normalizeLocale(preferredLanguage) != "is" {
		return nil
	}

	var issues []siteconfig.Issue
	add := func(path, text string) {
		if exempt.exempt(text) {
			return
		}
		word, ok := firstEnglishMarker(text)
		if !ok {
			return
		}
		issues = append(issues, siteconfig.Issue{
			Path: path,
			Code: languageConformanceIssueCode,
			Message: fmt.Sprintf(
				"content locale is Icelandic but %s reads as English (found %q in %q); rewrite it in natural Icelandic",
				path, word, conformanceSnippet(text),
			),
		})
	}

	add("siteGoal", plan.SiteGoal)
	for pageIndex, page := range plan.Pages {
		base := fmt.Sprintf("pages[%d]", pageIndex)
		add(base+".title", page.Title)
		add(base+".seo.title", page.SEO.Title)
		add(base+".seo.description", page.SEO.Description)
		for blockIndex, block := range page.Blocks {
			walkPropsForCopy(fmt.Sprintf("%s.blocks[%d].props", base, blockIndex), block.Props, add)
		}
	}
	return issues
}

// walkPropsForCopy recurses through a block's prop tree, invoking add for each
// user-visible string value while skipping structural keys.
func walkPropsForCopy(path string, value any, add func(path, text string)) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if nonCopyPropKeys[key] {
				continue
			}
			walkPropsForCopy(conformanceChildPath(path, key), child, add)
		}
	case []any:
		for index, item := range typed {
			walkPropsForCopy(fmt.Sprintf("%s[%d]", path, index), item, add)
		}
	case string:
		add(path, typed)
	}
}

// firstEnglishMarker returns the first English function word found in text as a
// whole word, matching case-insensitively. Tokenisation splits on any
// non-letter, so Icelandic letters stay attached to their word and never split a
// token into a spurious English match.
func firstEnglishMarker(text string) (string, bool) {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r)
	})
	for _, field := range fields {
		if englishFunctionWords[strings.ToLower(field)] {
			return strings.ToLower(field), true
		}
	}
	return "", false
}

func conformanceChildPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// conformanceSnippet trims copy to a short, message-friendly excerpt.
func conformanceSnippet(text string) string {
	const maxRunes = 80
	trimmed := strings.TrimSpace(text)
	runes := []rune(trimmed)
	if len(runes) <= maxRunes {
		return trimmed
	}
	return string(runes[:maxRunes]) + "…"
}
