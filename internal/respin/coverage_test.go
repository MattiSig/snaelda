package respin

import (
	"strings"
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// draftWithText builds a minimal draft carrying a single text block whose body
// is the given copy, for coverage-audit assertions.
func draftWithText(body string) siteconfig.SiteDraft {
	return siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{ID: "site-1", Name: "Test"},
		Pages: []siteconfig.PageDraft{{
			Blocks: []siteconfig.BlockInstance{{
				Type:  "text_section",
				Props: map[string]any{"heading": "About", "body": body},
			}},
		}},
	}
}

func TestAuditFactCoverageAllFactsPresent(t *testing.T) {
	fields := ExtractedFields{
		Contact: ContactDetails{Phone: "555-1234", Email: "hi@example.com"},
		Services: []ExtractService{
			{Name: "Drain Cleaning"},
			{Name: "Emergency Repair"},
		},
	}
	draft := draftWithText("Call 555 1234 or email hi@example.com. We offer drain cleaning and emergency repair around the clock.")

	report := AuditFactCoverage(draft, fields)
	if !report.Complete() {
		t.Fatalf("expected complete coverage, got missing: %v", report.Missing())
	}
	if report.DegradationReason() != "" {
		t.Fatalf("expected empty degradation reason, got %q", report.DegradationReason())
	}
}

func TestAuditFactCoverageMissingContactAndService(t *testing.T) {
	fields := ExtractedFields{
		Contact: ContactDetails{Phone: "555-1234", Email: "hi@example.com"},
		Services: []ExtractService{
			{Name: "Drain Cleaning"},
			{Name: "Emergency Repair"},
		},
	}
	// Draft mentions only one service and neither contact fact.
	draft := draftWithText("We do emergency repair work for homeowners across the region.")

	report := AuditFactCoverage(draft, fields)
	if report.Complete() {
		t.Fatal("expected incomplete coverage")
	}
	if report.MissingPhone != "555-1234" {
		t.Fatalf("expected missing phone, got %q", report.MissingPhone)
	}
	if report.MissingEmail != "hi@example.com" {
		t.Fatalf("expected missing email, got %q", report.MissingEmail)
	}
	if len(report.MissingServices) != 1 || report.MissingServices[0] != "Drain Cleaning" {
		t.Fatalf("expected only Drain Cleaning missing, got %v", report.MissingServices)
	}
	reason := report.DegradationReason()
	if !strings.HasPrefix(reason, "missing_facts:") ||
		!strings.Contains(reason, "phone") ||
		!strings.Contains(reason, "email") ||
		!strings.Contains(reason, `service "Drain Cleaning"`) {
		t.Fatalf("degradation reason missing detail: %q", reason)
	}
}

func TestAuditFactCoveragePhoneToleratesFormattingAndCountryCode(t *testing.T) {
	fields := ExtractedFields{Contact: ContactDetails{Phone: "+354 555 1234"}}

	// Rendered without the country code and with different spacing.
	if r := AuditFactCoverage(draftWithText("Hringdu í síma 555-1234."), fields); !r.Complete() {
		t.Fatalf("expected phone matched across formatting, got %v", r.Missing())
	}
	// A different number must still be flagged.
	if r := AuditFactCoverage(draftWithText("Hringdu í síma 555-9999."), fields); r.MissingPhone == "" {
		t.Fatal("expected a different phone number to be flagged missing")
	}
}

func TestAuditFactCoverageFindsFactsInCollectionEntries(t *testing.T) {
	// When services are promoted to a collection, the names live in entries, not
	// static blocks. The audit must count entry values as valid landing spots.
	fields := ExtractedFields{
		Services: []ExtractService{{Name: "Drain Cleaning"}, {Name: "Hydro Jetting"}},
	}
	draft := siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{ID: "site-1"},
		Pages: []siteconfig.PageDraft{{
			Blocks: []siteconfig.BlockInstance{{
				Type:  "hero",
				Props: map[string]any{"headline": "Clogged drain? We fix them."},
			}},
		}},
		Collections: []siteconfig.Collection{{
			ID:          "col-services",
			PluralLabel: "Services",
			Entries: []siteconfig.CollectionEntry{
				{Fields: map[string]any{"title": "Drain Cleaning", "price": "9.900 kr."}},
				{Fields: map[string]any{"title": "Hydro Jetting", "price": "14.900 kr."}},
			},
		}},
	}

	if r := AuditFactCoverage(draft, fields); !r.Complete() {
		t.Fatalf("expected collection entries to satisfy coverage, got %v", r.Missing())
	}
}

func TestAuditFactCoverageNoExtractedFactsIsComplete(t *testing.T) {
	if r := AuditFactCoverage(draftWithText("anything"), ExtractedFields{}); !r.Complete() {
		t.Fatalf("expected complete coverage when nothing was extracted, got %v", r.Missing())
	}
}
