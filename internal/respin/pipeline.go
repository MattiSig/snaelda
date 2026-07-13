package respin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/MattiSig/snaelda/internal/generation"
)

// Generator is the seam onto the Spec 07 generation engine. *generation.Service
// satisfies it; the pipeline composes the canonical input and hands off, adding
// no second generation path (Spec 21 Hard Contract).
type Generator interface {
	GenerateWithProgress(ctx context.Context, workspaceID string, userID string, input generation.GenerateInput, sink generation.ProgressSink) (generation.GenerateResult, error)
}

// SiteReserver pre-allocates the draft's site id and cleans it up on failure.
// Brand and hero assets are ingested (assets.ImportExternal, which requires and
// scopes to a site) before generation composes the draft, so the site must exist
// up front and generation must reuse its id. *generation.Service satisfies it.
type SiteReserver interface {
	ReserveSite(ctx context.Context, workspaceID string, nameHint string, locale string) (string, error)
	DeleteReservedSite(ctx context.Context, workspaceID string, siteID string) error
}

// ProgressSink receives pipeline progress so the demo UI can watch a re-spin run.
// Status carries the import state-machine transitions; Step forwards the
// underlying generation steps once composition hands off (Spec 21).
type ProgressSink interface {
	Status(status string)
	Step(step generation.ProgressStep)
}

// defaultMaxDiscoveryPages bounds same-origin discovery per the security
// contract (never off-site; small budget).
const defaultMaxDiscoveryPages = 5

// Pipeline runs the re-spin import pipeline for a single import record: fetch,
// analyze, pull brand, compose, and generate — advancing the import state
// machine and degrading gracefully at every tier (Spec 21). It holds no
// per-request state and is safe for concurrent use across imports.
type Pipeline struct {
	store     pipelineStore
	fetcher   *Fetcher
	analyzer  *Analyzer
	brand     *BrandPuller
	reserver  SiteReserver
	generator Generator
	maxPages  int
	logger    *slog.Logger
}

// PipelineConfig wires the pipeline's collaborators.
type PipelineConfig struct {
	Store     pipelineStore
	Fetcher   *Fetcher
	Analyzer  *Analyzer
	Brand     *BrandPuller
	// Reserver pre-allocates the site the brand assets ingest into. When nil (or
	// when Brand is nil) the pipeline skips the brand pull and generation mints
	// its own site id, exactly as the non-re-spin path does.
	Reserver  SiteReserver
	Generator Generator
	MaxPages  int
	Logger    *slog.Logger
}

// NewPipeline builds a Pipeline. Store, Fetcher, and Generator are required; the
// analyzer and brand puller degrade gracefully when nil (no LLM key, no asset
// service), matching the seams the earlier substrate exposes.
func NewPipeline(cfg PipelineConfig) *Pipeline {
	maxPages := cfg.MaxPages
	if maxPages <= 0 {
		maxPages = defaultMaxDiscoveryPages
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Pipeline{
		store:     cfg.Store,
		fetcher:   cfg.Fetcher,
		analyzer:  cfg.Analyzer,
		brand:     cfg.Brand,
		reserver:  cfg.Reserver,
		generator: cfg.Generator,
		maxPages:  maxPages,
		logger:    logger,
	}
}

// RunParams are the per-import inputs to a pipeline run.
type RunParams struct {
	ImportID string
	// WorkspaceID owns the generated site and ingested assets. For a public
	// demo it is the detached demo workspace; for a session-bound re-spin it is
	// the caller's workspace.
	WorkspaceID string
	// UserID attributes the generation. Empty for an unauthenticated demo.
	UserID string
	// SourceURL is the (already normalized) URL to re-spin.
	SourceURL string
	// LanguageOverride pins the target locale; empty defers to the detected
	// source language, then the Icelandic default (Spec 22).
	LanguageOverride string
}

// RunResult reports the terminal outcome of a pipeline run for the UI/handler.
type RunResult struct {
	Status        string
	SiteID        string
	JobID         string
	ShareSlug     string
	Degraded      bool
	PromptPrefill string
}

// Run executes the full pipeline. It records every state transition on the
// import record and never dead-ends: a fetch/analysis failure degrades to a
// prompt-flow handoff (marked degraded with a salvaged prefill) instead of a
// hard error. A genuine generation failure moves the import to failed. The sink
// may be nil.
func (p *Pipeline) Run(ctx context.Context, params RunParams, sink ProgressSink) (RunResult, error) {
	emit := func(status string) {
		if sink != nil {
			sink.Status(status)
		}
	}

	// Fetch.
	if _, err := p.store.UpdateStatus(ctx, params.ImportID, StatusFetching, ""); err != nil {
		return RunResult{}, err
	}
	emit(StatusFetching)
	site, err := p.fetcher.FetchSite(ctx, params.SourceURL, p.maxPages)
	if err != nil {
		reason := degradeReasonForFetch(err)
		p.logger.Info("respin fetch degraded", "importId", params.ImportID, "reason", reason, "error", err.Error())
		return p.degradeToPrompt(ctx, params, Salvage{Locale: params.LanguageOverride}, reason, sink)
	}
	if _, err := p.store.UpdateStatus(ctx, params.ImportID, StatusExtracting, site.FetchMode); err != nil {
		return RunResult{}, err
	}
	emit(StatusExtracting)

	// Analyze (classify + extract + rewrite). Degrades to prompt flow on budget
	// exhaustion or analysis failure, salvaging whatever the plain fetch read.
	content := BuildSourceContent(site.AllPages()...)
	analysis, err := p.analyzer.Analyze(ctx, content, params.LanguageOverride)
	if err != nil {
		if errors.Is(err, ErrBudgetExhausted) {
			// Budget exhaustion is a capacity signal, not a per-URL failure: do
			// not burn the import into a degraded cache entry. Surface it so the
			// endpoint can return the friendly busy response and the URL can be
			// retried once the budget resets.
			return RunResult{}, err
		}
		reason := "analysis_unavailable"
		p.logger.Warn("respin analysis degraded", "importId", params.ImportID, "error", err.Error())
		return p.degradeToPrompt(ctx, params, salvageFromContent(content, params.LanguageOverride), reason, sink)
	}

	// Reserve the draft's site up front so the brand pull can ingest the source's
	// logo and hero photos against it (assets are site-scoped) and generation can
	// reuse the same id. This runs only when both a brand puller and a reserver
	// are wired; otherwise generation mints its own id and the brand pull is
	// skipped, matching the non-re-spin path.
	var (
		brandResult     BrandResult
		reservedSiteID  string
		reservedCleanup bool
	)
	if p.brand != nil && p.reserver != nil {
		reservedSiteID, err = p.reserver.ReserveSite(ctx, params.WorkspaceID, analysis.Fields.BusinessName, analysis.TargetLocale)
		if err != nil {
			// A reservation failure must not sink the run: fall back to a brand-less
			// draft with a generation-minted site id.
			p.logger.Warn("respin reserve site failed", "importId", params.ImportID, "error", err.Error())
			reservedSiteID = ""
		} else {
			reservedCleanup = true
		}
	}

	// Brand + asset pull (best-effort; a thin brand is fine). Requires the
	// reserved site so ingested assets validate against the final draft.
	if p.brand != nil && reservedSiteID != "" {
		brandResult, err = p.brand.PullBrand(ctx, site.AllPages(), PullOptions{
			WorkspaceID:  params.WorkspaceID,
			SiteID:       reservedSiteID,
			UserID:       params.UserID,
			ImportID:     params.ImportID,
			BusinessName: analysis.Fields.BusinessName,
			SourceURL:    params.SourceURL,
			LanguageAlt:  analysis.TargetLocale,
		})
		if err != nil {
			p.logger.Warn("respin brand pull failed", "importId", params.ImportID, "error", err.Error())
			brandResult = BrandResult{}
		}
		if len(brandResult.PulledAssetIDs) > 0 {
			if _, err := p.store.SavePulledAssets(ctx, params.ImportID, brandResult.PulledAssetIDs); err != nil {
				p.logger.Warn("respin save pulled assets", "importId", params.ImportID, "error", err.Error())
			}
		}
	}

	// Persist the extraction + classification for provenance and the share page.
	extractedJSON := marshalRaw(map[string]any{
		"fields":       analysis.Fields,
		"targetLocale": analysis.TargetLocale,
		"sourceUrl":    params.SourceURL,
	})
	classificationJSON := marshalRaw(analysis.Classification)
	if _, err := p.store.SaveExtraction(ctx, params.ImportID, extractedJSON, classificationJSON); err != nil {
		p.logger.Warn("respin save extraction", "importId", params.ImportID, "error", err.Error())
	}

	// Compose the canonical Spec 07 input and generate.
	if _, err := p.store.UpdateStatus(ctx, params.ImportID, StatusComposing, ""); err != nil {
		return RunResult{}, err
	}
	emit(StatusComposing)
	comp := Compose(analysis, brandResult, ComposeContext{SourceURL: params.SourceURL, SiteID: reservedSiteID})

	genSink := generationSinkAdapter{onStep: func(step generation.ProgressStep) {
		if sink != nil {
			sink.Step(step)
		}
	}}
	result, err := p.generator.GenerateWithProgress(ctx, params.WorkspaceID, params.UserID, comp.Input, genSink)
	if err != nil {
		p.logger.Error("respin generation failed", "importId", params.ImportID, "error", err.Error())
		// Generation never populated the reserved site; remove the bare row so a
		// failed demo does not leave an empty site behind.
		if reservedCleanup {
			if derr := p.reserver.DeleteReservedSite(ctx, params.WorkspaceID, reservedSiteID); derr != nil {
				p.logger.Warn("respin delete reserved site", "importId", params.ImportID, "siteId", reservedSiteID, "error", derr.Error())
			}
		}
		_, _ = p.store.Fail(ctx, params.ImportID, marshalRaw(map[string]string{
			"stage":   "generate",
			"message": err.Error(),
		}))
		emit(StatusFailed)
		return RunResult{Status: StatusFailed}, err
	}

	// Link the generation job back to the import (publish gate + claim lookup).
	if err := p.store.LinkGenerationJob(ctx, result.JobID, params.ImportID); err != nil {
		p.logger.Warn("respin link generation job", "importId", params.ImportID, "jobId", result.JobID, "error", err.Error())
	}

	shareSlug, err := p.assignShareSlug(ctx, params.ImportID)
	if err != nil {
		p.logger.Warn("respin assign share slug", "importId", params.ImportID, "error", err.Error())
	}

	status := StatusSucceeded
	if comp.Degraded {
		if _, err := p.store.MarkDegraded(ctx, params.ImportID, comp.DegradationReason); err != nil {
			p.logger.Warn("respin mark degraded", "importId", params.ImportID, "error", err.Error())
		}
		status = StatusDegraded
	} else if _, err := p.store.UpdateStatus(ctx, params.ImportID, StatusSucceeded, ""); err != nil {
		return RunResult{}, err
	}
	emit(status)

	return RunResult{
		Status:        status,
		SiteID:        result.Draft.Site.ID,
		JobID:         result.JobID,
		ShareSlug:     shareSlug,
		Degraded:      comp.Degraded,
		PromptPrefill: comp.PromptPrefill,
	}, nil
}

// degradeToPrompt records a hard degradation: no unattended generation runs;
// instead the import is flagged degraded with a salvaged prompt prefill the demo
// UI drops into the ordinary homepage prompt flow (Spec 21). This is a
// successful terminal state, not an error.
func (p *Pipeline) degradeToPrompt(ctx context.Context, params RunParams, salvage Salvage, reason string, sink ProgressSink) (RunResult, error) {
	comp := ComposeDegraded(reason, salvage, BrandResult{}, ComposeContext{SourceURL: params.SourceURL})
	prefillJSON := marshalRaw(map[string]any{
		"promptPrefill": comp.PromptPrefill,
		"salvage":       salvage,
		"sourceUrl":     params.SourceURL,
	})
	if _, err := p.store.SaveExtraction(ctx, params.ImportID, prefillJSON, nil); err != nil {
		p.logger.Warn("respin save degrade prefill", "importId", params.ImportID, "error", err.Error())
	}
	if _, err := p.store.MarkDegraded(ctx, params.ImportID, reason); err != nil {
		return RunResult{}, err
	}
	if sink != nil {
		sink.Status(StatusDegraded)
	}
	return RunResult{
		Status:        StatusDegraded,
		Degraded:      true,
		PromptPrefill: comp.PromptPrefill,
	}, nil
}

// assignShareSlug mints a unique, URL-safe share slug for the completed demo.
func (p *Pipeline) assignShareSlug(ctx context.Context, importID string) (string, error) {
	slug, err := newShareSlug()
	if err != nil {
		return "", err
	}
	if _, err := p.store.AssignShareSlug(ctx, importID, slug); err != nil {
		return "", err
	}
	return slug, nil
}

func newShareSlug() (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

// generationSinkAdapter bridges generation progress into the pipeline's sink.
type generationSinkAdapter struct {
	onStep func(generation.ProgressStep)
}

func (a generationSinkAdapter) OnJobCreated(string) {}

func (a generationSinkAdapter) OnProgress(step generation.ProgressStep) {
	if a.onStep != nil {
		a.onStep(step)
	}
}

func degradeReasonForFetch(err error) string {
	switch {
	case errors.Is(err, ErrInsufficientContent):
		return "thin_content"
	case errors.Is(err, ErrResponseTooLarge):
		return "oversize"
	case errors.Is(err, ErrBlockedAddress), errors.Is(err, ErrDisallowedScheme), errors.Is(err, ErrDisallowedPort), errors.Is(err, ErrCredentialsInURL):
		return "blocked_url"
	case errors.Is(err, ErrTooManyRedirects):
		return "too_many_redirects"
	default:
		return "fetch_failed"
	}
}

func salvageFromContent(content SourceContent, locale string) Salvage {
	snippet := strings.TrimSpace(content.Description)
	if snippet == "" {
		snippet = strings.TrimSpace(content.Title)
	}
	loc := strings.TrimSpace(locale)
	if loc == "" {
		loc = strings.TrimSpace(content.DetectedLang)
	}
	return Salvage{
		BusinessName: strings.TrimSpace(content.Title),
		Locale:       loc,
		Snippet:      snippet,
	}
}

func marshalRaw(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return json.RawMessage(data)
}
