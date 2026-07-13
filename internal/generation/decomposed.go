package generation

import (
	"context"
	"errors"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// ErrDecomposedPlannerUnavailable signals that the decomposed pipeline is not
// wired. Callers should fall back to the legacy BuildPlan path.
var ErrDecomposedPlannerUnavailable = errors.New("decomposed planner is not configured")

// DecomposedPlanner runs the three-step generation pipeline: outline (site
// structure + theme + pages), per-page layout (ordered block skeletons), and
// per-page content (full props for the selected layout). Each method maps to
// one structured LLM call so the orchestrator can stream partial results and
// parallelise per-page work.
//
// The same OpenAIPlanner implements this interface alongside BuildPlan, so
// switching execution paths is a wiring decision, not a code change in the
// planner.
type DecomposedPlanner interface {
	BuildOutline(ctx context.Context, request OutlineRequest) (OutlineResult, error)
	BuildPageLayout(ctx context.Context, request PageLayoutRequest) (PageLayoutResult, error)
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

// SourceHero carries a re-spin source site's hero as structured data (Spec 07
// optionalHints.sourceHero, populated by Spec 21). It is only ever attached to
// the home page's layout/content requests, where it steers the hero copy to
// match the source's register and CTA intent — never layout cloning. Every field
// is optional.
type SourceHero struct {
	Headline     string `json:"headline,omitempty"`
	Subheadline  string `json:"subheadline,omitempty"`
	CTALabel     string `json:"ctaLabel,omitempty"`
	ImageAssetID string `json:"imageAssetId,omitempty"`
	// TextOnly is true when the source hero carried no image — a type-led hero the
	// layout stage prefers the statement variant for (Spec 04).
	TextOnly bool `json:"textOnly,omitempty"`
}

// PageLayoutRequest tells the per-page layout planner everything it needs to
// pick an ordered block list without writing full block props.
type PageLayoutRequest struct {
	SiteName          string                 `json:"siteName"`
	SiteGoal          string                 `json:"siteGoal,omitempty"`
	Prompt            string                 `json:"prompt,omitempty"`
	PreferredLanguage string                 `json:"preferredLanguage,omitempty"`
	Brand             siteconfig.BrandConfig `json:"brand,omitempty"`
	Page              OutlinePage            `json:"page"`
	Outline           []OutlinePage          `json:"outline"`
	InterviewAnswers  []ClarifyingAnswer     `json:"interviewAnswers,omitempty"`
	// SourceHero is set only on the home page for a re-spin (Spec 21), so the
	// layout can match the source hero's energy and variant.
	SourceHero *SourceHero `json:"sourceHero,omitempty"`
}

// PageLayoutResult is the model's chosen ordered block skeleton for one page.
type PageLayoutResult struct {
	Blocks []PageLayoutBlock `json:"blocks"`
}

// PageLayoutBlock carries structural intent only. The content pass consumes
// this layout and fills props for each block in the same order.
type PageLayoutBlock struct {
	Type         string `json:"type"`
	Purpose      string `json:"purpose"`
	ContentBrief string `json:"contentBrief"`
	VariantHint  string `json:"variantHint"`
}

// PageContentRequest tells the per-page composer to fill props for the already
// selected ordered layout. It no longer chooses block types or order.
type PageContentRequest struct {
	SiteName          string                 `json:"siteName"`
	SiteGoal          string                 `json:"siteGoal,omitempty"`
	Prompt            string                 `json:"prompt,omitempty"`
	PreferredLanguage string                 `json:"preferredLanguage,omitempty"`
	Brand             siteconfig.BrandConfig `json:"brand,omitempty"`
	Page              OutlinePage            `json:"page"`
	Outline           []OutlinePage          `json:"outline"`
	Layout            []PageLayoutBlock      `json:"layout"`
	InterviewAnswers  []ClarifyingAnswer     `json:"interviewAnswers,omitempty"`
	// SourceHero is set only on the home page for a re-spin (Spec 21), so the hero
	// copy matches the source headline register and CTA intent instead of generic
	// category copy.
	SourceHero *SourceHero `json:"sourceHero,omitempty"`
	// Feedback carries the reason a previous attempt at this page failed so the
	// retry can correct exactly that instead of the whole run collapsing to the
	// mega-call fallback. Empty on the first attempt.
	Feedback string `json:"feedback,omitempty"`
}

// PageContentResult is the selected layout filled with full props.
type PageContentResult struct {
	Blocks []PageContentBlock `json:"blocks"`
}

// PageContentBlock is one entry in the page composer's output. Props matches
// the per-type PropSchema of the chosen block.
type PageContentBlock struct {
	Type  string         `json:"type"`
	Props map[string]any `json:"props"`
}
