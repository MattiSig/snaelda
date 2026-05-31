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
	Prompt        string                 `json:"prompt"`
	NameHint      string                 `json:"nameHint,omitempty"`
	Brand         siteconfig.BrandConfig `json:"brand,omitempty"`
	OptionalHints map[string]string      `json:"optionalHints,omitempty"`
}

// ClarifyingQuestion is a single intake-form question. ID is opaque to the
// model — the backend echoes it so the frontend can match answers back. Kind
// drives the input control: single-select, multi-select, or short free-text.
// Options are model-suggested chips; the frontend always offers a "skip" too.
type ClarifyingQuestion struct {
	ID       string   `json:"id"`
	Prompt   string   `json:"prompt"`
	Kind     string   `json:"kind"`
	Options  []string `json:"options,omitempty"`
	Helper   string   `json:"helper,omitempty"`
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
