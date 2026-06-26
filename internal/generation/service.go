package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/platform/audit"
	"github.com/MattiSig/snaelda/internal/platform/ids"
	"github.com/MattiSig/snaelda/internal/platform/slugs"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrPromptRequired        = errors.New("generation prompt is required")
	ErrPromptTooLong         = errors.New("generation prompt is too long")
	ErrSiteSlugInvalid       = errors.New("site slug is invalid")
	ErrSiteSlugConflict      = errors.New("site slug is already in use")
	ErrNoDraftRevision       = errors.New("no draft revision available")
	ErrGenerationRateLimited = errors.New("generation is rate limited")
)

const maxGenerationValidationAttempts = 2

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type draftWriter interface {
	SaveDraft(ctx context.Context, workspaceID string, draft siteconfig.SiteDraft) error
}

type draftReader interface {
	LoadDraft(ctx context.Context, siteID string) (siteconfig.SiteDraft, error)
}

type GenerateInput struct {
	Name              string
	Slug              string
	Prompt            string
	PreferredLanguage string
	OptionalHints     map[string]string
	Brand             siteconfig.BrandConfig
	InterviewAnswers  []ClarifyingAnswer
}

type RepromptInput struct {
	Prompt string
}

type GenerateResult struct {
	JobID string               `json:"jobId"`
	Draft siteconfig.SiteDraft `json:"draft"`
}

type generationPlanFeedback struct {
	Attempt          int
	ValidationIssues []siteconfig.Issue
	ReportProgress   func(stepName string)
}

type generationPlanBuilder func(context.Context, generationInputContext, generationPlanFeedback) (generationPlan, error)

type Service struct {
	db                   DB
	reader               draftReader
	writer               draftWriter
	planner              generationPlanBuilder
	suggester            BlockSuggester
	imageRewriter        ImageQueryRewriter
	pageChangeSetPlanner PageChangeSetPlanner
	clarifyingPlanner    ClarifyingQuestionPlanner
	decomposedPlanner    DecomposedPlanner
	imagery              *StarterImagery
	assetImporter        AssetImporter
	logger               *slog.Logger
	recorder             *audit.Recorder
}

// ServiceOption customizes the Service constructed by NewService.
type ServiceOption func(*Service)

// WithStarterImagery attaches a starter imagery provider so generation can
// fetch backend-owned starter images for image slots.
func WithStarterImagery(imagery *StarterImagery) ServiceOption {
	return func(s *Service) {
		s.imagery = imagery
	}
}

// WithAssetImporter wires the asset import target used when starter
// imagery is enabled. Required for starter imagery to take effect.
func WithAssetImporter(importer AssetImporter) ServiceOption {
	return func(s *Service) {
		s.assetImporter = importer
	}
}

// WithLogger sets a structured logger used when starter imagery falls back.
func WithLogger(logger *slog.Logger) ServiceOption {
	return func(s *Service) {
		s.logger = logger
	}
}

// WithAuditRecorder attaches an audit recorder so generation and re-prompt
// lifecycle events are written to audit_events.
func WithAuditRecorder(recorder *audit.Recorder) ServiceOption {
	return func(s *Service) {
		s.recorder = recorder
	}
}

// WithBlockSuggester wires the block-suggest rewriter used by
// SuggestBlock. When nil, SuggestBlock returns ErrBlockSuggestUnavailable.
func WithBlockSuggester(suggester BlockSuggester) ServiceOption {
	return func(s *Service) {
		s.suggester = suggester
	}
}

// WithImageQueryRewriter wires the model-side image query rewriter used by
// SuggestImage. When nil, SuggestImage falls back to the deterministic
// headline/page-title heuristic.
func WithImageQueryRewriter(rewriter ImageQueryRewriter) ServiceOption {
	return func(s *Service) {
		s.imageRewriter = rewriter
	}
}

// WithPageChangeSetPlanner wires the diff-style page reprompt planner. When
// nil (or when the block suggester is also nil) RepromptPageWithProgress
// falls back to the legacy whole-page regeneration path.
func WithPageChangeSetPlanner(planner PageChangeSetPlanner) ServiceOption {
	return func(s *Service) {
		s.pageChangeSetPlanner = planner
	}
}

// WithClarifyingQuestionPlanner wires the intake-form planner used by
// BuildInterviewQuestions. When nil, the interview endpoint returns no
// questions and generation proceeds without an intake step.
func WithClarifyingQuestionPlanner(planner ClarifyingQuestionPlanner) ServiceOption {
	return func(s *Service) {
		s.clarifyingPlanner = planner
	}
}

// WithDecomposedPlanner wires the three-step generation pipeline. When set,
// generation prefers outline → page layout → page content over the legacy
// single BuildPlan call.
func WithDecomposedPlanner(planner DecomposedPlanner) ServiceOption {
	return func(s *Service) {
		s.decomposedPlanner = planner
	}
}

func (s *Service) recordAudit(ctx context.Context, event audit.Event) {
	if s == nil || s.recorder == nil {
		return
	}
	if err := s.recorder.Record(ctx, event); err != nil {
		if s.logger != nil {
			s.logger.Warn("record audit event",
				"action", event.Action,
				"siteId", event.SiteID,
				"workspaceId", event.WorkspaceID,
				"error", err.Error(),
			)
		}
	}
}

func NewService(db DB, planner generationPlanBuilder, options ...ServiceOption) *Service {
	if planner == nil {
		planner = defaultGenerationPlanBuilder
	}
	service := &Service{
		db:      db,
		reader:  sites.NewPostgresReader(db),
		writer:  sites.NewPostgresWriter(db),
		planner: planner,
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

func (s *Service) Generate(ctx context.Context, workspaceID string, userID string, input GenerateInput) (GenerateResult, error) {
	return s.GenerateWithProgress(ctx, workspaceID, userID, input, nil)
}

// BuildInterviewQuestions runs the cheapest call in the pipeline: ask the
// model whether anything would meaningfully sharpen the generated site if
// the user answered first. Returns an empty list when the planner is not
// configured or when the model decides nothing is worth asking.
func (s *Service) BuildInterviewQuestions(ctx context.Context, input GenerateInput) ([]ClarifyingQuestion, error) {
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return nil, ErrPromptRequired
	}
	if s.clarifyingPlanner == nil {
		return nil, nil
	}
	questions, err := s.clarifyingPlanner.BuildClarifyingQuestions(ctx, ClarifyingQuestionsRequest{
		Prompt:        prompt,
		NameHint:      strings.TrimSpace(input.Name),
		Brand:         input.Brand,
		OptionalHints: cloneStringMap(input.OptionalHints),
	})
	if err != nil {
		return nil, err
	}
	if len(questions) > MaxClarifyingQuestions {
		questions = questions[:MaxClarifyingQuestions]
	}
	return questions, nil
}

func (s *Service) GenerateWithProgress(ctx context.Context, workspaceID string, userID string, input GenerateInput, sink ProgressSink) (GenerateResult, error) {
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return GenerateResult{}, ErrPromptRequired
	}
	s.pruneGenerationJobs(ctx)

	inputContext := generationInputContext{
		NameHint:          strings.TrimSpace(input.Name),
		SlugHint:          strings.TrimSpace(input.Slug),
		Prompt:            prompt,
		PreferredLanguage: strings.TrimSpace(input.PreferredLanguage),
		OptionalHints:     cloneStringMap(input.OptionalHints),
		Brand:             input.Brand,
		InterviewAnswers:  input.InterviewAnswers,
	}
	jobID, err := s.createGenerationJob(ctx, workspaceID, userID, JobKindSite, inputContext)
	if err != nil {
		return GenerateResult{}, err
	}
	tracker := newProgressTracker(s, jobID, JobKindSite, ProgressStepsForKind(JobKindSite, s.imagery.available()), sink)
	if sink != nil {
		sink.OnJobCreated(jobID)
	}
	if err := tracker.emit(ctx, "prompt.normalize"); err != nil {
		return GenerateResult{}, err
	}

	var (
		plan                 generationPlan
		draft                siteconfig.SiteDraft
		validationRetryCount int
	)
	partialEmitter := partialEmitterFromSink(sink)
	plan, draft, err = s.generateDraftDecomposed(ctx, workspaceID, inputContext, tracker, partialEmitter)
	if err != nil {
		if !errors.Is(err, ErrDecomposedPlannerUnavailable) && s.logger != nil {
			s.logger.Warn("decomposed generation failed; falling back",
				"workspaceId", workspaceID,
				"error", err.Error(),
			)
		}
		plan, draft, validationRetryCount, err = s.generateDraftWithRetry(ctx, workspaceID, inputContext, tracker)
	}
	if err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}

	if err := tracker.emit(ctx, "persist"); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}
	if enriched, ok := s.enrichDraftWithStarterImagery(ctx, workspaceID, userID, draft, prompt); ok {
		draft = enriched
	}

	metadataErr := s.saveSiteMetadata(ctx, workspaceID, draft.Site.ID, prompt, plan, validationRetryCount)
	jobErr := s.completeGenerationJob(ctx, jobID, draft.Site.ID, plan)
	if metadataErr != nil || jobErr != nil {
		persistErr := errors.Join(metadataErr, jobErr)
		_ = s.failGenerationJob(ctx, jobID, persistErr)
		return GenerateResult{}, persistErr
	}

	s.recordAudit(ctx, audit.Event{
		WorkspaceID: workspaceID,
		SiteID:      draft.Site.ID,
		UserID:      userID,
		Action:      "site.generate",
		Metadata: map[string]any{
			"jobId":                jobID,
			"siteName":             draft.Site.Name,
			"siteSlug":             draft.Site.Slug,
			"themePreset":          plan.ThemePreset,
			"pageCount":            len(plan.Pages),
			"validationRetryCount": validationRetryCount,
		},
	})

	return GenerateResult{
		JobID: jobID,
		Draft: draft,
	}, nil
}

func (s *Service) RepromptSite(ctx context.Context, workspaceID string, userID string, siteID string, input RepromptInput) (GenerateResult, error) {
	return s.RepromptSiteWithProgress(ctx, workspaceID, userID, siteID, input, nil)
}

func (s *Service) RepromptSiteWithProgress(ctx context.Context, workspaceID string, userID string, siteID string, input RepromptInput, sink ProgressSink) (GenerateResult, error) {
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return GenerateResult{}, ErrPromptRequired
	}
	s.pruneGenerationJobs(ctx)

	currentDraft, err := s.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return GenerateResult{}, err
	}

	metadata, err := s.loadSiteMetadata(ctx, workspaceID, siteID)
	if err != nil {
		return GenerateResult{}, err
	}
	previousRevisionID, err := s.captureDraftRevision(ctx, workspaceID, siteID, draftRevisionRecord{
		Scope:                 "site",
		Prompt:                prompt,
		Draft:                 currentDraft,
		GenerationPrompt:      metadata.Prompt,
		GenerationSummaryJSON: metadata.SummaryJSON,
		CreatedBy:             userID,
	})
	if err != nil {
		return GenerateResult{}, err
	}

	inputContext := generationInputContext{
		SiteID:   siteID,
		NameHint: currentDraft.Site.Name,
		Prompt:   prompt,
		Scope:    "site",
	}
	jobID, err := s.createGenerationJob(ctx, workspaceID, userID, JobKindSiteReprompt, inputContext)
	if err != nil {
		return GenerateResult{}, err
	}
	tracker := newProgressTracker(s, jobID, JobKindSiteReprompt, ProgressStepsForKind(JobKindSiteReprompt, s.imagery.available()), sink)
	if sink != nil {
		sink.OnJobCreated(jobID)
	}
	if err := tracker.emit(ctx, "prompt.normalize"); err != nil {
		return GenerateResult{}, err
	}

	var (
		plan                 generationPlan
		nextDraft            siteconfig.SiteDraft
		validationRetryCount int
	)
	partialEmitter := partialEmitterFromSink(sink)
	plan, nextDraft, err = s.repromptSiteDecomposed(ctx, workspaceID, currentDraft, prompt, tracker, partialEmitter)
	if err != nil {
		if !errors.Is(err, ErrDecomposedPlannerUnavailable) && s.logger != nil {
			s.logger.Warn("decomposed site reprompt failed; falling back",
				"siteId", siteID,
				"error", err.Error(),
			)
		}
		plan, nextDraft, validationRetryCount, err = s.generateDraftWithRetry(ctx, workspaceID, inputContext, tracker)
	}
	if err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}

	if err := tracker.emit(ctx, "persist"); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}
	nextDraft = applySiteIdentity(nextDraft, currentDraft)
	if err := s.writer.SaveDraft(ctx, workspaceID, nextDraft); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}

	if enriched, ok := s.enrichDraftWithStarterImagery(ctx, workspaceID, userID, nextDraft, prompt); ok {
		nextDraft = enriched
	}

	if err := s.saveSiteMetadata(ctx, workspaceID, siteID, prompt, plan, validationRetryCount); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}
	if err := s.completeGenerationJob(ctx, jobID, siteID, plan); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}

	savedDraft, err := s.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return GenerateResult{}, err
	}
	resultSummaryJSON, err := json.Marshal(map[string]any{
		"siteGoal":             plan.SiteGoal,
		"themePreset":          plan.ThemePreset,
		"assetsNeeded":         plan.AssetsNeeded,
		"assumptions":          plan.Assumptions,
		"pageCount":            len(plan.Pages),
		"validationRetryCount": validationRetryCount,
	})
	if err != nil {
		return GenerateResult{}, fmt.Errorf("encode reprompt summary: %w", err)
	}
	resultRevisionID, err := s.captureDraftRevision(ctx, workspaceID, siteID, draftRevisionRecord{
		Scope:                 "site",
		Prompt:                prompt,
		Draft:                 savedDraft,
		GenerationPrompt:      prompt,
		GenerationSummaryJSON: resultSummaryJSON,
		CreatedBy:             userID,
	})
	if err != nil {
		return GenerateResult{}, err
	}
	if err := s.recordRepromptHistory(ctx, workspaceID, siteID, repromptHistoryRecord{
		Scope:              "site",
		Prompt:             prompt,
		ChangeSummary:      summarizeReprompt("site", savedDraft, ""),
		PreviousRevisionID: previousRevisionID,
		ResultRevisionID:   resultRevisionID,
		JobID:              jobID,
		CreatedBy:          userID,
	}); err != nil {
		return GenerateResult{}, err
	}
	s.recordAudit(ctx, audit.Event{
		WorkspaceID: workspaceID,
		SiteID:      siteID,
		UserID:      userID,
		Action:      "site.reprompt",
		Metadata: map[string]any{
			"jobId":                jobID,
			"themePreset":          plan.ThemePreset,
			"pageCount":            len(plan.Pages),
			"validationRetryCount": validationRetryCount,
		},
	})
	return GenerateResult{
		JobID: jobID,
		Draft: savedDraft,
	}, nil
}

func (s *Service) RepromptPage(ctx context.Context, workspaceID string, userID string, siteID string, pageID string, input RepromptInput) (GenerateResult, error) {
	return s.RepromptPageWithProgress(ctx, workspaceID, userID, siteID, pageID, input, nil)
}

func (s *Service) RepromptPageWithProgress(ctx context.Context, workspaceID string, userID string, siteID string, pageID string, input RepromptInput, sink ProgressSink) (GenerateResult, error) {
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return GenerateResult{}, ErrPromptRequired
	}
	s.pruneGenerationJobs(ctx)

	currentDraft, err := s.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return GenerateResult{}, err
	}
	pageIndex := findDraftPageIndex(currentDraft, pageID)
	if pageIndex == -1 {
		return GenerateResult{}, sites.ErrPageNotFound
	}

	metadata, err := s.loadSiteMetadata(ctx, workspaceID, siteID)
	if err != nil {
		return GenerateResult{}, err
	}
	previousRevisionID, err := s.captureDraftRevision(ctx, workspaceID, siteID, draftRevisionRecord{
		Scope:                 "page",
		PageID:                pageID,
		Prompt:                prompt,
		Draft:                 currentDraft,
		GenerationPrompt:      metadata.Prompt,
		GenerationSummaryJSON: metadata.SummaryJSON,
		CreatedBy:             userID,
	})
	if err != nil {
		return GenerateResult{}, err
	}

	page := currentDraft.Pages[pageIndex]
	inputContext := generationInputContext{
		SiteID:   siteID,
		PageID:   pageID,
		NameHint: currentDraft.Site.Name,
		Prompt:   prompt,
		Scope:    "page",
	}
	jobID, err := s.createGenerationJob(ctx, workspaceID, userID, JobKindPageReprompt, inputContext)
	if err != nil {
		return GenerateResult{}, err
	}
	tracker := newProgressTracker(s, jobID, JobKindPageReprompt, ProgressStepsForKind(JobKindPageReprompt, false), sink)
	if sink != nil {
		sink.OnJobCreated(jobID)
	}
	if err := tracker.emit(ctx, "prompt.normalize"); err != nil {
		return GenerateResult{}, err
	}
	if err := tracker.emit(ctx, "plan.blocks"); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}

	pageResult, err := s.repromptPage(ctx, currentDraft, page, prompt)
	if err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}
	if err := tracker.emit(ctx, "copy.write"); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}
	if err := tracker.emit(ctx, "validate.repair"); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}

	nextDraft := currentDraft
	nextDraft.Pages = append([]siteconfig.PageDraft(nil), currentDraft.Pages...)
	nextDraft.Pages[pageIndex] = pageResult.Page
	nextDraft.Navigation = syncRepromptNavigation(currentDraft.Navigation, nextDraft.Pages)

	if err := tracker.emit(ctx, "persist"); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}
	if err := s.writer.SaveDraft(ctx, workspaceID, nextDraft); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}

	pageSummaryPlan := generationPlan{
		SiteName:     currentDraft.Site.Name,
		SiteGoal:     currentDraft.Site.SEO.Description,
		ThemePreset:  metadata.themePreset(),
		Theme:        currentDraft.Theme,
		Pages:        []generationPagePlan{pageResult.Plan},
		AssetsNeeded: metadata.assetsNeeded(),
		Assumptions:  metadata.assumptions(),
	}
	if err := s.completeGenerationJob(ctx, jobID, siteID, pageSummaryPlan); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}

	savedDraft, err := s.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return GenerateResult{}, err
	}
	resultRevisionID, err := s.captureDraftRevision(ctx, workspaceID, siteID, draftRevisionRecord{
		Scope:                 "page",
		PageID:                pageID,
		Prompt:                prompt,
		Draft:                 savedDraft,
		GenerationPrompt:      metadata.Prompt,
		GenerationSummaryJSON: metadata.SummaryJSON,
		CreatedBy:             userID,
	})
	if err != nil {
		return GenerateResult{}, err
	}
	if err := s.recordRepromptHistory(ctx, workspaceID, siteID, repromptHistoryRecord{
		Scope:              "page",
		TargetID:           pageID,
		Prompt:             prompt,
		ChangeSummary:      firstNonEmpty(pageResult.ChangeSummary, summarizeReprompt("page", savedDraft, pageID)),
		PreviousRevisionID: previousRevisionID,
		ResultRevisionID:   resultRevisionID,
		JobID:              jobID,
		CreatedBy:          userID,
	}); err != nil {
		return GenerateResult{}, err
	}
	s.recordAudit(ctx, audit.Event{
		WorkspaceID: workspaceID,
		SiteID:      siteID,
		UserID:      userID,
		Action:      "page.reprompt",
		Metadata: map[string]any{
			"jobId":  jobID,
			"pageId": pageID,
			"title":  pageResult.Plan.Title,
			"slug":   pageResult.Plan.Slug,
			"blocks": len(pageResult.Plan.Blocks),
		},
	})
	return GenerateResult{
		JobID: jobID,
		Draft: savedDraft,
	}, nil
}

func (s *Service) UndoLastDraftRevision(ctx context.Context, workspaceID string, siteID string) (siteconfig.SiteDraft, error) {
	entry, err := s.loadLatestRepromptHistory(ctx, workspaceID, siteID)
	if err == nil {
		return s.RevertReprompt(ctx, workspaceID, siteID, entry.ID)
	}
	if !errors.Is(err, ErrRepromptNotFound) {
		return siteconfig.SiteDraft{}, err
	}

	revision, err := s.loadLatestDraftRevision(ctx, workspaceID, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	currentDraft, err := s.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	revision.Draft.Revision = currentDraft.Revision

	if err := s.writer.SaveDraft(ctx, workspaceID, revision.Draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	if err := s.restoreSiteMetadata(ctx, workspaceID, siteID, revision.GenerationPrompt, revision.GenerationSummaryJSON); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	if err := s.deleteDraftRevision(ctx, revision.ID); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	return s.reader.LoadDraft(ctx, siteID)
}

func defaultGenerationPlanBuilder(_ context.Context, input generationInputContext, feedback generationPlanFeedback) (generationPlan, error) {
	if feedback.ReportProgress != nil {
		feedback.ReportProgress("plan.pages")
		feedback.ReportProgress("plan.theme")
		feedback.ReportProgress("plan.blocks")
	}
	return buildGenerationPlan(input.NameHint, input.Prompt), nil
}

// enrichDraftWithStarterImagery fills empty image slots in the supplied
// draft using the configured imagery provider, persists the updated draft,
// and returns the enriched draft. When imagery is not configured or the
// provider returns no usable images the draft is returned unchanged and the
// second return value is false.
func (s *Service) enrichDraftWithStarterImagery(ctx context.Context, workspaceID string, userID string, draft siteconfig.SiteDraft, prompt string) (siteconfig.SiteDraft, bool) {
	if !s.imagery.available() || s.assetImporter == nil {
		return draft, false
	}

	enriched := s.applyStarterImagery(ctx, workspaceID, userID, cloneDraftShallow(draft), prompt)
	if !draftImageSlotsChanged(draft, enriched) {
		return draft, false
	}
	// The caller just persisted the draft, which bumped the row's revision but
	// did not update the in-memory copy. Re-sync from the reader so the
	// optimistic-concurrency check in SaveDraft uses the current revision
	// instead of the pre-save value.
	if s.reader != nil {
		if current, err := s.reader.LoadDraft(ctx, draft.Site.ID); err == nil {
			enriched.Revision = current.Revision
		}
	}
	if err := s.writer.SaveDraft(ctx, workspaceID, enriched); err != nil {
		if s.logger != nil {
			s.logger.Warn("persist starter imagery enrichment", "siteId", draft.Site.ID, "error", err.Error())
		}
		return draft, false
	}
	return enriched, true
}

func cloneDraftShallow(draft siteconfig.SiteDraft) siteconfig.SiteDraft {
	clone := draft
	clone.Pages = make([]siteconfig.PageDraft, len(draft.Pages))
	for pageIndex, page := range draft.Pages {
		clonedPage := page
		clonedPage.Blocks = make([]siteconfig.BlockInstance, len(page.Blocks))
		for blockIndex, block := range page.Blocks {
			clonedBlock := block
			if block.Props != nil {
				clonedBlock.Props = deepCloneProps(block.Props)
			}
			clonedPage.Blocks[blockIndex] = clonedBlock
		}
		clone.Pages[pageIndex] = clonedPage
	}
	return clone
}

func deepCloneProps(props map[string]any) map[string]any {
	clone := make(map[string]any, len(props))
	for key, value := range props {
		clone[key] = deepCloneValue(value)
	}
	return clone
}

func deepCloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return deepCloneProps(typed)
	case []any:
		copyList := make([]any, len(typed))
		for index, item := range typed {
			copyList[index] = deepCloneValue(item)
		}
		return copyList
	default:
		return typed
	}
}

func draftImageSlotsChanged(before siteconfig.SiteDraft, after siteconfig.SiteDraft) bool {
	if len(before.Pages) != len(after.Pages) {
		return true
	}
	for pageIndex, beforePage := range before.Pages {
		afterPage := after.Pages[pageIndex]
		if len(beforePage.Blocks) != len(afterPage.Blocks) {
			return true
		}
		for blockIndex, beforeBlock := range beforePage.Blocks {
			afterBlock := afterPage.Blocks[blockIndex]
			if propsHaveDifferentAssets(beforeBlock.Props, afterBlock.Props) {
				return true
			}
		}
	}
	return false
}

func propsHaveDifferentAssets(before map[string]any, after map[string]any) bool {
	beforeIDs := siteconfig.CollectAssetIDs(siteconfig.BrandConfig{}, []siteconfig.PageDraft{{Blocks: []siteconfig.BlockInstance{{Props: before}}}}, nil)
	afterIDs := siteconfig.CollectAssetIDs(siteconfig.BrandConfig{}, []siteconfig.PageDraft{{Blocks: []siteconfig.BlockInstance{{Props: after}}}}, nil)
	if len(beforeIDs) != len(afterIDs) {
		return true
	}
	for id := range afterIDs {
		if _, ok := beforeIDs[id]; !ok {
			return true
		}
	}
	return false
}

func (s *Service) generateDraftWithRetry(ctx context.Context, workspaceID string, input generationInputContext, tracker *progressTracker) (generationPlan, siteconfig.SiteDraft, int, error) {
	feedback := generationPlanFeedback{}
	for attempt := 1; attempt <= maxGenerationValidationAttempts; attempt++ {
		feedback.Attempt = attempt
		feedback.ReportProgress = func(stepName string) {
			if tracker == nil {
				return
			}
			if err := tracker.emit(ctx, stepName); err != nil && s.logger != nil {
				s.logger.Warn("emit generation progress", "jobId", tracker.jobID, "step", stepName, "error", err.Error())
			}
		}

		plan, err := s.buildPlan(ctx, input, feedback)
		if err != nil {
			return generationPlan{}, siteconfig.SiteDraft{}, attempt - 1, fmt.Errorf("build generation plan: %w", err)
		}
		plan = repairGenerationPlan(plan)
		if tracker != nil && len(plan.AssetsNeeded) > 0 && s.imagery.available() {
			if err := tracker.emit(ctx, "assets.fetch"); err != nil {
				return generationPlan{}, siteconfig.SiteDraft{}, attempt - 1, err
			}
		}
		if tracker != nil {
			if err := tracker.emit(ctx, "copy.write"); err != nil {
				return generationPlan{}, siteconfig.SiteDraft{}, attempt - 1, err
			}
			if err := tracker.emit(ctx, "validate.repair"); err != nil {
				return generationPlan{}, siteconfig.SiteDraft{}, attempt - 1, err
			}
		}

		slugValue, err := s.createSlug(ctx, workspaceID, input.SlugHint, plan.SiteName)
		if err != nil {
			return generationPlan{}, siteconfig.SiteDraft{}, attempt - 1, err
		}

		draft, err := buildDraftFromPlan(plan, slugValue, input.PreferredLanguage, input.Brand)
		if err == nil {
			err = s.writer.SaveDraft(ctx, workspaceID, draft)
		}
		if err == nil {
			return plan, draft, attempt - 1, nil
		}

		var validationErr siteconfig.ValidationError
		if attempt < maxGenerationValidationAttempts && errors.As(err, &validationErr) {
			feedback.ValidationIssues = append([]siteconfig.Issue(nil), validationErr.Issues...)
			continue
		}

		return generationPlan{}, siteconfig.SiteDraft{}, attempt - 1, err
	}

	return generationPlan{}, siteconfig.SiteDraft{}, maxGenerationValidationAttempts - 1, errors.New("generation retry attempts exhausted")
}

func (s *Service) buildPlan(ctx context.Context, input generationInputContext, feedback generationPlanFeedback) (generationPlan, error) {
	if s.planner != nil {
		return s.planner(ctx, input, feedback)
	}
	return defaultGenerationPlanBuilder(ctx, input, feedback)
}

type generationInputContext struct {
	SiteID            string                 `json:"siteId,omitempty"`
	PageID            string                 `json:"pageId,omitempty"`
	NameHint          string                 `json:"nameHint,omitempty"`
	SlugHint          string                 `json:"slugHint,omitempty"`
	Prompt            string                 `json:"prompt"`
	Scope             string                 `json:"scope,omitempty"`
	PreferredLanguage string                 `json:"preferredLanguage,omitempty"`
	OptionalHints     map[string]string      `json:"optionalHints,omitempty"`
	Brand             siteconfig.BrandConfig `json:"brand,omitempty"`
	InterviewAnswers  []ClarifyingAnswer     `json:"interviewAnswers,omitempty"`
}

type generationPlan struct {
	SiteName       string                    `json:"siteName"`
	SiteGoal       string                    `json:"siteGoal"`
	ThemePreset    string                    `json:"themePreset"`
	ThemeSelection siteconfig.ThemeSelection `json:"themeSelection,omitempty"`
	Theme          siteconfig.ThemeConfig    `json:"theme"`
	Pages          []generationPagePlan      `json:"pages"`
	AssetsNeeded   []string                  `json:"assetsNeeded"`
	Assumptions    []string                  `json:"assumptions"`
}

type generationPagePlan struct {
	Title  string                `json:"title"`
	Slug   string                `json:"slug"`
	Goal   string                `json:"goal"`
	Blocks []generationBlockPlan `json:"blocks"`
	SEO    siteconfig.SEOConfig  `json:"seo"`
}

type generationBlockPlan struct {
	Type    string         `json:"type"`
	Purpose string         `json:"purpose"`
	Props   map[string]any `json:"props"`
}

type repromptPageResult struct {
	Page          siteconfig.PageDraft
	Plan          generationPagePlan
	ChangeSummary string
}

type siteMetadata struct {
	Prompt      string
	SummaryJSON []byte
	Summary     map[string]any
}

type draftRevisionRecord struct {
	ID                    string
	Scope                 string
	PageID                string
	Prompt                string
	Draft                 siteconfig.SiteDraft
	GenerationPrompt      string
	GenerationSummaryJSON []byte
	CreatedBy             string
	CreatedAt             time.Time
}

type promptProfile struct {
	Category          string
	CategoryLabel     string
	ThemePreset       string
	PrimaryCTA        string
	ServicesTitle     string
	ServicesIntro     string
	FeatureItems      []map[string]any
	AboutHeading      string
	AboutBody         string
	GalleryHeading    string
	GalleryBody       string
	ContactHeading    string
	ContactBody       string
	WantsGallery      bool
	WantsWorkshops    bool
	WantsPricing      bool
	WantsTestimonials bool
	WantsFAQ          bool
	WantsTeam         bool
}

func (s *Service) createGenerationJob(ctx context.Context, workspaceID string, userID string, kind JobKind, input generationInputContext) (string, error) {
	return s.promptActions().CreateJob(ctx, PromptActionInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Kind:        kind,
		Prompt:      input.Prompt,
		Payload:     input,
	})
}

func (s *Service) completeGenerationJob(ctx context.Context, jobID string, siteID string, plan generationPlan) error {
	return s.promptActions().CompleteJob(ctx, jobID, siteID, plan)
}

func (s *Service) failGenerationJob(ctx context.Context, jobID string, cause error) error {
	return s.promptActions().FailJob(ctx, jobID, cause)
}

func (s *Service) promptActions() *PromptActionManager {
	return NewPromptActionManagerFromDB(s.db, s.logger)
}

func (s *Service) saveSiteMetadata(ctx context.Context, workspaceID string, siteID string, prompt string, plan generationPlan, validationRetryCount int) error {
	summaryJSON, err := json.Marshal(map[string]any{
		"siteGoal":             plan.SiteGoal,
		"themePreset":          plan.ThemePreset,
		"assetsNeeded":         plan.AssetsNeeded,
		"assumptions":          plan.Assumptions,
		"pageCount":            len(plan.Pages),
		"validationRetryCount": validationRetryCount,
	})
	if err != nil {
		return fmt.Errorf("encode generation summary: %w", err)
	}

	tag, err := s.db.Exec(ctx, `
		update sites
		set generation_prompt = $1,
		    generation_summary = $2,
		    updated_at = now()
		where id = $3::uuid
		  and workspace_id = $4::uuid
	`, prompt, summaryJSON, siteID, workspaceID)
	if err != nil {
		return fmt.Errorf("save generation summary: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return sites.ErrNotFound
	}
	return nil
}

func (s *Service) restoreSiteMetadata(ctx context.Context, workspaceID string, siteID string, prompt string, summaryJSON []byte) error {
	if len(summaryJSON) == 0 {
		summaryJSON = []byte(`{}`)
	}

	tag, err := s.db.Exec(ctx, `
		update sites
		set generation_prompt = $1,
		    generation_summary = $2,
		    updated_at = now()
		where id = $3::uuid
		  and workspace_id = $4::uuid
	`, prompt, summaryJSON, siteID, workspaceID)
	if err != nil {
		return fmt.Errorf("restore generation summary: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return sites.ErrNotFound
	}
	return nil
}

func (s *Service) loadSiteMetadata(ctx context.Context, workspaceID string, siteID string) (siteMetadata, error) {
	var prompt string
	var summaryJSON []byte
	if err := s.db.QueryRow(ctx, `
		select coalesce(generation_prompt, ''),
		       generation_summary
		from sites
		where id = $1::uuid
		  and workspace_id = $2::uuid
	`, siteID, workspaceID).Scan(&prompt, &summaryJSON); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return siteMetadata{}, sites.ErrNotFound
		}
		return siteMetadata{}, fmt.Errorf("load site metadata: %w", err)
	}

	summary := map[string]any{}
	if len(summaryJSON) > 0 {
		if err := json.Unmarshal(summaryJSON, &summary); err != nil {
			return siteMetadata{}, fmt.Errorf("decode generation summary: %w", err)
		}
	}

	return siteMetadata{
		Prompt:      prompt,
		SummaryJSON: summaryJSON,
		Summary:     summary,
	}, nil
}

func (s *Service) deleteDraftRevision(ctx context.Context, revisionID string) error {
	if _, err := s.db.Exec(ctx, `
		delete from draft_revisions
		where id = $1::uuid
	`, revisionID); err != nil {
		return fmt.Errorf("delete draft revision: %w", err)
	}
	return nil
}

func (s *Service) createSlug(ctx context.Context, workspaceID string, requested string, name string) (string, error) {
	if value := strings.TrimSpace(requested); value != "" {
		if !slugs.IsValid(value) {
			return "", ErrSiteSlugInvalid
		}
		taken, err := s.siteSlugExists(ctx, workspaceID, value)
		if err != nil {
			return "", err
		}
		if taken {
			return "", ErrSiteSlugConflict
		}
		return value, nil
	}

	value, err := slugs.EnsureUnique(name, func(candidate string) (bool, error) {
		return s.siteSlugExists(ctx, workspaceID, candidate)
	})
	if err != nil {
		return "", fmt.Errorf("generate site slug: %w", err)
	}
	return value, nil
}

func (m siteMetadata) themePreset() string {
	if preset, ok := m.Summary["themePreset"].(string); ok && preset != "" {
		return preset
	}
	return siteconfig.ThemePaletteCleanLocal
}

func (m siteMetadata) assetsNeeded() []string {
	items, ok := m.Summary["assetsNeeded"].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if ok && text != "" {
			result = append(result, text)
		}
	}
	return result
}

func (m siteMetadata) assumptions() []string {
	items, ok := m.Summary["assumptions"].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if ok && text != "" {
			result = append(result, text)
		}
	}
	return result
}

func (s *Service) siteSlugExists(ctx context.Context, workspaceID string, slugValue string) (bool, error) {
	var exists bool
	if err := s.db.QueryRow(ctx, `
		select exists(
			select 1
			from sites
			where workspace_id = $1::uuid
			  and slug = $2
		)
	`, workspaceID, slugValue).Scan(&exists); err != nil {
		return false, fmt.Errorf("check site slug: %w", err)
	}
	return exists, nil
}

func buildGenerationPlan(nameHint string, prompt string) generationPlan {
	profile := profilePrompt(prompt)
	siteName := deriveSiteName(nameHint, profile)
	primaryCTAHref := "/contact"
	if profile.WantsWorkshops {
		primaryCTAHref = "/workshops"
	}

	pages := []generationPagePlan{
		homePagePlan(siteName, prompt, profile, primaryCTAHref),
	}
	if profile.WantsWorkshops {
		pages = append(pages, workshopsPagePlan(siteName, profile))
	} else {
		pages = append(pages, servicesPagePlan(siteName, profile))
	}
	if profile.WantsGallery {
		pages = append(pages, galleryPagePlan(siteName, profile))
	} else {
		pages = append(pages, aboutPagePlan(siteName, profile))
	}
	pages = append(pages, contactPagePlan(siteName, profile))

	assetsNeeded := []string{}
	if profile.WantsGallery || profile.Category == "photography" || profile.Category == "craft" {
		assetsNeeded = append(assetsNeeded, "hero-image", "supporting-image")
	}

	return generationPlan{
		SiteName:     siteName,
		SiteGoal:     siteGoalForCategory(profile.Category),
		ThemePreset:  profile.ThemePreset,
		Theme:        siteconfig.ThemePreset(profile.ThemePreset),
		Pages:        pages,
		AssetsNeeded: assetsNeeded,
		Assumptions:  assumptionsForProfile(profile),
	}
}

func buildDraftFromPlan(plan generationPlan, slugValue string, preferredLanguage string, brandHint siteconfig.BrandConfig) (siteconfig.SiteDraft, error) {
	siteID, err := ids.New()
	if err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("generate site id: %w", err)
	}

	pages := make([]siteconfig.PageDraft, 0, len(plan.Pages))
	navigation := make([]siteconfig.NavigationItem, 0, len(plan.Pages))
	siteDescription := clampSentence(firstNonEmpty(plan.Pages[0].SEO.Description, plan.SiteGoal), 180)

	for _, pagePlan := range plan.Pages {
		pageID, err := ids.New()
		if err != nil {
			return siteconfig.SiteDraft{}, fmt.Errorf("generate page id: %w", err)
		}
		blocks := make([]siteconfig.BlockInstance, 0, len(pagePlan.Blocks))
		for _, blockPlan := range pagePlan.Blocks {
			blockID, err := ids.New()
			if err != nil {
				return siteconfig.SiteDraft{}, fmt.Errorf("generate block id: %w", err)
			}
			blocks = append(blocks, siteconfig.BlockInstance{
				ID:      blockID,
				Type:    blockPlan.Type,
				Version: siteconfig.BlockVersionV1,
				Props:   blockPlan.Props,
			})
		}

		pages = append(pages, siteconfig.PageDraft{
			ID:     pageID,
			Title:  pagePlan.Title,
			Slug:   pagePlan.Slug,
			Status: siteconfig.PageStatusDraft,
			SEO:    pagePlan.SEO,
			Blocks: blocks,
		})
		navigation = append(navigation, siteconfig.NavigationItem{
			Label:  pagePlan.Title,
			PageID: pageID,
		})
	}

	brand := brandHint
	if strings.TrimSpace(brand.BusinessName) == "" {
		brand.BusinessName = plan.SiteName
	}
	if strings.TrimSpace(brand.PrimaryColor) == "" {
		brand.PrimaryColor = plan.Theme.Tokens.Colors["primary"]
	}
	plan.Theme = siteconfig.BuildThemeWithBrand(
		siteconfig.DetectThemeSelection(plan.Theme),
		brand,
	)

	draft := siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:            siteID,
			Name:          plan.SiteName,
			Slug:          slugValue,
			Status:        "draft",
			DefaultLocale: firstNonEmpty(strings.TrimSpace(preferredLanguage), "en"),
			SEO: siteconfig.SEOConfig{
				Title:       clampSentence(plan.SiteName, 70),
				Description: siteDescription,
			},
		},
		Brand:      brand,
		Theme:      plan.Theme,
		Navigation: siteconfig.NavigationConfig{Primary: navigation},
		Pages:      pages,
	}

	if err := siteconfig.ValidateDraft(draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	return draft, nil
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func applySiteIdentity(nextDraft siteconfig.SiteDraft, currentDraft siteconfig.SiteDraft) siteconfig.SiteDraft {
	nextDraft.Site.ID = currentDraft.Site.ID
	nextDraft.Site.Slug = currentDraft.Site.Slug
	nextDraft.Site.Status = currentDraft.Site.Status
	nextDraft.Site.DefaultLocale = currentDraft.Site.DefaultLocale
	if currentDraft.Brand.BusinessName != "" || currentDraft.Brand.PrimaryColor != "" || currentDraft.Brand.Logo != nil {
		nextDraft.Brand = currentDraft.Brand
	}
	if currentBrandColor := currentDraft.Brand.PrimaryColor; currentBrandColor != "" {
		nextDraft.Theme = siteconfig.BuildThemeWithBrand(
			siteconfig.DetectThemeSelection(nextDraft.Theme),
			currentDraft.Brand,
		)
	}
	return nextDraft
}

func findDraftPageIndex(draft siteconfig.SiteDraft, pageID string) int {
	for index, page := range draft.Pages {
		if page.ID == pageID {
			return index
		}
	}
	return -1
}

func replaceDraftPage(currentPage siteconfig.PageDraft, plan generationPagePlan) siteconfig.PageDraft {
	blocks := make([]siteconfig.BlockInstance, 0, len(plan.Blocks))
	for _, blockPlan := range plan.Blocks {
		blockID, err := ids.New()
		if err != nil {
			continue
		}
		blocks = append(blocks, siteconfig.BlockInstance{
			ID:      blockID,
			Type:    blockPlan.Type,
			Version: siteconfig.BlockVersionV1,
			Props:   blockPlan.Props,
		})
	}

	updatedPage := currentPage
	updatedPage.Title = firstNonEmpty(plan.Title, currentPage.Title)
	updatedPage.SEO = plan.SEO
	updatedPage.Slug = currentPage.Slug
	updatedPage.Blocks = blocks
	return updatedPage
}

func syncRepromptNavigation(
	current siteconfig.NavigationConfig,
	pages []siteconfig.PageDraft,
) siteconfig.NavigationConfig {
	pageByID := make(map[string]siteconfig.PageDraft, len(pages))
	for _, page := range pages {
		pageByID[page.ID] = page
	}

	nextPrimary := make([]siteconfig.NavigationItem, 0, len(current.Primary))
	seenPages := map[string]bool{}
	for _, item := range current.Primary {
		if item.PageID == "" {
			nextPrimary = append(nextPrimary, item)
			continue
		}
		page, ok := pageByID[item.PageID]
		if !ok {
			continue
		}
		if !includePageInNavigation(page) {
			continue
		}
		label := strings.TrimSpace(item.Label)
		if label == "" || label == page.Title {
			label = page.Title
		}
		nextPrimary = append(nextPrimary, siteconfig.NavigationItem{
			Label:  label,
			PageID: page.ID,
			Href:   "",
		})
		seenPages[page.ID] = true
	}

	for _, page := range pages {
		if seenPages[page.ID] || !includePageInNavigation(page) {
			continue
		}
		nextPrimary = append(nextPrimary, siteconfig.NavigationItem{
			Label:  page.Title,
			PageID: page.ID,
		})
	}

	return siteconfig.NavigationConfig{Primary: nextPrimary}
}

func includePageInNavigation(page siteconfig.PageDraft) bool {
	include, ok := page.Settings["includeInNavigation"]
	if !ok {
		return true
	}
	value, ok := include.(bool)
	if !ok {
		return true
	}
	return value
}

func (s *Service) repromptPage(
	ctx context.Context,
	draft siteconfig.SiteDraft,
	page siteconfig.PageDraft,
	prompt string,
) (repromptPageResult, error) {
	nextPage, plan, changeSummary, err := s.applyPageChangeSetWithSummary(ctx, draft, page, prompt)
	if err == nil {
		return repromptPageResult{
			Page:          nextPage,
			Plan:          plan,
			ChangeSummary: changeSummary,
		}, nil
	}
	if !errors.Is(err, ErrPageChangeSetUnavailable) && !errors.Is(err, ErrPageChangeSetEmpty) && s.logger != nil {
		s.logger.Warn("page change-set reprompt failed; falling back",
			"siteId", draft.Site.ID,
			"pageId", page.ID,
			"error", err.Error(),
		)
	}

	plan, err = s.buildWholePageRepromptPlan(ctx, draft, page, prompt)
	if err != nil {
		return repromptPageResult{}, err
	}
	return repromptPageResult{
		Page: replaceDraftPage(page, plan),
		Plan: plan,
	}, nil
}

func (s *Service) buildPageRepromptPlan(
	ctx context.Context,
	draft siteconfig.SiteDraft,
	page siteconfig.PageDraft,
	prompt string,
) (generationPagePlan, error) {
	result, err := s.repromptPage(ctx, draft, page, prompt)
	if err != nil {
		return generationPagePlan{}, err
	}
	return result.Plan, nil
}

func (s *Service) buildWholePageRepromptPlan(
	ctx context.Context,
	draft siteconfig.SiteDraft,
	page siteconfig.PageDraft,
	prompt string,
) (generationPagePlan, error) {
	fallback := fallbackPageRepromptPlan(draft, page, prompt)

	if s.decomposedPlanner != nil {
		outline := summarizeDraftAsOutline(draft)
		outlinePage := OutlinePage{
			Title: firstNonEmpty(page.Title, "Page"),
			Slug:  page.Slug,
			Goal:  firstNonEmpty(strings.TrimSpace(prompt), page.SEO.Description, draft.Site.SEO.Description),
			SEO:   page.SEO,
		}
		pagePlan, err := s.buildPagePlanFromLayout(ctx, draft.Site.Name, draft.Site.SEO.Description, draft.Brand, outlinePage, outline.Pages, nil)
		if err == nil && len(pagePlan.Blocks) > 0 {
			pagePlan.Title = firstNonEmpty(pagePlan.Title, page.Title)
			pagePlan.Slug = page.Slug
			return pagePlan, nil
		}
		if err != nil && s.logger != nil {
			s.logger.Warn("page layout reprompt failed; falling back",
				"siteId", draft.Site.ID,
				"pageId", page.ID,
				"error", err.Error(),
			)
		}
	}

	plan, err := s.buildPlan(ctx, generationInputContext{
		SiteID:   draft.Site.ID,
		PageID:   page.ID,
		NameHint: draft.Site.Name,
		Prompt:   prompt,
		Scope:    "page",
	}, generationPlanFeedback{})
	if err != nil {
		return fallback, nil
	}

	if len(plan.Pages) > 0 {
		pagePlan := repairSecondaryPage(draft.Site.Name, draft.Site.SEO.Description, plan.Pages[0], map[string]bool{
			page.Slug: true,
		})
		if len(pagePlan.Blocks) > 0 {
			pagePlan.Title = firstNonEmpty(pagePlan.Title, page.Title)
			pagePlan.Slug = page.Slug
			return pagePlan, nil
		}
	}

	return fallback, nil
}

func fallbackPageRepromptPlan(draft siteconfig.SiteDraft, page siteconfig.PageDraft, prompt string) generationPagePlan {
	profile := profilePrompt(prompt)
	primaryCTAHref := "/contact"
	if profile.WantsWorkshops {
		primaryCTAHref = "/workshops"
	}

	var pagePlan generationPagePlan
	switch {
	case page.Slug == "/":
		pagePlan = homePagePlan(draft.Site.Name, prompt, profile, primaryCTAHref)
	case strings.Contains(page.Slug, "contact"):
		pagePlan = contactPagePlan(draft.Site.Name, profile)
	case strings.Contains(page.Slug, "gallery"):
		pagePlan = galleryPagePlan(draft.Site.Name, profile)
	case strings.Contains(page.Slug, "about"):
		pagePlan = aboutPagePlan(draft.Site.Name, profile)
	case strings.Contains(page.Slug, "workshop"):
		pagePlan = workshopsPagePlan(draft.Site.Name, profile)
	default:
		pagePlan = servicesPagePlan(draft.Site.Name, profile)
	}
	pagePlan.Title = firstNonEmpty(pagePlan.Title, page.Title)
	pagePlan.Slug = page.Slug
	return pagePlan
}

func profilePrompt(prompt string) promptProfile {
	lower := strings.ToLower(prompt)
	profile := promptProfile{
		Category:      "business",
		CategoryLabel: "Small business website",
		ThemePreset:   siteconfig.ThemePaletteCleanLocal,
		PrimaryCTA:    "Get in touch",
		ServicesTitle: "What you can book or buy",
		ServicesIntro: "A concise overview of the main offers people should understand before they reach out.",
		FeatureItems: []map[string]any{
			{
				"title": "Clear offer",
				"body":  "Explain the core service in direct language without turning the homepage into a project.",
			},
			{
				"title": "Friendly process",
				"body":  "Set expectations, answer obvious questions, and make the next step feel easy.",
			},
			{
				"title": "Simple contact",
				"body":  "Point visitors toward one action that turns interest into an actual conversation.",
			},
		},
		AboutHeading:   "A small operation with a real point of view",
		AboutBody:      "Use this section to explain how the work feels, who it is for, and why customers trust you to do it well.",
		GalleryHeading: "Selected work",
		GalleryBody:    "Show a focused sample of recent work, finished pieces, or notable projects so visitors can picture the result.",
		ContactHeading: "Start the conversation",
		ContactBody:    "Make it easy for a visitor to ask a question, book a time, or request a quote without hunting for the next step.",
	}

	switch {
	case hasAny(lower, "photo", "photography", "portrait", "wedding photographer", "studio session"):
		profile.Category = "photography"
		profile.CategoryLabel = "Photography studio"
		profile.PrimaryCTA = "Book a session"
		profile.ServicesTitle = "Sessions and coverage"
		profile.ServicesIntro = "Group the main shoot types together so people can quickly see what you cover and what kind of experience you offer."
		profile.FeatureItems = []map[string]any{
			{"title": "Portrait sessions", "body": "Natural, low-pressure shoots for people who want honest photos without stiff direction."},
			{"title": "Brand imagery", "body": "A tidy set of images for websites, campaigns, launches, and day-to-day marketing."},
			{"title": "Events and occasions", "body": "Coverage that keeps the day moving while still catching the moments people actually remember."},
		}
		profile.AboutHeading = "A calm process makes better photographs"
		profile.AboutBody = "Use this space to describe your visual style, what a session feels like, and why clients leave with images that still feel like themselves."
		profile.GalleryHeading = "Recent shoots"
		profile.GalleryBody = "Break the work into a few focused examples: portraits, events, or brand sessions that show range without overwhelming the page."
		profile.WantsGallery = true
	case hasAny(lower, "florist", "flowers", "bouquet", "wedding flowers", "flower shop"):
		profile.Category = "florist"
		profile.CategoryLabel = "Florist studio"
		profile.PrimaryCTA = "Ask about an order"
		profile.ServicesTitle = "Seasonal flower work"
		profile.ServicesIntro = "Call out the formats that matter most: weddings, weekly flowers, custom orders, and one-off installations."
		profile.FeatureItems = []map[string]any{
			{"title": "Event flowers", "body": "Design for dinners, launches, celebrations, and weddings that need warmth rather than fuss."},
			{"title": "Weekly arrangements", "body": "Recurring flowers for shops, studios, offices, or homes that want the room to feel lived in."},
			{"title": "Custom orders", "body": "Smaller commissions, gift bouquets, and specific requests handled with a clear process."},
		}
		profile.WantsGallery = true
	case hasAny(lower, "yoga", "wellness", "massage", "therap", "coach", "counsel"):
		profile.Category = "wellness"
		profile.CategoryLabel = "Wellness practice"
		profile.PrimaryCTA = "Book a session"
		profile.ServicesTitle = "Sessions and support"
		profile.ServicesIntro = "Focus on the few ways people can work with you so the path from curiosity to booking stays short."
		profile.FeatureItems = []map[string]any{
			{"title": "Private sessions", "body": "One-to-one support tuned to a person, a goal, or a season of life."},
			{"title": "Group offerings", "body": "Classes, circles, or small-group formats that create rhythm without losing the human feel."},
			{"title": "Practical guidance", "body": "A grounded explanation of what happens, who it helps, and how to know if it is a fit."},
		}
		profile.WantsWorkshops = hasAny(lower, "class", "classes", "workshop", "course")
	case hasAny(lower, "design", "branding", "brand studio", "creative studio", "agency", "copywriter"):
		profile.Category = "creative"
		profile.CategoryLabel = "Creative studio"
		profile.PrimaryCTA = "Start a project"
		profile.ServicesTitle = "Services and retainers"
		profile.ServicesIntro = "Keep the offer structured around a few outcomes instead of a long inventory of tasks."
		profile.FeatureItems = []map[string]any{
			{"title": "Brand systems", "body": "Identity work that gives small businesses a clearer shape, voice, and visual rhythm."},
			{"title": "Launch pages", "body": "Focused pages for offers, openings, or campaigns that need to ship without drag."},
			{"title": "Ongoing support", "body": "A flexible way to keep copy, design, and site updates moving after the first launch."},
		}
		profile.WantsGallery = true
	case hasAny(lower, "textile", "yarn", "knit", "ceramic", "pottery", "craft", "maker", "atelier"):
		profile.Category = "craft"
		profile.CategoryLabel = "Craft studio"
		profile.PrimaryCTA = "See upcoming workshops"
		profile.ServicesTitle = "Pieces, commissions, and workshops"
		profile.ServicesIntro = "Explain what is available now, what can be commissioned, and how classes or workshops fit into the business."
		profile.FeatureItems = []map[string]any{
			{"title": "Small-batch pieces", "body": "Limited runs and seasonal releases with enough context to make each piece feel tangible."},
			{"title": "Custom work", "body": "Commission pathways for customers who want something made for a space, event, or gift."},
			{"title": "Hands-on workshops", "body": "Short classes that let people learn the material, process, and rhythm behind the work."},
		}
		profile.WantsGallery = true
		profile.WantsWorkshops = true
	case hasAny(lower, "bakery", "cafe", "coffee", "restaurant", "kitchen", "pastry"):
		profile.Category = "food"
		profile.CategoryLabel = "Local food business"
		profile.PrimaryCTA = "Plan a visit"
		profile.ServicesTitle = "Menu highlights and orders"
		profile.ServicesIntro = "Keep the story practical: what is available, when to stop by, and how to order for larger needs."
		profile.FeatureItems = []map[string]any{
			{"title": "Daily offering", "body": "A rotating line of fresh items that keeps regulars curious and first-time visitors oriented."},
			{"title": "Pre-orders", "body": "A clear way to reserve cakes, boxes, or larger orders without forcing a long back-and-forth."},
			{"title": "Events and catering", "body": "A simple summary of what kinds of gatherings you support and how to ask about them."},
		}
	}

	if hasAny(lower, "premium", "luxury", "editorial", "portfolio", "gallery", "showcase", "visual") {
		profile.ThemePreset = siteconfig.ThemePaletteEditorialStudio
	}
	if hasAny(lower, "dark", "dramatic", "bold", "night", "bar", "music", "event", "events", "tattoo") {
		profile.ThemePreset = siteconfig.ThemePaletteAfterHours
	}
	if hasAny(lower, "playful", "fun", "bright", "colorful", "quirky", "silly") {
		profile.ThemePreset = siteconfig.ThemePaletteBrightShopfront
	}
	if hasAny(lower, "portfolio", "gallery", "showcase", "visual") {
		profile.WantsGallery = true
	}
	if hasAny(lower, "workshop", "workshops", "class", "classes", "course", "courses") {
		profile.WantsWorkshops = true
	}
	if hasAny(lower, "price", "pricing", "package", "packages", "rates") {
		profile.WantsPricing = true
	}
	if hasAny(lower, "testimonial", "testimonials", "review", "reviews", "social proof") {
		profile.WantsTestimonials = true
	}
	if hasAny(lower, "faq", "frequently asked", "common questions", "questions") {
		profile.WantsFAQ = true
	}
	if hasAny(lower, "team", "staff", "our people", "founder", "bios", "bio") {
		profile.WantsTeam = true
	}

	return profile
}

func homePagePlan(siteName string, prompt string, profile promptProfile, primaryCTAHref string) generationPagePlan {
	description := clampSentence(prompt, 180)
	if description == "" {
		description = clampSentence(siteGoalForCategory(profile.Category), 180)
	}

	blocks := []generationBlockPlan{
		{
			Type:    "hero",
			Purpose: "Position the business in one clear screen with a single primary call to action.",
			Props: map[string]any{
				"eyebrow":     profile.CategoryLabel,
				"headline":    homeHeadline(siteName, profile),
				"subheadline": homeSubheadline(profile),
				"primaryCta": map[string]any{
					"label": profile.PrimaryCTA,
					"href":  primaryCTAHref,
				},
				"layout": "split-left",
			},
		},
		{
			Type:    "text_section",
			Purpose: "Turn the prompt into a short promise that explains how the work feels and why it matters.",
			Props: map[string]any{
				"heading":   "A first draft shaped around the actual work",
				"body":      aboutIntro(profile),
				"alignment": "left",
				"width":     "default",
			},
		},
		{
			Type:    "features_grid",
			Purpose: "List the main offers or reasons to trust the business in a scan-friendly format.",
			Props: map[string]any{
				"heading": profile.ServicesTitle,
				"intro":   profile.ServicesIntro,
				"columns": 3,
				"items":   toAnySlice(profile.FeatureItems),
			},
		},
		{
			Type:    "image_text",
			Purpose: "Reserve a visual slot and explain the experience, not just the deliverables.",
			Props: map[string]any{
				"heading":       "How the work usually feels",
				"body":          workProcessCopy(profile),
				"imagePosition": "right",
			},
		},
	}
	if profile.WantsTeam && profile.WantsGallery {
		blocks = append(blocks, teamProfileCardsBlockPlan(profile))
	}
	if profile.WantsTestimonials {
		blocks = append(blocks, testimonialsBlockPlan(profile))
	}
	if profile.WantsPricing {
		blocks = append(blocks, pricingPackagesBlockPlan(profile, primaryCTAHref))
	}
	blocks = append(blocks, generationBlockPlan{
		Type:    "cta_band",
		Purpose: "Close the homepage with the single action the business most wants from new visitors.",
		Props: map[string]any{
			"heading": profile.ContactHeading,
			"body":    profile.ContactBody,
			"cta": map[string]any{
				"label": profile.PrimaryCTA,
				"href":  primaryCTAHref,
			},
		},
	})
	blocks = append(blocks, footerBlockPlan(siteName, profile))

	return generationPagePlan{
		Title: "Home",
		Slug:  "/",
		Goal:  "Introduce the business, prove relevance quickly, and move a new visitor toward the clearest next step.",
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence(siteName, 70),
			Description: description,
		},
		Blocks: blocks,
	}
}

func servicesPagePlan(siteName string, profile promptProfile) generationPagePlan {
	blocks := []generationBlockPlan{
		{
			Type:    "text_section",
			Purpose: "Open the services page with a practical framing paragraph.",
			Props: map[string]any{
				"heading":   profile.ServicesTitle,
				"body":      profile.ServicesIntro,
				"alignment": "left",
				"width":     "wide",
			},
		},
		{
			Type:    "features_grid",
			Purpose: "Break services into scannable sections that map to real customer questions.",
			Props: map[string]any{
				"heading": "What this includes",
				"intro":   "Use these cards to outline the typical formats, deliverables, or ways someone can work with you.",
				"columns": 3,
				"items":   toAnySlice(profile.FeatureItems),
			},
		},
	}
	if profile.WantsPricing {
		blocks = append(blocks, pricingPackagesBlockPlan(profile, "/contact"))
	}
	blocks = append(blocks, generationBlockPlan{
		Type:    "cta_band",
		Purpose: "Push the reader to contact once the offer is clear enough.",
		Props: map[string]any{
			"heading": "Need a version of this tailored to your project?",
			"body":    "Invite people to ask about timing, scope, or fit instead of making them guess how to start.",
			"cta": map[string]any{
				"label": profile.PrimaryCTA,
				"href":  "/contact",
			},
		},
	})
	blocks = append(blocks, footerBlockPlan(siteName, profile))

	return generationPagePlan{
		Title: "Services",
		Slug:  "/services",
		Goal:  "Explain the main offer structure without forcing visitors to decode a long wall of copy.",
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence("Services | "+siteName, 70),
			Description: clampSentence(profile.ServicesIntro, 180),
		},
		Blocks: blocks,
	}
}

func workshopsPagePlan(siteName string, profile promptProfile) generationPagePlan {
	items := []map[string]any{
		{"title": "Intro sessions", "body": "A low-pressure first step for people who want to try the format before committing to more."},
		{"title": "Small-group workshops", "body": "Focused sessions with enough structure to teach a useful skill and enough space to keep them human."},
		{"title": "Private bookings", "body": "Custom sessions for teams, events, or people who need a more specific format."},
	}

	return generationPagePlan{
		Title: "Workshops",
		Slug:  "/workshops",
		Goal:  "Turn interest in classes or workshops into a concrete inquiry.",
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence("Workshops | "+siteName, 70),
			Description: clampSentence("Classes, small-group sessions, and private bookings.", 180),
		},
		Blocks: []generationBlockPlan{
			{
				Type:    "text_section",
				Purpose: "Explain what a workshop is for and who should join.",
				Props: map[string]any{
					"heading":   "Learn it by doing it",
					"body":      "Describe the pace, materials, and level of guidance so people can quickly tell whether the class fits them.",
					"alignment": "left",
					"width":     "default",
				},
			},
			{
				Type:    "features_grid",
				Purpose: "Show the formats someone can book without over-designing a schedule system yet.",
				Props: map[string]any{
					"heading": "Ways to join",
					"intro":   "Use these cards as the starting menu for your teaching formats.",
					"columns": 3,
					"items":   toAnySlice(items),
				},
			},
			{
				Type:    "cta_band",
				Purpose: "Send interested visitors toward the simplest way to ask about dates or availability.",
				Props: map[string]any{
					"heading": "Want the next date or a private booking?",
					"body":    "Invite the conversation before you build a more complex enrollment flow.",
					"cta": map[string]any{
						"label": "Ask about workshops",
						"href":  "/contact",
					},
				},
			},
			footerBlockPlan(siteName, profile),
		},
	}
}

func aboutPagePlan(siteName string, profile promptProfile) generationPagePlan {
	blocks := []generationBlockPlan{
		{
			Type:    "text_section",
			Purpose: "Tell the origin story without making the visitor work to find the point.",
			Props: map[string]any{
				"heading":   profile.AboutHeading,
				"body":      profile.AboutBody,
				"alignment": "left",
				"width":     "default",
			},
		},
		{
			Type:    "image_text",
			Purpose: "Pair a visual slot with a practical explanation of the process or point of view.",
			Props: map[string]any{
				"heading":       "Why people come back",
				"body":          workProcessCopy(profile),
				"imagePosition": "left",
			},
		},
	}
	if profile.WantsTeam {
		blocks = append(blocks, teamProfileCardsBlockPlan(profile))
	}
	blocks = append(blocks, generationBlockPlan{
		Type:    "cta_band",
		Purpose: "Bridge the story back to action.",
		Props: map[string]any{
			"heading": "If the approach fits, the next step should be simple",
			"body":    "Use the close of this page to invite a visit, a question, or a booking without changing tone.",
			"cta": map[string]any{
				"label": profile.PrimaryCTA,
				"href":  "/contact",
			},
		},
	})
	blocks = append(blocks, footerBlockPlan(siteName, profile))

	return generationPagePlan{
		Title: "About",
		Slug:  "/about",
		Goal:  "Add the human context that makes a small business feel trustworthy.",
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence("About | "+siteName, 70),
			Description: clampSentence(profile.AboutBody, 180),
		},
		Blocks: blocks,
	}
}

func galleryPagePlan(siteName string, profile promptProfile) generationPagePlan {
	return generationPagePlan{
		Title: "Gallery",
		Slug:  "/gallery",
		Goal:  "Provide visual proof without depending on a full image library feature yet.",
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence("Gallery | "+siteName, 70),
			Description: clampSentence(profile.GalleryBody, 180),
		},
		Blocks: []generationBlockPlan{
			{
				Type:    "text_section",
				Purpose: "Frame the gallery with a short editorial note.",
				Props: map[string]any{
					"heading":   profile.GalleryHeading,
					"body":      profile.GalleryBody,
					"alignment": "left",
					"width":     "default",
				},
			},
			{
				Type:    "gallery",
				Purpose: "Show a small, structured set of visual highlights with captions until uploads are fully wired in.",
				Props: map[string]any{
					"heading": "A small set of representative highlights",
					"intro":   "These slots can hold portfolio images, event examples, or product shots without needing a larger media system yet.",
					"layout":  "masonry",
					"images": toAnySlice([]map[string]any{
						{"title": "Signature example", "caption": "Use the first slot for the work that best captures the tone of the business."},
						{"title": "Process detail", "caption": "Add a closer view that shows texture, materials, or the way the work is made."},
						{"title": "Finished outcome", "caption": "Use the final slot to show the result in context so visitors can picture the real-world use."},
					}),
				},
			},
			{
				Type:    "cta_band",
				Purpose: "Convert visual interest into contact.",
				Props: map[string]any{
					"heading": "Seen enough to ask about your own project?",
					"body":    "This page should end with the clearest contact action, not with a dead stop.",
					"cta": map[string]any{
						"label": profile.PrimaryCTA,
						"href":  "/contact",
					},
				},
			},
			footerBlockPlan(siteName, profile),
		},
	}
}

func contactPagePlan(siteName string, profile promptProfile) generationPagePlan {
	blocks := []generationBlockPlan{
		{
			Type:    "hero",
			Purpose: "Lead with the contact promise instead of a cold form placeholder.",
			Props: map[string]any{
				"eyebrow":     "Contact",
				"headline":    profile.ContactHeading,
				"subheadline": profile.ContactBody,
				"primaryCta": map[string]any{
					"label": "Send an inquiry",
					"href":  "mailto:hello@example.com",
				},
				"layout": "centered",
			},
		},
		{
			Type:    "text_section",
			Purpose: "Add the practical details that reduce hesitation.",
			Props: map[string]any{
				"heading":   "What to include when you reach out",
				"body":      "A short note about timing, budget, scale, or the kind of help you need is enough to start a useful conversation.",
				"alignment": "left",
				"width":     "default",
			},
		},
	}
	if profile.WantsFAQ {
		blocks = append(blocks, faqBlockPlan(profile))
	}
	blocks = append(blocks,
		generationBlockPlan{
			Type:    "contact_form",
			Purpose: "Give ready visitors a real inquiry form with only the fields needed for a useful first reply.",
			Props: map[string]any{
				"heading":     "Send a quick inquiry",
				"intro":       "Use the form for timing, availability, custom requests, or the first question that gets the project moving.",
				"submitLabel": "Send inquiry",
				"fields": toAnySlice([]map[string]any{
					{"name": "name", "label": "Name", "type": "name", "required": true},
					{"name": "email", "label": "Email", "type": "email", "required": true},
					{"name": "phone", "label": "Phone", "type": "phone"},
					{"name": "message", "label": "Message", "type": "message", "required": true},
				}),
			},
		},
		generationBlockPlan{
			Type:    "cta_band",
			Purpose: "Repeat the action so the page does not end ambiguously.",
			Props: map[string]any{
				"heading": "Prefer email first?",
				"body":    "Keep a direct email path visible for people who would rather write from their own inbox.",
				"cta": map[string]any{
					"label": "Email hello@example.com",
					"href":  "mailto:hello@example.com",
				},
			},
		},
		footerBlockPlan(siteName, profile),
	)

	return generationPagePlan{
		Title: "Contact",
		Slug:  "/contact",
		Goal:  "Convert a ready visitor into an inquiry.",
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence("Contact | "+siteName, 70),
			Description: clampSentence(profile.ContactBody, 180),
		},
		Blocks: blocks,
	}
}

func testimonialsBlockPlan(profile promptProfile) generationBlockPlan {
	return generationBlockPlan{
		Type:    "testimonials",
		Purpose: "Use believable, specific testimonials to add social proof without inflating the tone.",
		Props: map[string]any{
			"heading": "What clients tend to notice",
			"intro":   "Keep the quotes short, concrete, and tied to the real experience of working together.",
			"items":   toAnySlice(testimonialItemsForProfile(profile)),
		},
	}
}

func pricingPackagesBlockPlan(profile promptProfile, href string) generationBlockPlan {
	return generationBlockPlan{
		Type:    "pricing_packages",
		Purpose: "Make the offer feel tangible with a small set of packages or starting points.",
		Props: map[string]any{
			"heading": "Packages and starting points",
			"intro":   "Use these as clear entry points so visitors can understand scale before they reach out.",
			"plans":   toAnySlice(pricingPlansForProfile(profile, href)),
		},
	}
}

func faqBlockPlan(profile promptProfile) generationBlockPlan {
	return generationBlockPlan{
		Type:    "faq",
		Purpose: "Answer the questions that most often slow down a first inquiry.",
		Props: map[string]any{
			"heading": "Questions people ask before they reach out",
			"intro":   "Use this space to lower friction, set expectations, and make the next step feel straightforward.",
			"items":   toAnySlice(faqItemsForProfile(profile)),
		},
	}
}

func teamProfileCardsBlockPlan(profile promptProfile) generationBlockPlan {
	return generationBlockPlan{
		Type:    "team_profile_cards",
		Purpose: "Introduce the people behind the work so the site feels personal and accountable.",
		Props: map[string]any{
			"heading": "People behind the work",
			"intro":   "A small team section works best when it explains what each person actually brings to the client experience.",
			"people":  toAnySlice(teamPeopleForProfile(profile)),
		},
	}
}

func footerBlockPlan(siteName string, profile promptProfile) generationBlockPlan {
	return generationBlockPlan{
		Type:    "footer",
		Purpose: "Close the page with practical navigation and a final tone-setting line.",
		Props: map[string]any{
			"showBrand":   true,
			"tagline":     footerTagline(profile),
			"contact":     map[string]any{"email": "hello@example.com"},
			"copyright":   "Copyright 2026 " + siteName,
			"socialLinks": []any{},
		},
	}
}

func pricingPlansForProfile(profile promptProfile, href string) []map[string]any {
	switch profile.Category {
	case "photography":
		return []map[string]any{
			{
				"name":        "Portrait session",
				"price":       "From $450",
				"description": "A focused session for individuals, couples, or families who want relaxed direction and a tidy final gallery.",
				"features":    toAnySlice([]map[string]any{{"text": "Planning call"}, {"text": "Edited image set"}, {"text": "Private gallery"}}),
				"cta":         map[string]any{"label": "Ask about sessions", "href": href},
			},
			{
				"name":        "Brand coverage",
				"price":       "From $900",
				"description": "A half-day or full-day package for launches, campaigns, or small-business website imagery.",
				"features":    toAnySlice([]map[string]any{{"text": "Shot list support"}, {"text": "Usage-ready edits"}, {"text": "Delivery plan"}}),
				"cta":         map[string]any{"label": "Plan your shoot", "href": href},
			},
		}
	case "wellness":
		return []map[string]any{
			{
				"name":        "Private sessions",
				"price":       "From $120",
				"description": "One-to-one work for people who want dedicated support and a calmer starting point.",
				"features":    toAnySlice([]map[string]any{{"text": "Individual pacing"}, {"text": "Practical guidance"}, {"text": "Follow-up notes"}}),
				"cta":         map[string]any{"label": "Ask about sessions", "href": href},
			},
			{
				"name":        "Small-group series",
				"price":       "From $280",
				"description": "A short series for people who want continuity, structure, and a shared rhythm.",
				"features":    toAnySlice([]map[string]any{{"text": "Group format"}, {"text": "Repeat sessions"}, {"text": "Clear next steps"}}),
				"cta":         map[string]any{"label": "Ask about dates", "href": href},
			},
		}
	default:
		return []map[string]any{
			{
				"name":        "Starter",
				"price":       "From $350",
				"description": "A smaller package for focused work where the goal and scope are already fairly clear.",
				"features":    toAnySlice([]map[string]any{{"text": "Defined scope"}, {"text": "Clear timeline"}, {"text": "Simple handoff"}}),
				"cta":         map[string]any{"label": "Ask about fit", "href": href},
			},
			{
				"name":        "Signature",
				"price":       "From $900",
				"description": "A broader package for projects that need more collaboration, shaping, or coverage.",
				"features":    toAnySlice([]map[string]any{{"text": "Collaborative planning"}, {"text": "More coverage"}, {"text": "Refined delivery"}}),
				"cta":         map[string]any{"label": "Start the conversation", "href": href},
			},
		}
	}
}

func testimonialItemsForProfile(profile promptProfile) []map[string]any {
	switch profile.Category {
	case "photography":
		return []map[string]any{
			{"quote": "The whole process felt calm and clear, and the photos still looked like us instead of a version of us trying too hard.", "name": "Mira L.", "role": "Portrait client"},
			{"quote": "We got a set of images that finally made the website feel current, consistent, and ready to send people to.", "name": "Aren Studio", "role": "Brand client"},
		}
	case "wellness":
		return []map[string]any{
			{"quote": "The sessions felt grounded from the first minute, and I left knowing exactly what I wanted to keep practicing.", "name": "Elin S.", "role": "Private client"},
			{"quote": "Everything was explained plainly, which made it much easier to book than other places I looked at.", "name": "Johan K.", "role": "New client"},
		}
	default:
		return []map[string]any{
			{"quote": "The whole thing felt surprisingly easy, and the result looked more polished than I expected this early in the process.", "name": "Nora P.", "role": "Client"},
			{"quote": "Clear communication, no fuss, and a final result that actually matched how we wanted the business to come across.", "name": "S. Berg", "role": "Owner"},
		}
	}
}

func faqItemsForProfile(profile promptProfile) []map[string]any {
	switch profile.Category {
	case "photography":
		return []map[string]any{
			{"question": "How much detail should I send in the first email?", "answer": "A short note about the kind of shoot, rough timing, and what you need the images to do is enough to start."},
			{"question": "Do I need a final brief before reaching out?", "answer": "No. A rough idea is fine, and the specifics can be shaped together once the first conversation starts."},
		}
	case "wellness":
		return []map[string]any{
			{"question": "How do I know if this is the right fit for me?", "answer": "Use the first conversation to ask practical questions about pace, format, and what support usually looks like."},
			{"question": "What should I bring to the first session?", "answer": "A short sense of what feels difficult right now and what kind of support you hope to get is usually enough."},
		}
	default:
		return []map[string]any{
			{"question": "What should someone include in the first inquiry?", "answer": "A little context about timing, scope, and the kind of help you need is enough to get a useful conversation started."},
			{"question": "Do you work with smaller projects as well?", "answer": "Use this answer to clarify whether there is a minimum scope, a starter package, or a referral path when something is too small."},
		}
	}
}

func teamPeopleForProfile(profile promptProfile) []map[string]any {
	switch profile.Category {
	case "creative":
		return []map[string]any{
			{"name": "Creative lead", "role": "Direction and framing", "bio": "Shapes the brief, keeps the work focused, and makes sure the final result still sounds like the business.", "links": toAnySlice([]map[string]any{{"label": "Start a project", "href": "/contact"}})},
			{"name": "Project partner", "role": "Planning and delivery", "bio": "Handles timing, revisions, and the practical details that keep a small project from turning into a long one.", "links": toAnySlice([]map[string]any{{"label": "Ask a question", "href": "/contact"}})},
		}
	default:
		return []map[string]any{
			{"name": "Founder", "role": "Lead point of contact", "bio": "Owns the work, keeps communication direct, and stays close to the details that shape the final experience.", "links": toAnySlice([]map[string]any{{"label": "Get in touch", "href": "/contact"}})},
		}
	}
}

func footerTagline(profile promptProfile) string {
	switch profile.Category {
	case "photography":
		return "Calm planning, honest images, and a straightforward path to booking."
	case "wellness":
		return "Grounded support, plain language, and a gentler first step."
	case "craft":
		return "Handmade work, practical beauty, and room to learn by doing."
	default:
		return "A small, focused website that keeps the next step close."
	}
}

func footerNavigationLinks(profile promptProfile) []map[string]any {
	links := []map[string]any{
		{"label": "Home", "href": "/"},
	}
	if profile.WantsWorkshops {
		links = append(links, map[string]any{"label": "Workshops", "href": "/workshops"})
	} else {
		links = append(links, map[string]any{"label": "Services", "href": "/services"})
	}
	if profile.WantsGallery {
		links = append(links, map[string]any{"label": "Gallery", "href": "/gallery"})
	} else {
		links = append(links, map[string]any{"label": "About", "href": "/about"})
	}
	links = append(links, map[string]any{"label": "Contact", "href": "/contact"})
	return links
}

func deriveSiteName(nameHint string, profile promptProfile) string {
	if value := cleanName(nameHint); value != "" {
		return value
	}
	switch profile.Category {
	case "photography":
		return "North Light Studio"
	case "florist":
		return "Field Note Florals"
	case "wellness":
		return "Quiet Room Practice"
	case "creative":
		return "Threadline Studio"
	case "craft":
		return "Ribbon & Reed Atelier"
	case "food":
		return "Morning Table"
	default:
		return "Small Good Studio"
	}
}

func cleanName(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	spacePattern := regexp.MustCompile(`\s+`)
	text = spacePattern.ReplaceAllString(text, " ")
	if len(text) > 120 {
		text = text[:120]
	}
	return text
}

func siteGoalForCategory(category string) string {
	switch category {
	case "photography":
		return "Turn visitors into photography inquiries and bookings."
	case "florist":
		return "Turn visitors into flower orders, event inquiries, and repeat customers."
	case "wellness":
		return "Turn visitors into session bookings and confident first conversations."
	case "creative":
		return "Turn visitors into well-qualified project inquiries."
	case "craft":
		return "Turn visitors into workshop bookings, commissions, and product interest."
	case "food":
		return "Turn visitors into visits, pre-orders, and catering inquiries."
	default:
		return "Turn visitors into clear, low-friction inquiries."
	}
}

func assumptionsForProfile(profile promptProfile) []string {
	assumptions := []string{
		"Default locale is English.",
		"Contact routes use placeholder email copy until forms or real contact details are added.",
	}
	if profile.WantsGallery {
		assumptions = append(assumptions, "Visual sections use placeholder image slots until asset uploads are available.")
	}
	if profile.WantsWorkshops {
		assumptions = append(assumptions, "Workshop and class information is presented as structured marketing content, not a booking system.")
	}
	return assumptions
}

func homeHeadline(siteName string, profile promptProfile) string {
	switch profile.Category {
	case "photography":
		return "Natural photography for real people, places, and moments"
	case "florist":
		return "Flowers for gatherings, gifting, and the everyday table"
	case "wellness":
		return "A steadier practice for busy bodies and loud minds"
	case "creative":
		return "Thoughtful design for small businesses that need to look ready"
	case "craft":
		return "Handmade work with real texture, practical beauty, and room to learn"
	case "food":
		return "A local spot people want to return to"
	default:
		return siteName + " needs a clear, confident website that gets to the point"
	}
}

func homeSubheadline(profile promptProfile) string {
	switch profile.Category {
	case "photography":
		return "Use this draft to introduce your style, show the work, and make booking a session feel straightforward."
	case "florist":
		return "Start with a warm overview of seasonal work, custom orders, and the events or spaces you help shape."
	case "wellness":
		return "Give new visitors a calm explanation of what you offer, who it helps, and the easiest way to begin."
	case "creative":
		return "Frame the offer around outcomes, not jargon, so good-fit clients understand the value quickly."
	case "craft":
		return "Explain the mix of products, commissions, and workshops in one place without losing the handmade feel."
	case "food":
		return "Show the atmosphere, the offering, and the practical next step before the visitor bounces for logistics."
	default:
		return "This first draft keeps the message short, the structure clear, and the next step obvious."
	}
}

func aboutIntro(profile promptProfile) string {
	switch profile.Category {
	case "photography":
		return "Lead with the kind of moments you help people keep, then describe the relaxed experience around the camera."
	case "florist":
		return "Anchor the page in seasonality, mood, and the practical kinds of orders you actually want to receive."
	case "wellness":
		return "Explain your approach plainly: what happens, how it feels, and what someone should expect after the first session."
	case "creative":
		return "Describe the way you work with small teams so the process feels structured without sounding rigid or inflated."
	case "craft":
		return "Use the page to connect the finished pieces with the materials, techniques, and teaching behind them."
	case "food":
		return "A good food site gives enough atmosphere to make a visit sound appealing without turning basic details into a scavenger hunt."
	default:
		return "Small business sites work better when the visitor can understand the offer, the style, and the next step in under a minute."
	}
}

func workProcessCopy(profile promptProfile) string {
	switch profile.Category {
	case "photography":
		return "A strong draft process usually moves from a quick inquiry to clear planning, a low-drama shoot, and a tidy handoff of the final images."
	case "florist":
		return "Keep the process readable: a short inquiry, a quick alignment on scale and mood, then a clear path to delivery or installation."
	case "wellness":
		return "The point here is clarity: who this is for, what support looks like in practice, and how people know when to reach out."
	case "creative":
		return "Good-fit projects start with scope, direction, and a clean decision-maker path, then move through concise rounds instead of endless drift."
	case "craft":
		return "People respond well when they can see the handwork and understand the rhythm behind it, whether that ends in a class, a commission, or a purchase."
	case "food":
		return "Food businesses benefit from a simple explanation of what is fresh, what changes often, and how larger or special orders are handled."
	default:
		return "Use this section to replace vague promises with a short description of how the work actually happens and what a customer gets from it."
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func clampSentence(value string, limit int) string {
	text := strings.TrimSpace(value)
	if text == "" || len(text) <= limit {
		return text
	}
	if limit <= 3 {
		return text[:limit]
	}
	return strings.TrimSpace(text[:limit-3]) + "..."
}

func toAnySlice(items []map[string]any) []any {
	values := make([]any, 0, len(items))
	for _, item := range items {
		values = append(values, item)
	}
	return values
}

func hasAny(value string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(value, part) {
			return true
		}
	}
	return false
}
