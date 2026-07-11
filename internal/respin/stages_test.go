package respin

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeCompleter returns canned JSON responses keyed by request name and records
// every request it received, so tests can assert both outputs and prompt wiring.
type fakeCompleter struct {
	responses map[string]string
	tokens    int
	err       error
	requests  []CompletionRequest
}

func (f *fakeCompleter) Complete(_ context.Context, req CompletionRequest) (CompletionResult, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return CompletionResult{}, f.err
	}
	content, ok := f.responses[req.Name]
	if !ok {
		return CompletionResult{}, errors.New("no canned response for " + req.Name)
	}
	return CompletionResult{Content: content, TotalTokens: f.tokens}, nil
}

func sampleContent() SourceContent {
	return BuildSourceContent(Page{
		URL:       "https://klippt.is",
		FinalURL:  "https://klippt.is/",
		Title:     "Hárgreiðslustofan Klippt",
		Text:      "Við klippum og litum hár í hjarta Reykjavíkur. Opið alla virka daga. Hringdu í 555-1234.",
		WordCount: 16,
		Meta:      PageMeta{Lang: "is-IS"},
	})
}

func TestClassifyNormalizesResult(t *testing.T) {
	completer := &fakeCompleter{
		responses: map[string]string{
			"respin_classification": `{"vertical":"  Salon ","services":["Klipping","klipping","Litun"],"locale":"is-IS","tone":"warm and homey","confidence":0.9}`,
		},
		tokens: 120,
	}
	analyzer := NewAnalyzer(completer)

	result, err := analyzer.Classify(context.Background(), sampleContent())
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if result.Vertical != "salon" {
		t.Fatalf("vertical not lowercased/trimmed: %q", result.Vertical)
	}
	if result.Locale != "is" {
		t.Fatalf("locale not reduced to primary subtag: %q", result.Locale)
	}
	if len(result.Services) != 2 { // deduped case-insensitively
		t.Fatalf("expected 2 deduped services, got %v", result.Services)
	}
	if result.LowConfidence() {
		t.Fatal("0.9 confidence should not be low")
	}
	// The source document must reach the model.
	if len(completer.requests) != 1 || !contains(completer.requests[0].User, "555-1234") {
		t.Fatalf("source text was not sent to the classifier: %+v", completer.requests)
	}
}

func TestClassifyLowConfidenceFlagged(t *testing.T) {
	completer := &fakeCompleter{responses: map[string]string{
		"respin_classification": `{"vertical":"","services":[],"locale":"is","tone":"","confidence":0.2}`,
	}}
	result, err := NewAnalyzer(completer).Classify(context.Background(), sampleContent())
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if !result.LowConfidence() {
		t.Fatal("empty vertical + 0.2 confidence should be low")
	}
}

func TestClassifyInsufficientContent(t *testing.T) {
	analyzer := NewAnalyzer(&fakeCompleter{})
	if _, err := analyzer.Classify(context.Background(), SourceContent{Pages: []SourcePage{}}); !errors.Is(err, ErrInsufficientContent) {
		t.Fatalf("expected ErrInsufficientContent, got %v", err)
	}
}

func TestClassifyNoCompleter(t *testing.T) {
	analyzer := NewAnalyzer(nil)
	if _, err := analyzer.Classify(context.Background(), sampleContent()); !errors.Is(err, ErrCompleterUnavailable) {
		t.Fatalf("expected ErrCompleterUnavailable, got %v", err)
	}
}

func TestExtractFieldsNormalizesAndFlagsMissing(t *testing.T) {
	completer := &fakeCompleter{responses: map[string]string{
		"respin_extraction": `{
			"businessName":" Klippt ",
			"tagline":"",
			"about":"Hárstofa í miðbænum.",
			"services":[{"name":" Klipping ","description":"","price":"6.900 kr."},{"name":"","description":"drop me","price":""}],
			"hours":[{"day":"Mánudagur","opens":"9","closes":"17.30","closed":false},{"day":"sun","opens":"","closes":"","closed":true},{"day":"garbage","opens":"","closes":"","closed":false}],
			"contact":{"phone":"555-1234","email":" ","address":"Laugavegur 1"},
			"testimonials":[{"quote":"Frábær þjónusta","author":"Anna"},{"quote":"","author":"nobody"}],
			"missingFields":["everything"]
		}`,
	}}
	fields, err := NewAnalyzer(completer).ExtractFields(context.Background(), sampleContent(), Classification{Vertical: "salon"})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if fields.BusinessName != "Klippt" {
		t.Fatalf("business name not trimmed: %q", fields.BusinessName)
	}
	if len(fields.Services) != 1 || fields.Services[0].Name != "Klipping" || fields.Services[0].Price != "6.900 kr." {
		t.Fatalf("services not cleaned/verbatim price: %+v", fields.Services)
	}
	if len(fields.Hours) != 2 {
		t.Fatalf("expected 2 valid hour rows (invalid day dropped), got %+v", fields.Hours)
	}
	if fields.Hours[0].Day != "monday" || fields.Hours[0].Opens != "09:00" || fields.Hours[0].Closes != "17:30" {
		t.Fatalf("weekday/clock not canonicalized: %+v", fields.Hours[0])
	}
	if fields.Hours[1].Day != "sunday" || !fields.Hours[1].Closed {
		t.Fatalf("closed day not preserved: %+v", fields.Hours[1])
	}
	if fields.Contact.Email != "" || fields.Contact.Phone != "555-1234" {
		t.Fatalf("contact not cleaned: %+v", fields.Contact)
	}
	if len(fields.Testimonials) != 1 || fields.Testimonials[0].Quote != "Frábær þjónusta" {
		t.Fatalf("testimonials not cleaned: %+v", fields.Testimonials)
	}
	// MissingFields is recomputed from the cleaned result, not the model's claim.
	if contains(joinList(fields.MissingFields), "everything") {
		t.Fatalf("missing fields should be recomputed, got %v", fields.MissingFields)
	}
	if !contains(joinList(fields.MissingFields), "tagline") {
		t.Fatalf("empty tagline should be flagged missing, got %v", fields.MissingFields)
	}
}

func TestRewriteMergesProseAndPreservesVerbatim(t *testing.T) {
	completer := &fakeCompleter{responses: map[string]string{
		"respin_rewrite": `{"tagline":"Fersk klipping í miðbænum","about":"Ný og betri lýsing.","services":[{"name":"Klipping","description":"Snyrtileg klipping fyrir alla."}]}`,
	}}
	in := ExtractedFields{
		BusinessName: "Klippt",
		About:        "gamall texti",
		Services:     []ExtractService{{Name: "Klipping", Description: "old", Price: "6.900 kr."}},
		Contact:      ContactDetails{Phone: "555-1234"},
		Testimonials: []Testimonial{{Quote: "Frábær", Author: "Anna"}},
	}
	out, err := NewAnalyzer(completer).RewriteCopy(context.Background(), in, "is")
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if out.About != "Ný og betri lýsing." || out.Tagline != "Fersk klipping í miðbænum" {
		t.Fatalf("prose not rewritten: %+v", out)
	}
	if out.Services[0].Description != "Snyrtileg klipping fyrir alla." {
		t.Fatalf("service description not rewritten: %+v", out.Services[0])
	}
	// Verbatim facts must be preserved untouched.
	if out.Services[0].Price != "6.900 kr." || out.Contact.Phone != "555-1234" || out.BusinessName != "Klippt" {
		t.Fatalf("verbatim facts changed: %+v", out)
	}
	if out.Testimonials[0].Quote != "Frábær" {
		t.Fatalf("testimonial changed: %+v", out.Testimonials[0])
	}
	// The Icelandic register guidance should be appended to the system prompt.
	if len(completer.requests) != 1 || !contains(completer.requests[0].System, "íslenska") {
		t.Fatalf("icelandic guidance missing from rewrite prompt")
	}
}

func TestRewriteSkippedWithoutProse(t *testing.T) {
	completer := &fakeCompleter{responses: map[string]string{}}
	in := ExtractedFields{Contact: ContactDetails{Phone: "555"}}
	out, err := NewAnalyzer(completer).RewriteCopy(context.Background(), in, "is")
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if len(completer.requests) != 0 {
		t.Fatal("rewrite should be skipped (no budget spent) when there is no prose")
	}
	if out.Contact.Phone != "555" {
		t.Fatalf("fields should pass through unchanged: %+v", out)
	}
}

func TestAnalyzeChainsStagesAndResolvesLocale(t *testing.T) {
	completer := &fakeCompleter{responses: map[string]string{
		"respin_classification": `{"vertical":"salon","services":["klipping"],"locale":"is","tone":"warm","confidence":0.8}`,
		"respin_extraction":     `{"businessName":"Klippt","tagline":"","about":"Texti","services":[{"name":"Klipping","description":"gott","price":""}],"hours":[],"contact":{"phone":"","email":"","address":""},"testimonials":[],"missingFields":[]}`,
		"respin_rewrite":        `{"tagline":"","about":"Betri texti","services":[{"name":"Klipping","description":"betra"}]}`,
	}}
	result, err := NewAnalyzer(completer).Analyze(context.Background(), sampleContent(), "")
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.TargetLocale != "is" {
		t.Fatalf("target locale should resolve to is, got %q", result.TargetLocale)
	}
	if result.Fields.About != "Betri texti" {
		t.Fatalf("rewrite not applied in chain: %+v", result.Fields)
	}
	if result.Degraded {
		t.Fatal("high-confidence classification should not degrade")
	}
	if len(completer.requests) != 3 {
		t.Fatalf("expected 3 stage calls, got %d", len(completer.requests))
	}
}

func TestAnalyzeLanguageOverride(t *testing.T) {
	completer := &fakeCompleter{responses: map[string]string{
		"respin_classification": `{"vertical":"salon","services":[],"locale":"is","tone":"warm","confidence":0.8}`,
		"respin_extraction":     `{"businessName":"Klippt","tagline":"","about":"Texti","services":[],"hours":[],"contact":{"phone":"","email":"","address":""},"testimonials":[],"missingFields":[]}`,
		"respin_rewrite":        `{"tagline":"","about":"Better text","services":[]}`,
	}}
	result, err := NewAnalyzer(completer).Analyze(context.Background(), sampleContent(), "en-US")
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.TargetLocale != "en" {
		t.Fatalf("override should win, got %q", result.TargetLocale)
	}
}

func TestAnalyzeMarksDegradedOnLowConfidence(t *testing.T) {
	completer := &fakeCompleter{responses: map[string]string{
		"respin_classification": `{"vertical":"","services":[],"locale":"is","tone":"","confidence":0.1}`,
		"respin_extraction":     `{"businessName":"Klippt","tagline":"","about":"Texti","services":[],"hours":[],"contact":{"phone":"","email":"","address":""},"testimonials":[],"missingFields":[]}`,
		"respin_rewrite":        `{"tagline":"","about":"Betri","services":[]}`,
	}}
	result, err := NewAnalyzer(completer).Analyze(context.Background(), sampleContent(), "")
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if !result.Degraded || result.DegradationReason == "" {
		t.Fatalf("low confidence should mark degraded, got %+v", result)
	}
}

func TestStagesEnforceBudget(t *testing.T) {
	store := newMemBudgetStore()
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	budget := NewBudget(store, 100).WithClock(fixedClock(now))
	// Pre-spend the whole budget so the next stage is blocked.
	if err := budget.Record(context.Background(), 100); err != nil {
		t.Fatalf("record: %v", err)
	}

	completer := &fakeCompleter{responses: map[string]string{
		"respin_classification": `{"vertical":"salon","services":[],"locale":"is","tone":"warm","confidence":0.8}`,
	}}
	analyzer := NewAnalyzer(completer, WithBudget(budget))

	if _, err := analyzer.Classify(context.Background(), sampleContent()); !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("expected ErrBudgetExhausted, got %v", err)
	}
	if len(completer.requests) != 0 {
		t.Fatal("no LLM call should be made once the budget is exhausted")
	}
}

func TestStagesRecordBudgetSpend(t *testing.T) {
	store := newMemBudgetStore()
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	budget := NewBudget(store, 10_000).WithClock(fixedClock(now))

	completer := &fakeCompleter{
		responses: map[string]string{"respin_classification": `{"vertical":"salon","services":[],"locale":"is","tone":"warm","confidence":0.8}`},
		tokens:    250,
	}
	analyzer := NewAnalyzer(completer, WithBudget(budget))
	if _, err := analyzer.Classify(context.Background(), sampleContent()); err != nil {
		t.Fatalf("classify: %v", err)
	}
	if got, _ := store.DailyTokens(context.Background(), now); got != 250 {
		t.Fatalf("expected 250 tokens recorded, got %d", got)
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

func joinList(values []string) string {
	return strings.Join(values, "|")
}
