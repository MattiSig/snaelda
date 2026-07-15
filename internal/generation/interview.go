package generation

import (
	"context"
	"errors"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// ClarifyingQuestionKind enumerates how the intake form should render a question.
const (
	ClarifyingQuestionKindSingle = "single"
	ClarifyingQuestionKindMulti  = "multi"
	ClarifyingQuestionKindText   = "text"
)

// ErrClarifyingPlannerUnavailable signals that the interview planner is not
// configured. Callers should proceed without an interview (the model can run
// with assumptions, the previous behaviour).
var ErrClarifyingPlannerUnavailable = errors.New("clarifying question planner is not configured")

// MaxClarifyingQuestions caps the size of the intake form. We err on the side
// of tightness — three questions max so users do not feel interrogated.
const MaxClarifyingQuestions = 3

// ClarifyingQuestionPlanner produces 0-3 short, context-aware questions that
// the model believes would meaningfully reshape the generated site. The
// planner is permitted to return zero questions when the prompt is already
// detailed enough.
type ClarifyingQuestionPlanner interface {
	BuildClarifyingQuestions(ctx context.Context, request ClarifyingQuestionsRequest) ([]ClarifyingQuestion, error)
}

// ClarifyingQuestionsRequest carries the same minimal context the outline
// planner will receive — just enough for the model to decide what's missing.
type ClarifyingQuestionsRequest struct {
	Prompt            string                 `json:"prompt"`
	NameHint          string                 `json:"nameHint,omitempty"`
	PreferredLanguage string                 `json:"preferredLanguage,omitempty"`
	Brand             siteconfig.BrandConfig `json:"brand,omitempty"`
	OptionalHints     map[string]string      `json:"optionalHints,omitempty"`
}

// ClarifyingQuestion is a single intake-form question. ID is opaque to the
// model — the backend echoes it so the frontend can match answers back. Kind
// drives the input control: single-select, multi-select, or short free-text.
// Options are model-suggested chips; the frontend always offers a "skip" too.
type ClarifyingQuestion struct {
	ID      string   `json:"id"`
	Prompt  string   `json:"prompt"`
	Kind    string   `json:"kind"`
	Options []string `json:"options,omitempty"`
	Helper  string   `json:"helper,omitempty"`
}

// ClarifyingAnswer is the user's response to a single ClarifyingQuestion.
// SelectedOptions is used for single/multi kinds; Text is used for text kind.
// Skipped indicates the user chose to skip without answering.
type ClarifyingAnswer struct {
	QuestionID      string   `json:"questionId"`
	Prompt          string   `json:"prompt,omitempty"`
	SelectedOptions []string `json:"selectedOptions,omitempty"`
	Text            string   `json:"text,omitempty"`
	Skipped         bool     `json:"skipped,omitempty"`
}

// MaxSeedCollectionSuggestions caps step two of the intake flow. Like the
// clarifying questions, tightness is deliberate — one or two lists the user
// can rattle off, not a CMS setup wizard.
const MaxSeedCollectionSuggestions = 2

// SeedCollectionPlanner powers the collections step of the intake flow: it
// proposes which collections a fresh spin would benefit from, and later turns
// the user's raw item lines into a structured schema plus entries. Both calls
// are best-effort — when either fails, generation proceeds without the
// collection (mirroring re-spin's drop-on-failure rule).
type SeedCollectionPlanner interface {
	SuggestSeedCollections(ctx context.Context, request SeedCollectionSuggestRequest) ([]SeedCollectionSuggestion, error)
	DraftSeedCollection(ctx context.Context, request SeedCollectionDraftRequest) (SeedCollectionDraftResponse, error)
}

// SeedCollectionSuggestRequest carries the same minimal context as the
// clarifying-questions call so both intake calls can run in parallel.
type SeedCollectionSuggestRequest struct {
	Prompt            string            `json:"prompt"`
	NameHint          string            `json:"nameHint,omitempty"`
	PreferredLanguage string            `json:"preferredLanguage,omitempty"`
	OptionalHints     map[string]string `json:"optionalHints,omitempty"`
}

// SeedCollectionSuggestion is one candidate collection shown in step two of
// the intake form. Labels are in the site's language; ItemHint tells the user
// what to write per line and Example is one realistic line they can mimic.
type SeedCollectionSuggestion struct {
	ID            string `json:"id"`
	SingularLabel string `json:"singularLabel"`
	PluralLabel   string `json:"pluralLabel"`
	Helper        string `json:"helper,omitempty"`
	ItemHint      string `json:"itemHint,omitempty"`
	Example       string `json:"example,omitempty"`
}

// SeedCollectionInput is the user's confirmation of one suggestion: the
// echoed labels plus their raw items, one per line. The backend structures
// the lines into entries at generation time.
type SeedCollectionInput struct {
	SuggestionID  string `json:"suggestionId,omitempty"`
	SingularLabel string `json:"singularLabel"`
	PluralLabel   string `json:"pluralLabel"`
	ItemsText     string `json:"itemsText"`
}

// SeedCollectionDraftRequest asks the model to structure the user's raw item
// lines into a field schema plus one entry per line.
type SeedCollectionDraftRequest struct {
	SitePrompt        string `json:"sitePrompt"`
	SiteName          string `json:"siteName,omitempty"`
	PreferredLanguage string `json:"preferredLanguage,omitempty"`
	SingularLabel     string `json:"singularLabel"`
	PluralLabel       string `json:"pluralLabel"`
	ItemsText         string `json:"itemsText"`
}

// SeedCollectionDraftResponse is the structured shape returned by the model.
// The finisher (seed_collections.go) mints ids/slugs, filters fields to the
// schema, and validates before the collection is allowed into generation.
type SeedCollectionDraftResponse struct {
	Schema  []siteconfig.FieldDefinition `json:"schema"`
	Entries []SeedEntryDraft             `json:"entries"`
}

// SeedEntryDraft is one structured entry: the display title (drives the slug
// and SEO title) plus field values keyed by schema keys.
type SeedEntryDraft struct {
	Title  string         `json:"title"`
	Fields map[string]any `json:"fields"`
}
