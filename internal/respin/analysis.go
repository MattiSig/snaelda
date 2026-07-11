package respin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// maxSourceChars caps how much cleaned source text is fed to a stage prompt. The
// fetcher already bounds each document to 5 MB, but a handful of long pages can
// still produce a large brief; this keeps the LLM payload (and its token cost)
// predictable. It is generous enough to carry a small-business site's real copy.
const maxSourceChars = 24000

// SourcePage is one fetched-and-cleaned page fed to the LLM stages.
type SourcePage struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
	Text  string `json:"text"`
}

// SourceContent is the readable substrate the LLM stages classify and extract
// from: the source URL, the head-level title/description/language signals, and
// the cleaned page text discovered under the SSRF guards. It is deliberately
// decoupled from the raw Page shape so the stages depend on a small, stable
// input the composer can assemble however it likes.
type SourceContent struct {
	SourceURL    string       `json:"sourceUrl,omitempty"`
	Title        string       `json:"title,omitempty"`
	Description  string       `json:"description,omitempty"`
	DetectedLang string       `json:"detectedLang,omitempty"`
	Pages        []SourcePage `json:"pages"`
}

// BuildSourceContent assembles the stage input from fetched pages. The first
// page seeds the source URL, title, description, and detected language; every
// non-empty page contributes its cleaned text.
func BuildSourceContent(pages ...Page) SourceContent {
	content := SourceContent{Pages: []SourcePage{}}
	for i, page := range pages {
		text := strings.TrimSpace(page.Text)
		if i == 0 {
			content.SourceURL = firstNonEmptyString(page.FinalURL, page.URL)
			content.Title = strings.TrimSpace(page.Title)
			content.Description = strings.TrimSpace(page.Description)
			content.DetectedLang = normalizeStageLocale(page.Meta.Lang)
		}
		if text == "" {
			continue
		}
		content.Pages = append(content.Pages, SourcePage{
			URL:   firstNonEmptyString(page.FinalURL, page.URL),
			Title: strings.TrimSpace(page.Title),
			Text:  text,
		})
	}
	return content
}

// HasText reports whether any page carries readable text to work from.
func (c SourceContent) HasText() bool {
	for _, page := range c.Pages {
		if strings.TrimSpace(page.Text) != "" {
			return true
		}
	}
	return false
}

// promptDocument renders the source content into a compact, bounded text block
// for a stage prompt: title/description/URL header, then each page's title and
// cleaned text, truncated at maxSourceChars.
func (c SourceContent) promptDocument() string {
	var b strings.Builder
	if c.Title != "" {
		fmt.Fprintf(&b, "PAGE TITLE: %s\n", c.Title)
	}
	if c.Description != "" {
		fmt.Fprintf(&b, "META DESCRIPTION: %s\n", c.Description)
	}
	if c.SourceURL != "" {
		fmt.Fprintf(&b, "SOURCE URL: %s\n", c.SourceURL)
	}
	if c.DetectedLang != "" {
		fmt.Fprintf(&b, "DETECTED LANGUAGE: %s\n", c.DetectedLang)
	}
	for _, page := range c.Pages {
		b.WriteString("\n---\n")
		if page.Title != "" {
			fmt.Fprintf(&b, "# %s\n", page.Title)
		}
		b.WriteString(strings.TrimSpace(page.Text))
		b.WriteString("\n")
	}
	doc := strings.TrimSpace(b.String())
	if len(doc) > maxSourceChars {
		doc = doc[:maxSourceChars]
	}
	return doc
}

// Analyzer runs re-spin's LLM stages — classification, structured field
// extraction, and copy rewrite (Spec 21 pipeline steps 5, 7, 8) — over a shared
// completer and an optional daily budget. Construct one per request context; it
// holds no per-call state.
type Analyzer struct {
	completer Completer
	budget    *Budget
	logger    *slog.Logger
}

// AnalyzerOption customizes an Analyzer.
type AnalyzerOption func(*Analyzer)

// WithBudget enforces the daily LLM budget on every stage (the public demo
// tier). Omit it for session-bound re-spins, which are quota-accounted
// elsewhere.
func WithBudget(budget *Budget) AnalyzerOption {
	return func(a *Analyzer) { a.budget = budget }
}

// WithLogger attaches a logger for best-effort budget-recording diagnostics.
func WithLogger(logger *slog.Logger) AnalyzerOption {
	return func(a *Analyzer) {
		if logger != nil {
			a.logger = logger
		}
	}
}

// NewAnalyzer builds the LLM-stage runner. A nil completer is allowed and makes
// every stage return ErrCompleterUnavailable so the pipeline degrades to the
// prompt flow.
func NewAnalyzer(completer Completer, opts ...AnalyzerOption) *Analyzer {
	a := &Analyzer{completer: completer, logger: slog.Default()}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// runStage enforces the budget, runs one structured completion, records spend,
// and decodes the result into out.
func (a *Analyzer) runStage(ctx context.Context, req CompletionRequest, out any) error {
	if a == nil || a.completer == nil {
		return ErrCompleterUnavailable
	}
	if err := a.budget.Check(ctx); err != nil {
		return err
	}
	result, err := a.completer.Complete(ctx, req)
	if err != nil {
		return err
	}
	if err := a.budget.Record(ctx, result.TotalTokens); err != nil {
		// Spend already happened; log and continue rather than fail the stage.
		a.logger.Warn("record respin llm spend failed", "stage", req.Name, "error", err)
	}
	if err := json.Unmarshal([]byte(result.Content), out); err != nil {
		return fmt.Errorf("decode %s result: %w", req.Name, err)
	}
	return nil
}

// AnalysisResult is the combined output of the LLM stages: the business
// classification, the (rewritten) structured fields, the resolved target
// language, and whether the pipeline should mark the import degraded (low
// classification confidence). The composer turns this into the canonical
// generation input.
type AnalysisResult struct {
	Classification    Classification  `json:"classification"`
	Fields            ExtractedFields `json:"fields"`
	TargetLocale      string          `json:"targetLocale"`
	Degraded          bool            `json:"degraded"`
	DegradationReason string          `json:"degradationReason,omitempty"`
}

// Analyze runs the three LLM stages in order — classify, extract, rewrite —
// resolving the target language from the classification (or the caller's
// override) and rewriting the extracted copy into it. A low-confidence
// classification is not an error: the result is returned with Degraded set so
// the caller falls back to the generic block set per Spec 21.
//
// languageOverride, when non-empty, fixes the target language (the demo lets a
// visitor override before claiming); otherwise the detected content locale is
// used, defaulting to Icelandic for the Iceland-first phase.
func (a *Analyzer) Analyze(ctx context.Context, content SourceContent, languageOverride string) (AnalysisResult, error) {
	classification, err := a.Classify(ctx, content)
	if err != nil {
		return AnalysisResult{}, err
	}

	fields, err := a.ExtractFields(ctx, content, classification)
	if err != nil {
		return AnalysisResult{}, err
	}

	target := normalizeStageLocale(languageOverride)
	if target == "" {
		target = firstNonEmptyString(classification.Locale, content.DetectedLang, "is")
	}

	fields, err = a.RewriteCopy(ctx, fields, target)
	if err != nil {
		return AnalysisResult{}, err
	}

	result := AnalysisResult{
		Classification: classification,
		Fields:         fields,
		TargetLocale:   target,
	}
	if classification.LowConfidence() {
		result.Degraded = true
		result.DegradationReason = "low classification confidence; using generic block set"
	}
	return result, nil
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// normalizeStageLocale reduces a BCP-47-ish tag to its lowercase primary subtag
// ("is-IS" -> "is"), matching the backend locale allow-list convention.
func normalizeStageLocale(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if idx := strings.IndexAny(trimmed, "-_"); idx > 0 {
		trimmed = trimmed[:idx]
	}
	return strings.ToLower(trimmed)
}
