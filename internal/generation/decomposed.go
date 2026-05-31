package generation

import (
	"context"
	"errors"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// ErrDecomposedPlannerUnavailable signals that the decomposed pipeline is not
// wired. Callers should fall back to the legacy BuildPlan path.
var ErrDecomposedPlannerUnavailable = errors.New("decomposed planner is not configured")

// DecomposedPlanner runs the two-step generation pipeline: outline (site
// structure + theme + pages) and per-page content (one call per page produces
// the ordered block list with full props for that page). Each method maps to
// one structured LLM call so the orchestrator can stream partial results and
// parallelise per-page work.
//
// The same OpenAIPlanner implements this interface alongside BuildPlan, so
// switching execution paths is a wiring decision, not a code change in the
// planner.
type DecomposedPlanner interface {
	BuildOutline(ctx context.Context, request OutlineRequest) (OutlineResult, error)
	BuildPageContent(ctx context.Context, request PageContentRequest) (PageContentResult, error)
}

// OutlineRequest carries the user's prompt and any interview answers so the
// model can pick a tight page list + theme without seeing the full block
// registry.
type OutlineRequest struct {
	Prompt            string                 `json:"prompt"`
	NameHint          string                 `json:"nameHint,omitempty"`
	PreferredLanguage string                 `json:"preferredLanguage,omitempty"`
	OptionalHints     map[string]string      `json:"optionalHints,omitempty"`
	Brand             siteconfig.BrandConfig `json:"brand,omitempty"`
	InterviewAnswers  []ClarifyingAnswer     `json:"interviewAnswers,omitempty"`
	CurrentOutline    *OutlineResult         `json:"currentOutline,omitempty"`
}

// OutlineResult is the outline-stage output: structure only, no block copy.
type OutlineResult struct {
	SiteName       string                    `json:"siteName"`
	SiteGoal       string                    `json:"siteGoal"`
	ThemeSelection siteconfig.ThemeSelection `json:"themeSelection"`
	Pages          []OutlinePage             `json:"pages"`
	Assumptions    []string                  `json:"assumptions,omitempty"`
}

// OutlinePage is a single page in the outline. Slug is the absolute path
// (e.g. "/" or "/about"); Goal is a one-sentence intent the per-page planner
// will lean on.
type OutlinePage struct {
	Title string               `json:"title"`
	Slug  string               `json:"slug"`
	Goal  string               `json:"goal"`
	SEO   siteconfig.SEOConfig `json:"seo"`
}

// PageContentRequest tells the per-page composer everything it needs to pick
// an ordered block list AND produce each block's full props in a single call.
// The block catalog (taglines, theme catalog, brand direction) lives in the
// cached system prefix; this request only carries the per-call deltas.
type PageContentRequest struct {
	SiteName         string                 `json:"siteName"`
	SiteGoal         string                 `json:"siteGoal,omitempty"`
	Brand            siteconfig.BrandConfig `json:"brand,omitempty"`
	Page             OutlinePage            `json:"page"`
	Outline          []OutlinePage          `json:"outline"`
	AllowedTypes     []string               `json:"allowedTypes"`
	InterviewAnswers []ClarifyingAnswer     `json:"interviewAnswers,omitempty"`
}

// PageContentResult is the model's chosen ordered block list for one page,
// each carrying the full props object (no copy/structure split).
type PageContentResult struct {
	Blocks []PageContentBlock `json:"blocks"`
}

// PageContentBlock is one entry in the page composer's output. Props matches
// the per-type PropSchema of the chosen block.
type PageContentBlock struct {
	Type  string         `json:"type"`
	Props map[string]any `json:"props"`
}
