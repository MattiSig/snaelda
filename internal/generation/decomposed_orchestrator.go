package generation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"golang.org/x/sync/errgroup"
)

// maxParallelPageContent caps the number of concurrent per-page composer
// calls. A 5-page site finishes in roughly one call-duration.
const maxParallelPageContent = 4

// maxPageContentAttempts bounds the per-page content composer to one retry:
// the first attempt plus a single feedback-guided correction before the page
// (and only that page) is treated as failed.
const maxPageContentAttempts = 2

// pageContentRetryFeedback turns a content-composer failure into a short
// directive the retry attempt sees in its payload's "feedback" field.
func pageContentRetryFeedback(err error) string {
	if err == nil {
		return ""
	}
	return "The previous attempt failed to produce valid page content: " +
		strings.TrimSpace(err.Error()) +
		". Return every block for the supplied layout in order, with props that satisfy each block's schema exactly."
}

// generateDraftDecomposed runs the three-step pipeline:
//  1. BuildOutline (1 small call)         → structure + theme + pages
//  2. BuildPageLayout (1 call per page)   → ordered block skeleton
//  3. BuildPageContent (1 call per page)  → full props for that layout
//
// Returns ErrDecomposedPlannerUnavailable when the decomposed planner is not
// configured; callers should fall back to generateDraftWithRetry on that error.
//
// Partial events stream as outline → one per page-content arrival.
func (s *Service) generateDraftDecomposed(
	ctx context.Context,
	workspaceID string,
	input generationInputContext,
	tracker *progressTracker,
	partialEmitter partialEventEmitter,
) (generationPlan, siteconfig.SiteDraft, error) {
	if s.decomposedPlanner == nil {
		return generationPlan{}, siteconfig.SiteDraft{}, ErrDecomposedPlannerUnavailable
	}

	if err := emitTrackerStep(ctx, tracker, "plan.pages"); err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, err
	}
	outline, err := s.decomposedPlanner.BuildOutline(ctx, OutlineRequest{
		Prompt:            input.Prompt,
		NameHint:          input.NameHint,
		PreferredLanguage: input.PreferredLanguage,
		OptionalHints:     input.OptionalHints,
		Brand:             input.Brand,
		InterviewAnswers:  input.InterviewAnswers,
	})
	if err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, fmt.Errorf("build outline: %w", err)
	}
	if len(outline.Pages) == 0 {
		return generationPlan{}, siteconfig.SiteDraft{}, errors.New("outline returned no pages")
	}
	partialEmitter.emitOutline(ctx, outline)
	if err := emitTrackerStep(ctx, tracker, "plan.theme"); err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, err
	}

	if err := emitTrackerStep(ctx, tracker, "plan.blocks"); err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, err
	}
	if err := emitTrackerStep(ctx, tracker, "copy.write"); err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, err
	}
	pagePlans := make([]generationPagePlan, len(outline.Pages))
	pageGroup, pageCtx := errgroup.WithContext(ctx)
	pageGroup.SetLimit(maxParallelPageContent)
	for i, page := range outline.Pages {
		i, page := i, page
		pageGroup.Go(func() error {
			pagePlan, err := s.buildPagePlanFromLayout(pageCtx, outline.SiteName, outline.SiteGoal, input.Prompt, input.PreferredLanguage, input.Brand, page, outline.Pages, input.InterviewAnswers)
			if err != nil {
				return fmt.Errorf("compose page %s: %w", page.Slug, err)
			}
			pagePlans[i] = pagePlan
			partialEmitter.emitPageContent(pageCtx, page.Slug, pagePlans[i])
			return nil
		})
	}
	if err := pageGroup.Wait(); err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, err
	}

	if err := emitTrackerStep(ctx, tracker, "validate.repair"); err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, err
	}

	plan := generationPlan{
		SiteName:       outline.SiteName,
		SiteGoal:       outline.SiteGoal,
		ThemePreset:    outline.ThemeSelection.Palette,
		ThemeSelection: outline.ThemeSelection,
		Theme:          siteconfig.BuildTheme(outline.ThemeSelection),
		Pages:          pagePlans,
		Assumptions:    outline.Assumptions,
	}
	plan = repairGenerationPlan(plan, input.PreferredLanguage)

	// English must never leak into an Icelandic draft (Spec 22). The decomposed
	// pipeline has no retry loop of its own, so surface any leak as a
	// ValidationError: the caller falls back to generateDraftWithRetry, which
	// re-runs the plan with these issues threaded into the planner feedback.
	if issues := detectLanguageConformanceIssues(plan, input.PreferredLanguage, verbatimExemptionFromInput(input)); len(issues) > 0 {
		return generationPlan{}, siteconfig.SiteDraft{}, siteconfig.ValidationError{Issues: issues}
	}

	slugValue, err := s.createSlug(ctx, workspaceID, input.SlugHint, plan.SiteName)
	if err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, err
	}
	draft, err := buildDraftFromPlan(plan, slugValue, input.PreferredLanguage, input.Brand, input.PreallocatedSiteID)
	if err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, err
	}
	if err := s.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, err
	}
	return plan, draft, nil
}

// repromptSiteDecomposed runs the site reprompt through the decomposed
// pipeline: ask the outline planner to produce a new outline given the
// current outline + reprompt directive, then for each page in the new outline
// either (a) revise the matching draft page from its current blocks when the
// slug survives, or (b) run the per-page layout + content calls for new or
// structurally replaced pages.
//
// Returns ErrDecomposedPlannerUnavailable when the planner is not configured.
func (s *Service) repromptSiteDecomposed(
	ctx context.Context,
	workspaceID string,
	currentDraft siteconfig.SiteDraft,
	prompt string,
	tracker *progressTracker,
	partialEmitter partialEventEmitter,
) (generationPlan, siteconfig.SiteDraft, error) {
	if s.decomposedPlanner == nil {
		return generationPlan{}, siteconfig.SiteDraft{}, ErrDecomposedPlannerUnavailable
	}

	currentOutline := summarizeDraftAsOutline(currentDraft)
	if err := emitTrackerStep(ctx, tracker, "plan.pages"); err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, err
	}
	outline, err := s.decomposedPlanner.BuildOutline(ctx, OutlineRequest{
		Prompt:            prompt,
		NameHint:          currentDraft.Site.Name,
		PreferredLanguage: currentDraft.Site.DefaultLocale,
		Brand:             currentDraft.Brand,
		CurrentOutline:    &currentOutline,
	})
	if err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, fmt.Errorf("reprompt outline: %w", err)
	}
	if len(outline.Pages) == 0 {
		return generationPlan{}, siteconfig.SiteDraft{}, errors.New("reprompt outline returned no pages")
	}
	partialEmitter.emitOutline(ctx, outline)
	if err := emitTrackerStep(ctx, tracker, "plan.theme"); err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, err
	}

	if err := emitTrackerStep(ctx, tracker, "plan.blocks"); err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, err
	}
	if err := emitTrackerStep(ctx, tracker, "copy.write"); err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, err
	}

	existingPagesBySlug := make(map[string]siteconfig.PageDraft, len(currentDraft.Pages))
	for _, page := range currentDraft.Pages {
		existingPagesBySlug[page.Slug] = page
	}

	pagePlans := make([]generationPagePlan, len(outline.Pages))
	revisedPages := make([]siteconfig.PageDraft, len(outline.Pages))

	pageGroup, pageCtx := errgroup.WithContext(ctx)
	pageGroup.SetLimit(maxParallelPageContent)
	for i, page := range outline.Pages {
		i, page := i, page
		pageGroup.Go(func() error {
			if existing, ok := existingPagesBySlug[page.Slug]; ok {
				pagePrompt := repromptDirectiveForPage(prompt, page)
				result, err := s.repromptPage(pageCtx, currentDraft, existing, pagePrompt)
				if err != nil {
					return fmt.Errorf("reprompt existing page %s: %w", page.Slug, err)
				}
				result.Page.Title = firstNonEmpty(page.Title, existing.Title)
				result.Page.SEO = page.SEO
				result.Page.Slug = page.Slug
				result.Plan.Title = result.Page.Title
				result.Plan.SEO = result.Page.SEO
				result.Plan.Slug = result.Page.Slug
				result.Plan.Goal = page.Goal
				revisedPages[i] = result.Page
				pagePlans[i] = result.Plan
				partialEmitter.emitPageContent(pageCtx, page.Slug, pagePlans[i])
				return nil
			}

			pagePlan, err := s.buildPagePlanFromLayout(pageCtx, outline.SiteName, outline.SiteGoal, prompt, currentDraft.Site.DefaultLocale, currentDraft.Brand, page, outline.Pages, nil)
			if err != nil {
				return fmt.Errorf("reprompt compose page %s: %w", page.Slug, err)
			}
			pagePlans[i] = pagePlan
			partialEmitter.emitPageContent(pageCtx, page.Slug, pagePlans[i])
			return nil
		})
	}
	if err := pageGroup.Wait(); err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, err
	}

	if err := emitTrackerStep(ctx, tracker, "validate.repair"); err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, err
	}

	plan := generationPlan{
		SiteName:       outline.SiteName,
		SiteGoal:       outline.SiteGoal,
		ThemePreset:    outline.ThemeSelection.Palette,
		ThemeSelection: outline.ThemeSelection,
		Theme:          siteconfig.BuildTheme(outline.ThemeSelection),
		Pages:          pagePlans,
		Assumptions:    outline.Assumptions,
	}
	plan = repairGenerationPlan(plan, currentDraft.Site.DefaultLocale)

	// Reprompt keeps the existing site; buildDraftFromPlan mints a throwaway id
	// that applySiteIdentity (in the caller) replaces with currentDraft's id.
	draft, err := buildDraftFromPlan(plan, currentDraft.Site.Slug, "", currentDraft.Brand, "")
	if err != nil {
		return generationPlan{}, siteconfig.SiteDraft{}, err
	}
	for pageIndex, page := range revisedPages {
		if pageIndex >= len(draft.Pages) {
			continue
		}
		if page.ID == "" {
			continue
		}
		draft.Pages[pageIndex] = page
	}
	draft.Navigation = syncRepromptNavigation(currentDraft.Navigation, draft.Pages)
	return plan, draft, nil
}

func repromptDirectiveForPage(sitePrompt string, page OutlinePage) string {
	directive := strings.TrimSpace(sitePrompt)
	if directive == "" {
		directive = "Refresh this page to match the latest site direction."
	}

	title := firstNonEmpty(strings.TrimSpace(page.Title), "this page")
	if goal := strings.TrimSpace(page.Goal); goal != "" {
		return fmt.Sprintf("Apply this site-wide direction to %q while preserving unaffected sections.\nDirection: %s\nPage goal: %s", title, directive, goal)
	}
	return fmt.Sprintf("Apply this site-wide direction to %q while preserving unaffected sections.\nDirection: %s", title, directive)
}

func (s *Service) buildPagePlanFromLayout(
	ctx context.Context,
	siteName string,
	siteGoal string,
	prompt string,
	preferredLanguage string,
	brand siteconfig.BrandConfig,
	page OutlinePage,
	outline []OutlinePage,
	interviewAnswers []ClarifyingAnswer,
) (generationPagePlan, error) {
	layout, err := s.decomposedPlanner.BuildPageLayout(ctx, PageLayoutRequest{
		SiteName:          siteName,
		SiteGoal:          siteGoal,
		Prompt:            prompt,
		PreferredLanguage: preferredLanguage,
		Brand:             brand,
		Page:              page,
		Outline:           outline,
		InterviewAnswers:  interviewAnswers,
	})
	if err != nil {
		return generationPagePlan{}, fmt.Errorf("layout page: %w", err)
	}
	if len(layout.Blocks) == 0 {
		return generationPagePlan{}, errors.New("layout returned no blocks")
	}

	contentRequest := PageContentRequest{
		SiteName:          siteName,
		SiteGoal:          siteGoal,
		Prompt:            prompt,
		PreferredLanguage: preferredLanguage,
		Brand:             brand,
		Page:              page,
		Outline:           outline,
		Layout:            layout.Blocks,
		InterviewAnswers:  interviewAnswers,
	}
	// One retry, feeding the failure back as guidance. The schema now pins each
	// slot to its layout type so drift is structurally impossible, but a prop-
	// validation slip on a single page should be repaired in place rather than
	// abandoning the whole decomposed run to the mega-call fallback.
	var content PageContentResult
	var contentErr error
	for attempt := 0; attempt < maxPageContentAttempts; attempt++ {
		content, contentErr = s.decomposedPlanner.BuildPageContent(ctx, contentRequest)
		if contentErr == nil {
			break
		}
		if ctx.Err() != nil {
			return generationPagePlan{}, fmt.Errorf("write page content: %w", contentErr)
		}
		contentRequest.Feedback = pageContentRetryFeedback(contentErr)
	}
	if contentErr != nil {
		return generationPagePlan{}, fmt.Errorf("write page content: %w", contentErr)
	}

	blocks := make([]generationBlockPlan, 0, len(content.Blocks))
	for index, b := range content.Blocks {
		stripped, _ := stripGeneratedImages(b.Props).(map[string]any)
		if stripped == nil {
			stripped = b.Props
		}
		purpose := ""
		if index < len(layout.Blocks) {
			purpose = layout.Blocks[index].Purpose
		}
		blocks = append(blocks, generationBlockPlan{Type: b.Type, Purpose: purpose, Props: stripped})
	}
	return generationPagePlan{
		Title:  page.Title,
		Slug:   page.Slug,
		Goal:   page.Goal,
		SEO:    page.SEO,
		Blocks: blocks,
	}, nil
}

func summarizeDraftAsOutline(draft siteconfig.SiteDraft) OutlineResult {
	pages := make([]OutlinePage, 0, len(draft.Pages))
	for _, page := range draft.Pages {
		pages = append(pages, OutlinePage{
			Title: page.Title,
			Slug:  page.Slug,
			Goal:  "",
			SEO:   page.SEO,
		})
	}
	return OutlineResult{
		SiteName:       draft.Site.Name,
		SiteGoal:       draft.Site.SEO.Description,
		ThemeSelection: siteconfig.DetectThemeSelection(draft.Theme),
		Pages:          pages,
	}
}

func blocksToPlan(blocks []siteconfig.BlockInstance) []generationBlockPlan {
	out := make([]generationBlockPlan, 0, len(blocks))
	for _, block := range blocks {
		out = append(out, generationBlockPlan{
			Type:  block.Type,
			Props: block.Props,
		})
	}
	return out
}

func emitTrackerStep(ctx context.Context, tracker *progressTracker, step string) error {
	if tracker == nil {
		return nil
	}
	return tracker.emit(ctx, step)
}

// defaultAllowedBlockTypes returns the names of every registered block type.
// The page-content schema constrains the model's output to this set.
func defaultAllowedBlockTypes() []string {
	registry := siteconfig.DefaultBlockRegistry()
	definitions := registry.Definitions()
	out := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		out = append(out, definition.Type)
	}
	return out
}

// stripGeneratedImages walks the props tree of a model-authored block and
// removes any image / photo / avatar objects. The model invents asset IDs
// that don't exist in the workspace; rather than fight that, we drop the
// image entirely and let the existing image-suggest flow add real assets
// after generation. The block validators are written to accept missing image
// fields, so this leaves the draft in a valid shape.
func stripGeneratedImages(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"image", "photo", "avatar"} {
			delete(typed, key)
		}
		for k, v := range typed {
			typed[k] = stripGeneratedImages(v)
		}
		// images is a top-level array on the gallery block; the validator
		// requires at least one entry but each entry's image is optional.
		// Drop each entry's image so the gallery shows placeholders until
		// the user picks photos. This intentionally mirrors the single
		// image / photo / avatar handling above.
		return typed
	case []any:
		for i, item := range typed {
			typed[i] = stripGeneratedImages(item)
		}
		return typed
	default:
		return typed
	}
}

// nilPartialEmitter is the zero-value emitter used when the caller does not
// supply one (e.g. legacy non-streaming path). All methods are no-ops.
type nilPartialEmitter struct{}

func (nilPartialEmitter) emitOutline(context.Context, OutlineResult)                  {}
func (nilPartialEmitter) emitPageContent(context.Context, string, generationPagePlan) {}

// partialEventEmitter is implemented by the SSE handler so the orchestrator
// can stream structural updates as soon as they resolve. The legacy non-
// streaming code path uses nilPartialEmitter.
type partialEventEmitter interface {
	emitOutline(ctx context.Context, outline OutlineResult)
	emitPageContent(ctx context.Context, pageSlug string, page generationPagePlan)
}

// partialEmitterFromSink adapts a ProgressSink into a partialEventEmitter
// when the sink implements PartialEventSink; otherwise returns a no-op.
// Phase 3 wires the real PartialEventSink through the SSE handler.
func partialEmitterFromSink(sink ProgressSink) partialEventEmitter {
	if sink == nil {
		return nilPartialEmitter{}
	}
	if emitter, ok := sink.(partialEventEmitter); ok {
		return emitter
	}
	return nilPartialEmitter{}
}
