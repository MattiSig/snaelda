package respin

import (
	"fmt"
	"strings"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// FactCoverageReport is the result of reconciling a generated draft against the
// verbatim facts extraction found on the source. It is a pure, LLM-free
// containment check: does every business-critical fact the owner recognizes
// instantly — the phone, the email, and each named service — still appear
// somewhere in the finished draft?
//
// It exists because the 2026-07-12 QA showed a re-spin reporting
// "succeeded, degraded=false" while shipping a 24/7 emergency plumber with no
// phone, no email, and none of its services. The audit converts that silent
// content loss into a visible, honest signal (Spec 21 fidelity) and gives the
// 50-site QA loop a scorable metric.
type FactCoverageReport struct {
	// MissingPhone holds the extracted phone when it appears nowhere in the
	// draft, else "".
	MissingPhone string
	// MissingEmail holds the extracted email when it appears nowhere in the
	// draft, else "".
	MissingEmail string
	// MissingServices lists extracted service names absent from the draft, in
	// extraction order.
	MissingServices []string
}

// Complete reports whether every checked fact appears in the draft.
func (r FactCoverageReport) Complete() bool {
	return r.MissingPhone == "" && r.MissingEmail == "" && len(r.MissingServices) == 0
}

// Missing returns human-readable labels for every missing fact, suitable for
// logging and for the degradation reason surfaced to QA.
func (r FactCoverageReport) Missing() []string {
	out := make([]string, 0, 2+len(r.MissingServices))
	if r.MissingPhone != "" {
		out = append(out, "phone")
	}
	if r.MissingEmail != "" {
		out = append(out, "email")
	}
	for _, name := range r.MissingServices {
		out = append(out, fmt.Sprintf("service %q", name))
	}
	return out
}

// DegradationReason renders the report as a compact degradation_reason string
// (e.g. `missing_facts: phone, service "Drain Cleaning"`), or "" when coverage
// is complete.
func (r FactCoverageReport) DegradationReason() string {
	missing := r.Missing()
	if len(missing) == 0 {
		return ""
	}
	return "missing_facts: " + strings.Join(missing, ", ")
}

// coveragePhoneDigits is the number of trailing significant phone digits the
// audit matches on. Seven is the Icelandic local-number length; matching the
// suffix tolerates a country-code prefix on either the extracted or rendered
// side without producing false "missing" flags.
const coveragePhoneDigits = 7

// AuditFactCoverage reconciles the generated draft against the verbatim facts
// extraction found, using plain string containment (no LLM call). It walks every
// user-visible string in the draft — page blocks, footer contact, brand name,
// and collection entries (services promoted to a collection live there, not in
// static blocks) — and reports the highest-integrity facts that silently
// vanished during generation.
func AuditFactCoverage(draft siteconfig.SiteDraft, fields ExtractedFields) FactCoverageReport {
	nodes := collectDraftText(draft)

	haystack := normalizeCoverageText(strings.Join(nodes, "\n"))

	// Digit strings per node keep phone matching precise: a phone lives in one
	// text node, so a per-node match avoids the false positives a single
	// concatenated digit blob would create across unrelated numbers.
	nodeDigits := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if d := digitsOnly(node); d != "" {
			nodeDigits = append(nodeDigits, d)
		}
	}

	var report FactCoverageReport

	if phone := strings.TrimSpace(fields.Contact.Phone); phone != "" {
		if !phoneCovered(phone, nodeDigits) {
			report.MissingPhone = phone
		}
	}

	if email := strings.TrimSpace(fields.Contact.Email); email != "" {
		if !strings.Contains(haystack, normalizeCoverageText(email)) {
			report.MissingEmail = email
		}
	}

	for _, svc := range fields.Services {
		name := strings.TrimSpace(svc.Name)
		if name == "" {
			continue
		}
		if !strings.Contains(haystack, normalizeCoverageText(name)) {
			report.MissingServices = append(report.MissingServices, name)
		}
	}

	return report
}

// phoneCovered reports whether the phone's significant digits appear in any
// node. A number too short to audit reliably (fewer than coveragePhoneDigits
// digits) is treated as covered so an oddly formatted value never forces a
// false degradation.
func phoneCovered(phone string, nodeDigits []string) bool {
	want := digitsOnly(phone)
	if len(want) < coveragePhoneDigits {
		return true
	}
	needle := want[len(want)-coveragePhoneDigits:]
	for _, d := range nodeDigits {
		if strings.Contains(d, needle) {
			return true
		}
	}
	return false
}

// collectDraftText gathers every user-visible string in the draft into a flat
// slice for containment checks.
func collectDraftText(draft siteconfig.SiteDraft) []string {
	var out []string
	add := func(s string) {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}

	add(draft.Brand.BusinessName)
	add(draft.Site.Name)
	add(draft.Site.SEO.Title)
	add(draft.Site.SEO.Description)

	for _, item := range draft.Navigation.Primary {
		add(item.Label)
	}
	for _, item := range draft.Navigation.Footer {
		add(item.Label)
	}

	for _, page := range draft.Pages {
		add(page.Title)
		add(page.SEO.Title)
		add(page.SEO.Description)
		for _, block := range page.Blocks {
			collectAnyStrings(block.Props, &out)
		}
	}

	for _, collection := range draft.Collections {
		add(collection.SingularLabel)
		add(collection.PluralLabel)
		for _, entry := range collection.Entries {
			add(entry.SEO.Title)
			add(entry.SEO.Description)
			collectAnyStrings(entry.Fields, &out)
		}
	}

	return out
}

// collectAnyStrings recursively appends every string leaf of an arbitrary
// props/fields value (map, slice, or scalar) to out.
func collectAnyStrings(value any, out *[]string) {
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			*out = append(*out, v)
		}
	case map[string]any:
		for _, item := range v {
			collectAnyStrings(item, out)
		}
	case []any:
		for _, item := range v {
			collectAnyStrings(item, out)
		}
	case []string:
		for _, item := range v {
			collectAnyStrings(item, out)
		}
	}
}

// normalizeCoverageText lowercases and collapses whitespace so multi-word facts
// match regardless of the exact spacing the renderer produced.
func normalizeCoverageText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// digitsOnly returns just the ASCII digits of s.
func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
