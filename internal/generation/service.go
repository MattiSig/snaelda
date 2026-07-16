package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

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
	// SiteID pre-allocates the draft's site id. When set (re-spin, which must
	// ingest brand/hero assets against the site before generation runs), the
	// draft is built under this id instead of a freshly minted one, so
	// pre-ingested assets validate against draft.Site.ID. The site row must
	// already exist (see Service.ReserveSite). Empty on the ordinary path.
	SiteID string
	// SeedAssetIDs are already-ingested asset ids (e.g. the source site's own
	// hero/work photos pulled during re-spin) that the imagery pass prefers over
	// stock photos when filling hero/gallery/image_text slots.
	SeedAssetIDs []string
	// SourceHero carries a re-spin source site's hero (Spec 07
	// optionalHints.sourceHero, populated by Spec 21). When set, the home-page
	// content stage matches its headline register and CTA intent. Nil on the
	// ordinary path.
	SourceHero *SourceHero
	// SeedCollections are collections synthesized deterministically from the
	// source's list-shaped content (re-spin services/people, Spec 21). When set,
	// buildDraftFromPlan attaches them to the draft and appends a deterministic
	// index + detail page per collection. Entries are already published and in the
	// target language (RewriteCopy), so they bypass the plan-level
	// language-conformance walker, which only sees the plan. Empty on the ordinary
	// path.
	SeedCollections []siteconfig.Collection
	// SeedCollectionInputs carry the fresh-spin intake's collections step: the
	// user's confirmed suggestions plus their raw item lines. Generation drafts
	// them into SeedCollections (drop-on-failure) before planning. Ignored when
	// SeedCollections is already populated (re-spin).
	SeedCollectionInputs []SeedCollectionInput
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
	db                    DB
	reader                draftReader
	writer                draftWriter
	planner               generationPlanBuilder
	suggester             BlockSuggester
	imageRewriter         ImageQueryRewriter
	imageQueryPlanner     StarterImageQueryPlanner
	pageChangeSetPlanner  PageChangeSetPlanner
	clarifyingPlanner     ClarifyingQuestionPlanner
	seedCollectionPlanner SeedCollectionPlanner
	decomposedPlanner     DecomposedPlanner
	imagery               *StarterImagery
	assetImporter         AssetImporter
	logger                *slog.Logger
	recorder              *audit.Recorder
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

// WithStarterImageQueryPlanner wires the batched image-query normalizer used
// by starter-imagery enrichment. When nil, enrichment searches with the
// deterministic (draft-language) query chain only.
func WithStarterImageQueryPlanner(planner StarterImageQueryPlanner) ServiceOption {
	return func(s *Service) {
		s.imageQueryPlanner = planner
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

// WithSeedCollectionPlanner wires the collections step of the intake flow.
// When nil, the interview returns no collection suggestions and generate
// ignores SeedCollectionInputs.
func WithSeedCollectionPlanner(planner SeedCollectionPlanner) ServiceOption {
	return func(s *Service) {
		s.seedCollectionPlanner = planner
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
		Prompt:            prompt,
		NameHint:          strings.TrimSpace(input.Name),
		PreferredLanguage: strings.TrimSpace(input.PreferredLanguage),
		Brand:             input.Brand,
		OptionalHints:     cloneStringMap(input.OptionalHints),
	})
	if err != nil {
		return nil, err
	}
	if len(questions) > MaxClarifyingQuestions {
		questions = questions[:MaxClarifyingQuestions]
	}
	return questions, nil
}

// InterviewResult is the full intake payload: step one's clarifying questions
// plus step two's collection suggestions.
type InterviewResult struct {
	Questions   []ClarifyingQuestion
	Collections []SeedCollectionSuggestion
}

// BuildInterview runs both intake calls in parallel. Clarifying-question
// errors fail the interview (existing behaviour); collection suggestions are
// best-effort — a failure logs and returns an empty step two, never an error.
func (s *Service) BuildInterview(ctx context.Context, input GenerateInput) (InterviewResult, error) {
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return InterviewResult{}, ErrPromptRequired
	}

	var (
		result      InterviewResult
		suggestions []SeedCollectionSuggestion
		wg          sync.WaitGroup
	)
	if s.seedCollectionPlanner != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var err error
			suggestions, err = s.seedCollectionPlanner.SuggestSeedCollections(ctx, SeedCollectionSuggestRequest{
				Prompt:            prompt,
				NameHint:          strings.TrimSpace(input.Name),
				PreferredLanguage: strings.TrimSpace(input.PreferredLanguage),
				OptionalHints:     cloneStringMap(input.OptionalHints),
			})
			if err != nil {
				suggestions = nil
				if s.logger != nil {
					s.logger.Warn("seed collection suggestions failed; interview proceeds without them", "error", err.Error())
				}
			}
		}()
	}

	questions, err := s.BuildInterviewQuestions(ctx, input)
	wg.Wait()
	if err != nil {
		return InterviewResult{}, err
	}
	result.Questions = questions
	if len(suggestions) > MaxSeedCollectionSuggestions {
		suggestions = suggestions[:MaxSeedCollectionSuggestions]
	}
	result.Collections = suggestions
	return result, nil
}

func (s *Service) GenerateWithProgress(ctx context.Context, workspaceID string, userID string, input GenerateInput, sink ProgressSink) (GenerateResult, error) {
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return GenerateResult{}, ErrPromptRequired
	}
	s.pruneGenerationJobs(ctx)

	// Fresh-spin collections step: structure the user's intake items into seed
	// collections before the job is created, so the persisted input context
	// carries the final collections. Re-spin arrives with SeedCollections
	// already synthesized and skips this.
	if len(input.SeedCollections) == 0 && len(input.SeedCollectionInputs) > 0 {
		input.SeedCollections = s.draftSeedCollections(ctx, input)
	}

	inputContext := generationInputContext{
		NameHint:           strings.TrimSpace(input.Name),
		SlugHint:           strings.TrimSpace(input.Slug),
		Prompt:             prompt,
		PreferredLanguage:  strings.TrimSpace(input.PreferredLanguage),
		OptionalHints:      cloneStringMap(input.OptionalHints),
		Brand:              input.Brand,
		InterviewAnswers:   input.InterviewAnswers,
		PreallocatedSiteID: strings.TrimSpace(input.SiteID),
		SeedAssetIDs:       input.SeedAssetIDs,
		SourceHero:         input.SourceHero,
		SeedCollections:    input.SeedCollections,
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
	if enriched, ok := s.enrichDraftWithStarterImagery(ctx, workspaceID, userID, draft, prompt, inputContext.SeedAssetIDs); ok {
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

	if enriched, ok := s.enrichDraftWithStarterImagery(ctx, workspaceID, userID, nextDraft, prompt, nil); ok {
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
	return buildGenerationPlan(input.NameHint, input.Prompt, input.PreferredLanguage), nil
}

// enrichDraftWithStarterImagery fills empty image slots in the supplied
// draft using the configured imagery provider, persists the updated draft,
// and returns the enriched draft. When imagery is not configured or the
// provider returns no usable images the draft is returned unchanged and the
// second return value is false.
func (s *Service) enrichDraftWithStarterImagery(ctx context.Context, workspaceID string, userID string, draft siteconfig.SiteDraft, prompt string, seedAssetIDs []string) (siteconfig.SiteDraft, bool) {
	stockAvailable := s.imagery.available() && s.assetImporter != nil
	if !stockAvailable && len(seedAssetIDs) == 0 {
		return draft, false
	}

	enriched := s.applyStarterImagery(ctx, workspaceID, userID, cloneDraftShallow(draft), prompt, seedAssetIDs)
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
		plan = repairGenerationPlan(plan, input.PreferredLanguage, true)
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

		if issues := detectLanguageConformanceIssues(plan, input.PreferredLanguage, verbatimExemptionFromInput(input)); len(issues) > 0 {
			if attempt < maxGenerationValidationAttempts {
				feedback.ValidationIssues = issues
				continue
			}
			if s.logger != nil {
				s.logger.Warn("language conformance issues remain after retries; proceeding with draft",
					"workspaceId", workspaceID,
					"issueCount", len(issues),
				)
			}
		}

		slugValue, err := s.createSlug(ctx, workspaceID, input.SlugHint, plan.SiteName)
		if err != nil {
			return generationPlan{}, siteconfig.SiteDraft{}, attempt - 1, err
		}

		draft, err := buildDraftFromPlan(plan, slugValue, input.PreferredLanguage, input.Brand, input.PreallocatedSiteID, input.SeedCollections)
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
	// PreallocatedSiteID pins the draft's site id on the initial-generation path
	// (re-spin). It is deliberately separate from SiteID, which the reprompt and
	// block-suggest scopes use to name an already-existing site; keeping them
	// distinct means reprompt behaviour is untouched and this field is only ever
	// consumed by buildDraftFromPlan when a fresh draft is created.
	PreallocatedSiteID string                  `json:"preallocatedSiteId,omitempty"`
	SeedAssetIDs       []string                `json:"seedAssetIds,omitempty"`
	SourceHero         *SourceHero             `json:"sourceHero,omitempty"`
	SeedCollections    []siteconfig.Collection `json:"seedCollections,omitempty"`
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
	Locale            string
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

// ReserveSite pre-allocates a bare draft site row and returns its id. It exists
// for the re-spin pipeline: brand and hero assets are ingested through
// assets.ImportExternal (which requires the site to exist and scopes the asset
// to it) *before* generation composes the draft, so the site id must be minted
// and persisted up front. Generation later reuses the same id via
// GenerateInput.SiteID, and SaveDraft's upsert fills the reserved row in.
//
// The reserved row carries a UUID placeholder slug (its own id) and
// draft_revision 0. The placeholder never collides with the real slug generation
// derives from the business name, and the revision-0 row is exactly what
// SaveDraft's conflict-update expects, so the reservation is invisible in the
// finished draft. On generation failure the caller deletes it via
// DeleteReservedSite.
func (s *Service) ReserveSite(ctx context.Context, workspaceID string, nameHint string, locale string) (string, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return "", fmt.Errorf("reserve site: workspace id is required")
	}
	siteID, err := ids.New()
	if err != nil {
		return "", fmt.Errorf("reserve site id: %w", err)
	}
	name := strings.TrimSpace(nameHint)
	if name == "" {
		name = "Untitled site"
	}
	defaultLocale := "en"
	if strings.EqualFold(strings.TrimSpace(locale), "is") {
		defaultLocale = "is"
	}
	// slug = the site id: a guaranteed-unique, format-valid placeholder that the
	// business-name slug generation will never reproduce (passed as its own text
	// parameter so it is not type-unified with the uuid id column).
	if _, err := s.db.Exec(ctx, `
		insert into sites (id, workspace_id, name, slug, status, draft_revision, default_locale, settings, brand)
		values ($1, $2, $3, $4, 'draft', 0, $5, '{}'::jsonb, '{}'::jsonb)
	`, siteID, workspaceID, name, siteID, defaultLocale); err != nil {
		return "", fmt.Errorf("reserve site row: %w", err)
	}
	return siteID, nil
}

// DeleteReservedSite removes a site reserved by ReserveSite when generation
// never populated it (draft_revision still 0). The revision guard makes the
// delete a no-op once SaveDraft has filled the draft in, so a partially
// generated site is never destroyed. Ingested assets keep their (now dangling)
// site_id set null by the FK; the import's pulled_asset_ids drive their GC.
func (s *Service) DeleteReservedSite(ctx context.Context, workspaceID string, siteID string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	siteID = strings.TrimSpace(siteID)
	if workspaceID == "" || siteID == "" {
		return nil
	}
	if _, err := s.db.Exec(ctx, `
		delete from sites
		where id = $1::uuid
		  and workspace_id = $2::uuid
		  and draft_revision = 0
	`, siteID, workspaceID); err != nil {
		return fmt.Errorf("delete reserved site: %w", err)
	}
	return nil
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

func buildGenerationPlan(nameHint string, prompt string, locale string) generationPlan {
	profile := profilePrompt(prompt, locale)
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
		SiteGoal:     siteGoalForCategory(profile.Category, profile.Locale),
		ThemePreset:  profile.ThemePreset,
		Theme:        siteconfig.ThemePreset(profile.ThemePreset),
		Pages:        pages,
		AssetsNeeded: assetsNeeded,
		Assumptions:  assumptionsForProfile(profile),
	}
}

// buildDraftFromPlan assembles a draft from a plan. When preallocatedSiteID is
// non-empty the draft is built under that id (re-spin, where brand/hero assets
// were ingested against the site before generation ran); otherwise a fresh id
// is minted.
func buildDraftFromPlan(plan generationPlan, slugValue string, preferredLanguage string, brandHint siteconfig.BrandConfig, preallocatedSiteID string, seedCollections []siteconfig.Collection) (siteconfig.SiteDraft, error) {
	siteID := strings.TrimSpace(preallocatedSiteID)
	if siteID == "" {
		var err error
		siteID, err = ids.New()
		if err != nil {
			return siteconfig.SiteDraft{}, fmt.Errorf("generate site id: %w", err)
		}
	}

	pages := make([]siteconfig.PageDraft, 0, len(plan.Pages))
	navigation := make([]siteconfig.NavigationItem, 0, len(plan.Pages))
	usedSlugs := make(map[string]bool, len(plan.Pages))
	siteDescription := clampSentence(firstNonEmpty(plan.Pages[0].SEO.Description, plan.SiteGoal), 180)

	// Collections that expose detail URLs own /{collection.Slug}; a planner
	// page on that slug would fail draft validation outright. Fold such pages
	// into the collection's index page instead of failing the whole plan.
	reservedByPageSlug := make(map[string]string, len(seedCollections))
	for _, collection := range seedCollections {
		if collection.Slug != "" {
			reservedByPageSlug["/"+collection.Slug] = collection.ID
		}
	}
	introBlocksByCollection := map[string][]siteconfig.BlockInstance{}

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
				Version: siteconfig.LatestBlockVersion(blockPlan.Type),
				Props:   blockPlan.Props,
			})
		}

		if collectionID, reserved := reservedByPageSlug[strings.TrimSpace(pagePlan.Slug)]; reserved {
			introBlocksByCollection[collectionID] = append(
				introBlocksByCollection[collectionID],
				collectionIndexIntroBlocks(blocks)...,
			)
			continue
		}

		pages = append(pages, siteconfig.PageDraft{
			ID:    pageID,
			Title: pagePlan.Title,
			Slug:  pagePlan.Slug,
			// Generated pages ship publish-ready: the site-level publish action
			// is the gate, and per-page draft status is a user's opt-out. If
			// every generated page stayed "draft", the publish snapshot would
			// filter them all out and a fresh site could never go live.
			Status: siteconfig.PageStatusPublished,
			SEO:    pagePlan.SEO,
			Blocks: blocks,
		})
		navigation = append(navigation, siteconfig.NavigationItem{
			Label:  pagePlan.Title,
			PageID: pageID,
		})
		if slug := strings.TrimSpace(pagePlan.Slug); slug != "" {
			usedSlugs[slug] = true
		}
	}

	// Attach any seeded collections (re-spin services/people) and give each one a
	// deterministic index page (its public listing) and a detail template (its
	// per-entry page). The index page is added to primary navigation; the detail
	// template is a template address, not a nav destination.
	collectionPages, collectionNav, err := buildSeedCollectionPages(seedCollections, siteDescription, usedSlugs, introBlocksByCollection)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	pages = append(pages, collectionPages...)
	navigation = append(navigation, collectionNav...)

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
		Brand:       brand,
		Theme:       plan.Theme,
		Navigation:  siteconfig.NavigationConfig{Primary: navigation},
		Pages:       pages,
		Collections: seedCollections,
	}

	if err := siteconfig.ValidateDraft(draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	return draft, nil
}

// buildSeedCollectionPages synthesizes the pages that back each seeded
// collection: a collection_index page (public listing, added to nav) and a
// collection_detail template page (per-entry rendering, not in nav). Slugs are
// derived from the collection slug and deduped against usedSlugs so they never
// collide with a planner-generated page. The index owns /{collection.slug}; any
// planner page that claimed that slug has been folded into introBlocks, which
// render above the listing.
func buildSeedCollectionPages(collections []siteconfig.Collection, siteDescription string, usedSlugs map[string]bool, introBlocks map[string][]siteconfig.BlockInstance) ([]siteconfig.PageDraft, []siteconfig.NavigationItem, error) {
	if len(collections) == 0 {
		return nil, nil, nil
	}
	pages := make([]siteconfig.PageDraft, 0, len(collections)*2)
	navigation := make([]siteconfig.NavigationItem, 0, len(collections))

	for _, collection := range collections {
		indexID, err := ids.New()
		if err != nil {
			return nil, nil, fmt.Errorf("generate collection index page id: %w", err)
		}
		indexBlockID, err := ids.New()
		if err != nil {
			return nil, nil, fmt.Errorf("generate collection index block id: %w", err)
		}
		indexSlug := uniquePageSlug("/"+collection.Slug, usedSlugs)
		blocks := append([]siteconfig.BlockInstance{}, introBlocks[collection.ID]...)
		blocks = append(blocks, siteconfig.BlockInstance{
			ID:      indexBlockID,
			Type:    "collection_index",
			Version: siteconfig.LatestBlockVersion("collection_index"),
			Props: map[string]any{
				"heading": collection.PluralLabel,
				"sort":    siteconfig.CollectionSortManual,
				"layout":  "grid",
			},
		})
		pages = append(pages, siteconfig.PageDraft{
			ID:           indexID,
			Title:        collection.PluralLabel,
			Slug:         indexSlug,
			Status:       siteconfig.PageStatusPublished,
			Type:         siteconfig.PageTypeCollectionIndex,
			CollectionID: collection.ID,
			SEO: siteconfig.SEOConfig{
				Title:       clampSentence(collection.PluralLabel, 70),
				Description: siteDescription,
			},
			Blocks: blocks,
		})
		navigation = append(navigation, siteconfig.NavigationItem{
			Label:  collection.PluralLabel,
			PageID: indexID,
		})

		detailID, err := ids.New()
		if err != nil {
			return nil, nil, fmt.Errorf("generate collection detail page id: %w", err)
		}
		detailBlockID, err := ids.New()
		if err != nil {
			return nil, nil, fmt.Errorf("generate collection detail block id: %w", err)
		}
		// The detail slug is a template address (skipped from the public URL
		// namespace); "-entry" keeps it clear of both the index and the entry URLs
		// (/{collection.slug}/{entry.slug}).
		detailSlug := uniquePageSlug("/"+collection.Slug+"-entry", usedSlugs)
		pages = append(pages, siteconfig.PageDraft{
			ID:           detailID,
			Title:        collection.SingularLabel,
			Slug:         detailSlug,
			Status:       siteconfig.PageStatusPublished,
			Type:         siteconfig.PageTypeCollectionDetail,
			CollectionID: collection.ID,
			SEO: siteconfig.SEOConfig{
				Title:       clampSentence(collection.SingularLabel, 70),
				Description: siteDescription,
			},
			Blocks: []siteconfig.BlockInstance{{
				ID:      detailBlockID,
				Type:    "collection_detail",
				Version: siteconfig.LatestBlockVersion("collection_detail"),
				Props: map[string]any{
					"layout": "default",
				},
			}},
		})
	}
	return pages, navigation, nil
}

// collectionIndexIntroBlocks filters a folded-in planner page's blocks down to
// the ones that make sense above a collection listing: rosters and list blocks
// would duplicate the collection_index itself, and a footer belongs at the end
// of a page, not mid-page.
func collectionIndexIntroBlocks(blocks []siteconfig.BlockInstance) []siteconfig.BlockInstance {
	kept := make([]siteconfig.BlockInstance, 0, len(blocks))
	for _, block := range blocks {
		switch block.Type {
		case "team_profile_cards", "collection_list", "collection_index", "footer":
			continue
		}
		kept = append(kept, block)
	}
	return kept
}

// uniquePageSlug returns base, suffixing "-2", "-3", … until it is unused, and
// records the chosen slug in used. base is a slash-prefixed page slug.
func uniquePageSlug(base string, used map[string]bool) string {
	candidate := base
	for suffix := 2; used[candidate]; suffix++ {
		candidate = fmt.Sprintf("%s-%d", base, suffix)
	}
	used[candidate] = true
	return candidate
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
			Version: siteconfig.LatestBlockVersion(blockPlan.Type),
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
		pagePlan, err := s.buildPagePlanFromLayout(ctx, draft.Site.Name, draft.Site.SEO.Description, prompt, draft.Site.DefaultLocale, draft.Brand, outlinePage, outline.Pages, nil, nil)
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
		}, false)
		if len(pagePlan.Blocks) > 0 {
			pagePlan.Title = firstNonEmpty(pagePlan.Title, page.Title)
			pagePlan.Slug = page.Slug
			return pagePlan, nil
		}
	}

	return fallback, nil
}

func fallbackPageRepromptPlan(draft siteconfig.SiteDraft, page siteconfig.PageDraft, prompt string) generationPagePlan {
	profile := profilePrompt(prompt, draft.Site.DefaultLocale)
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

// profilePrompt classifies a raw prompt into a category and the structural
// flags (gallery, workshops, pricing, and so on) that shape the fallback page
// set. Every user-visible string is resolved from the locale catalog by
// applyProfileCopy, so this function stays language-agnostic: adding a locale
// requires no changes here (Spec 22).
func profilePrompt(prompt string, locale string) promptProfile {
	lower := strings.ToLower(prompt)
	profile := promptProfile{
		Category:    categoryBusiness,
		ThemePreset: siteconfig.ThemePaletteCleanLocal,
	}

	switch {
	case hasAny(lower, "photo", "photography", "portrait", "wedding photographer", "studio session"):
		profile.Category = "photography"
		profile.WantsGallery = true
	case hasAny(lower, "florist", "flowers", "bouquet", "wedding flowers", "flower shop"):
		profile.Category = "florist"
		profile.WantsGallery = true
	case hasAny(lower, "yoga", "wellness", "massage", "therap", "coach", "counsel"):
		profile.Category = "wellness"
		profile.WantsWorkshops = hasAny(lower, "class", "classes", "workshop", "course")
	case hasAny(lower, "design", "branding", "brand studio", "creative studio", "agency", "copywriter"):
		profile.Category = "creative"
		profile.WantsGallery = true
	case hasAny(lower, "textile", "yarn", "knit", "ceramic", "pottery", "craft", "maker", "atelier"):
		profile.Category = "craft"
		profile.WantsGallery = true
		profile.WantsWorkshops = true
	case hasAny(lower, "bakery", "cafe", "coffee", "restaurant", "kitchen", "pastry"):
		profile.Category = "food"
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

	applyProfileCopy(&profile, locale)
	return profile
}

// applyProfileCopy resolves the site locale and fills every category-dependent
// copy field on the profile from the locale catalog, so the deterministic
// fallback generator never emits English into a non-English draft.
func applyProfileCopy(profile *promptProfile, locale string) {
	profile.Locale = resolveGenLocale(locale)
	pc := genCatalogFor(profile.Locale).profileCopy(profile.Category)
	profile.CategoryLabel = pc.CategoryLabel
	profile.PrimaryCTA = pc.PrimaryCTA
	profile.ServicesTitle = pc.ServicesTitle
	profile.ServicesIntro = pc.ServicesIntro
	profile.FeatureItems = featureMaps(pc.FeatureItems)
	profile.AboutHeading = pc.AboutHeading
	profile.AboutBody = pc.AboutBody
	profile.GalleryHeading = pc.GalleryHeading
	profile.GalleryBody = pc.GalleryBody
	profile.ContactHeading = pc.ContactHeading
	profile.ContactBody = pc.ContactBody
}

func homePagePlan(siteName string, prompt string, profile promptProfile, primaryCTAHref string) generationPagePlan {
	description := clampSentence(prompt, 180)
	if description == "" {
		description = clampSentence(siteGoalForCategory(profile.Category, profile.Locale), 180)
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
				"heading":   genText(profile.Locale, "home.textHeading"),
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
				"heading":       genText(profile.Locale, "home.imageHeading"),
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
		Title: genText(profile.Locale, "nav.home"),
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
				"heading": genText(profile.Locale, "services.featuresHeading"),
				"intro":   genText(profile.Locale, "services.featuresIntro"),
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
			"heading": genText(profile.Locale, "services.ctaHeading"),
			"body":    genText(profile.Locale, "services.ctaBody"),
			"cta": map[string]any{
				"label": profile.PrimaryCTA,
				"href":  "/contact",
			},
		},
	})
	blocks = append(blocks, footerBlockPlan(siteName, profile))

	return generationPagePlan{
		Title: genText(profile.Locale, "nav.services"),
		Slug:  "/services",
		Goal:  "Explain the main offer structure without forcing visitors to decode a long wall of copy.",
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence(genText(profile.Locale, "nav.services")+" | "+siteName, 70),
			Description: clampSentence(profile.ServicesIntro, 180),
		},
		Blocks: blocks,
	}
}

func workshopsPagePlan(siteName string, profile promptProfile) generationPagePlan {
	items := []map[string]any{
		{"title": genText(profile.Locale, "workshops.item1.title"), "body": genText(profile.Locale, "workshops.item1.body")},
		{"title": genText(profile.Locale, "workshops.item2.title"), "body": genText(profile.Locale, "workshops.item2.body")},
		{"title": genText(profile.Locale, "workshops.item3.title"), "body": genText(profile.Locale, "workshops.item3.body")},
	}

	return generationPagePlan{
		Title: genText(profile.Locale, "nav.workshops"),
		Slug:  "/workshops",
		Goal:  "Turn interest in classes or workshops into a concrete inquiry.",
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence(genText(profile.Locale, "nav.workshops")+" | "+siteName, 70),
			Description: clampSentence(genText(profile.Locale, "workshops.seoDescription"), 180),
		},
		Blocks: []generationBlockPlan{
			{
				Type:    "text_section",
				Purpose: "Explain what a workshop is for and who should join.",
				Props: map[string]any{
					"heading":   genText(profile.Locale, "workshops.textHeading"),
					"body":      genText(profile.Locale, "workshops.textBody"),
					"alignment": "left",
					"width":     "default",
				},
			},
			{
				Type:    "features_grid",
				Purpose: "Show the formats someone can book without over-designing a schedule system yet.",
				Props: map[string]any{
					"heading": genText(profile.Locale, "workshops.featuresHeading"),
					"intro":   genText(profile.Locale, "workshops.featuresIntro"),
					"columns": 3,
					"items":   toAnySlice(items),
				},
			},
			{
				Type:    "cta_band",
				Purpose: "Send interested visitors toward the simplest way to ask about dates or availability.",
				Props: map[string]any{
					"heading": genText(profile.Locale, "workshops.ctaHeading"),
					"body":    genText(profile.Locale, "workshops.ctaBody"),
					"cta": map[string]any{
						"label": genText(profile.Locale, "workshops.ctaLabel"),
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
				"heading":       genText(profile.Locale, "about.imageHeading"),
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
			"heading": genText(profile.Locale, "about.ctaHeading"),
			"body":    genText(profile.Locale, "about.ctaBody"),
			"cta": map[string]any{
				"label": profile.PrimaryCTA,
				"href":  "/contact",
			},
		},
	})
	blocks = append(blocks, footerBlockPlan(siteName, profile))

	return generationPagePlan{
		Title: genText(profile.Locale, "nav.about"),
		Slug:  "/about",
		Goal:  "Add the human context that makes a small business feel trustworthy.",
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence(genText(profile.Locale, "nav.about")+" | "+siteName, 70),
			Description: clampSentence(profile.AboutBody, 180),
		},
		Blocks: blocks,
	}
}

func galleryPagePlan(siteName string, profile promptProfile) generationPagePlan {
	return generationPagePlan{
		Title: genText(profile.Locale, "nav.gallery"),
		Slug:  "/gallery",
		Goal:  "Provide visual proof without depending on a full image library feature yet.",
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence(genText(profile.Locale, "nav.gallery")+" | "+siteName, 70),
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
					"heading": genText(profile.Locale, "gallery.galleryHeading"),
					"intro":   genText(profile.Locale, "gallery.galleryIntro"),
					"layout":  "masonry",
					"images": toAnySlice([]map[string]any{
						{"title": genText(profile.Locale, "gallery.image1.title"), "caption": genText(profile.Locale, "gallery.image1.caption")},
						{"title": genText(profile.Locale, "gallery.image2.title"), "caption": genText(profile.Locale, "gallery.image2.caption")},
						{"title": genText(profile.Locale, "gallery.image3.title"), "caption": genText(profile.Locale, "gallery.image3.caption")},
					}),
				},
			},
			{
				Type:    "cta_band",
				Purpose: "Convert visual interest into contact.",
				Props: map[string]any{
					"heading": genText(profile.Locale, "gallery.ctaHeading"),
					"body":    genText(profile.Locale, "gallery.ctaBody"),
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
				"eyebrow":     genText(profile.Locale, "contact.eyebrow"),
				"headline":    profile.ContactHeading,
				"subheadline": profile.ContactBody,
				"primaryCta": map[string]any{
					"label": genText(profile.Locale, "contact.heroCtaLabel"),
					"href":  "mailto:hello@example.com",
				},
				"layout": "centered",
			},
		},
		{
			Type:    "text_section",
			Purpose: "Add the practical details that reduce hesitation.",
			Props: map[string]any{
				"heading":   genText(profile.Locale, "contact.textHeading"),
				"body":      genText(profile.Locale, "contact.textBody"),
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
				"heading":     genText(profile.Locale, "contact.formHeading"),
				"intro":       genText(profile.Locale, "contact.formIntro"),
				"submitLabel": genText(profile.Locale, "contact.formSubmitLabel"),
				"fields": toAnySlice([]map[string]any{
					{"name": "name", "label": genText(profile.Locale, "contact.field.name"), "type": "name", "required": true},
					{"name": "email", "label": genText(profile.Locale, "contact.field.email"), "type": "email", "required": true},
					{"name": "phone", "label": genText(profile.Locale, "contact.field.phone"), "type": "phone"},
					{"name": "message", "label": genText(profile.Locale, "contact.field.message"), "type": "message", "required": true},
				}),
			},
		},
		generationBlockPlan{
			Type:    "cta_band",
			Purpose: "Repeat the action so the page does not end ambiguously.",
			Props: map[string]any{
				"heading": genText(profile.Locale, "contact.ctaHeading"),
				"body":    genText(profile.Locale, "contact.ctaBody"),
				"cta": map[string]any{
					"label": genText(profile.Locale, "contact.ctaLabel"),
					"href":  "mailto:hello@example.com",
				},
			},
		},
		footerBlockPlan(siteName, profile),
	)

	return generationPagePlan{
		Title: genText(profile.Locale, "nav.contact"),
		Slug:  "/contact",
		Goal:  "Convert a ready visitor into an inquiry.",
		SEO: siteconfig.SEOConfig{
			Title:       clampSentence(genText(profile.Locale, "nav.contact")+" | "+siteName, 70),
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
			"heading": genText(profile.Locale, "testimonials.heading"),
			"intro":   genText(profile.Locale, "testimonials.intro"),
			"items":   toAnySlice(testimonialItemsForProfile(profile)),
		},
	}
}

func pricingPackagesBlockPlan(profile promptProfile, href string) generationBlockPlan {
	return generationBlockPlan{
		Type:    "pricing_packages",
		Purpose: "Make the offer feel tangible with a small set of packages or starting points.",
		Props: map[string]any{
			"heading": genText(profile.Locale, "pricing.heading"),
			"intro":   genText(profile.Locale, "pricing.intro"),
			"plans":   toAnySlice(pricingPlansForProfile(profile, href)),
		},
	}
}

func faqBlockPlan(profile promptProfile) generationBlockPlan {
	return generationBlockPlan{
		Type:    "faq",
		Purpose: "Answer the questions that most often slow down a first inquiry.",
		Props: map[string]any{
			"heading": genText(profile.Locale, "faq.heading"),
			"intro":   genText(profile.Locale, "faq.intro"),
			"items":   toAnySlice(faqItemsForProfile(profile)),
		},
	}
}

func teamProfileCardsBlockPlan(profile promptProfile) generationBlockPlan {
	return generationBlockPlan{
		Type:    "team_profile_cards",
		Purpose: "Introduce the people behind the work so the site feels personal and accountable.",
		Props: map[string]any{
			"heading": genText(profile.Locale, "team.heading"),
			"intro":   genText(profile.Locale, "team.intro"),
			"people":  toAnySlice(teamPeopleForProfile(profile)),
		},
	}
}

func footerBlockPlan(siteName string, profile promptProfile) generationBlockPlan {
	return generationBlockPlan{
		Type:    "footer",
		Purpose: "Close the page with practical navigation and a final tone-setting line.",
		Props: map[string]any{
			"showBrand":    true,
			"showMadeWith": true,
			"tagline":      footerTagline(profile),
			"contact":      map[string]any{"email": "hello@example.com"},
			"copyright":    genText(profile.Locale, "footer.copyrightPrefix") + siteName,
			"socialLinks":  []any{},
		},
	}
}

func pricingPlansForProfile(profile promptProfile, href string) []map[string]any {
	return pricingMaps(categoryList(genCatalogFor(profile.Locale).Pricing, profile.Category), href)
}

func testimonialItemsForProfile(profile promptProfile) []map[string]any {
	return testimonialMaps(categoryList(genCatalogFor(profile.Locale).Testimonials, profile.Category))
}

func faqItemsForProfile(profile promptProfile) []map[string]any {
	return faqMaps(categoryList(genCatalogFor(profile.Locale).FAQ, profile.Category))
}

func teamPeopleForProfile(profile promptProfile) []map[string]any {
	return teamMaps(categoryList(genCatalogFor(profile.Locale).Team, profile.Category))
}

func footerTagline(profile promptProfile) string {
	return genCatalogFor(profile.Locale).profileCopy(profile.Category).FooterTagline
}

func footerNavigationLinks(profile promptProfile) []map[string]any {
	links := []map[string]any{
		{"label": genText(profile.Locale, "nav.home"), "href": "/"},
	}
	if profile.WantsWorkshops {
		links = append(links, map[string]any{"label": genText(profile.Locale, "nav.workshops"), "href": "/workshops"})
	} else {
		links = append(links, map[string]any{"label": genText(profile.Locale, "nav.services"), "href": "/services"})
	}
	if profile.WantsGallery {
		links = append(links, map[string]any{"label": genText(profile.Locale, "nav.gallery"), "href": "/gallery"})
	} else {
		links = append(links, map[string]any{"label": genText(profile.Locale, "nav.about"), "href": "/about"})
	}
	links = append(links, map[string]any{"label": genText(profile.Locale, "nav.contact"), "href": "/contact"})
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

func siteGoalForCategory(category string, locale string) string {
	return genCatalogFor(locale).profileCopy(category).SiteGoal
}

func assumptionsForProfile(profile promptProfile) []string {
	assumptions := []string{
		genText(profile.Locale, "assumptions.contact"),
	}
	if profile.WantsGallery {
		assumptions = append(assumptions, genText(profile.Locale, "assumptions.gallery"))
	}
	if profile.WantsWorkshops {
		assumptions = append(assumptions, genText(profile.Locale, "assumptions.workshops"))
	}
	return assumptions
}

func homeHeadline(siteName string, profile promptProfile) string {
	return homeHeadlineFor(genCatalogFor(profile.Locale).profileCopy(profile.Category), siteName)
}

func homeSubheadline(profile promptProfile) string {
	return genCatalogFor(profile.Locale).profileCopy(profile.Category).HomeSubheadline
}

func aboutIntro(profile promptProfile) string {
	return genCatalogFor(profile.Locale).profileCopy(profile.Category).AboutIntro
}

func workProcessCopy(profile promptProfile) string {
	return genCatalogFor(profile.Locale).profileCopy(profile.Category).WorkProcess
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
		return truncateOnRuneBoundary(text, limit)
	}
	return strings.TrimSpace(truncateOnRuneBoundary(text, limit-3)) + "..."
}

// truncateOnRuneBoundary cuts text to at most limit bytes without splitting a
// multi-byte rune. A mid-rune cut leaves invalid UTF-8 that json encoding
// later rewrites to the 3-byte U+FFFD, silently growing the value past the
// byte limit it was clamped to (and past validation on the next read).
func truncateOnRuneBoundary(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut]
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
