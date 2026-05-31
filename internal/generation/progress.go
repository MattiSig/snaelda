package generation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
	"github.com/jackc/pgx/v5"
)

type JobKind string

const (
	JobKindSite            JobKind = "site"
	JobKindPageReprompt    JobKind = "page_reprompt"
	JobKindSiteReprompt    JobKind = "site_reprompt"
	JobKindThemeRegenerate JobKind = "theme_regenerate"
	JobKindBlockSuggest    JobKind = "block_suggest"
)

type ProgressStep struct {
	Name  string `json:"step"`
	Label string `json:"label"`
	Index int    `json:"index"`
	Total int    `json:"total"`
}

// ProgressPartial carries an intermediate fragment of the generated draft
// (page outline or one completed page including its blocks) so the UI can
// render a shadow of the upcoming draft as it resolves. Kind drives the
// payload interpretation:
//   - "outline"      → Payload is OutlineResult; PageSlug unused.
//   - "page-content" → Payload is a single page (title, slug, blocks with
//                      full props); PageSlug identifies the page.
type ProgressPartial struct {
	Kind     string `json:"kind"`
	PageSlug string `json:"pageSlug,omitempty"`
	Payload  any    `json:"payload"`
}

// ProgressPartial kind constants.
const (
	ProgressPartialKindOutline     = "outline"
	ProgressPartialKindPageContent = "page-content"
)

type JobStatus struct {
	ID          string        `json:"id"`
	Kind        JobKind       `json:"kind"`
	State       string        `json:"state"`
	CurrentStep *ProgressStep `json:"currentStep,omitempty"`
	SiteID      string        `json:"siteId,omitempty"`
	ErrorReason string        `json:"errorReason,omitempty"`
	StartedAt   *time.Time    `json:"startedAt,omitempty"`
	CompletedAt *time.Time    `json:"completedAt,omitempty"`
}

type ProgressSink interface {
	OnJobCreated(jobID string)
	OnProgress(ProgressStep)
}

type progressSinkHandlers struct {
	onJobCreated func(string)
	onProgress   func(ProgressStep)
	onPartial    func(ProgressPartial)
}

func (h progressSinkHandlers) OnJobCreated(jobID string) {
	if h.onJobCreated != nil {
		h.onJobCreated(jobID)
	}
}

func (h progressSinkHandlers) OnProgress(step ProgressStep) {
	if h.onProgress != nil {
		h.onProgress(step)
	}
}

// emitOutline / emitPageContent implement the partialEventEmitter interface
// so the decomposed orchestrator can stream structural updates through the
// same sink that carries ProgressStep events. The methods are no-ops when
// onPartial is nil.
func (h progressSinkHandlers) emitOutline(_ context.Context, outline OutlineResult) {
	if h.onPartial == nil {
		return
	}
	h.onPartial(ProgressPartial{Kind: ProgressPartialKindOutline, Payload: outline})
}

func (h progressSinkHandlers) emitPageContent(_ context.Context, pageSlug string, page generationPagePlan) {
	if h.onPartial == nil {
		return
	}
	h.onPartial(ProgressPartial{
		Kind:     ProgressPartialKindPageContent,
		PageSlug: pageSlug,
		Payload:  page,
	})
}

type progressTracker struct {
	service *Service
	jobID   string
	kind    JobKind
	steps   []ProgressStep
	sink    ProgressSink

	mu           sync.Mutex
	highestIndex int
}

func newProgressTracker(service *Service, jobID string, kind JobKind, steps []ProgressStep, sink ProgressSink) *progressTracker {
	return &progressTracker{
		service: service,
		jobID:   strings.TrimSpace(jobID),
		kind:    kind,
		steps:   steps,
		sink:    sink,
	}
}

// emit advances the visible progress step. Calls that target a step earlier in
// the pipeline than what's already been emitted are dropped, so retry loops in
// the generation pipeline cannot rewind the UI.
func (t *progressTracker) emit(ctx context.Context, stepName string) error {
	if t == nil || t.jobID == "" || stepName == "" {
		return nil
	}
	step, ok := findProgressStep(t.steps, stepName)
	if !ok {
		return nil
	}

	t.mu.Lock()
	if step.Index <= t.highestIndex {
		t.mu.Unlock()
		return nil
	}
	t.highestIndex = step.Index
	t.mu.Unlock()

	if _, err := t.service.db.Exec(ctx, `
		update generation_jobs
		set kind = $1,
		    state = 'running',
		    status = 'running',
		    current_step = $2,
		    error_reason = null,
		    started_at = coalesce(started_at, now()),
		    completed_at = null,
		    updated_at = now()
		where id = $3::uuid
	`, t.kind, step.Name, t.jobID); err != nil {
		return fmt.Errorf("update generation job progress: %w", err)
	}
	if t.sink != nil {
		t.sink.OnProgress(step)
	}
	return nil
}

func findProgressStep(steps []ProgressStep, stepName string) (ProgressStep, bool) {
	for _, step := range steps {
		if step.Name == stepName {
			return step, true
		}
	}
	return ProgressStep{}, false
}

func ProgressStepsForKind(kind JobKind, includeAssets bool) []ProgressStep {
	names := []struct {
		name  string
		label string
	}{
		{name: "prompt.normalize", label: "Reading your prompt"},
	}
	switch kind {
	case JobKindPageReprompt:
		names = append(names,
			struct {
				name  string
				label string
			}{name: "plan.blocks", label: "Choosing blocks for each page"},
		)
	case JobKindThemeRegenerate:
		names = append(names,
			struct {
				name  string
				label string
			}{name: "plan.theme", label: "Picking colors and typography"},
		)
	case JobKindBlockSuggest:
		names = append(names,
			struct {
				name  string
				label string
			}{name: "plan.blocks", label: "Rewriting block content"},
		)
	default:
		names = append(names,
			struct {
				name  string
				label string
			}{name: "plan.pages", label: "Planning pages and structure"},
			struct {
				name  string
				label string
			}{name: "plan.theme", label: "Picking colors and typography"},
			struct {
				name  string
				label string
			}{name: "plan.blocks", label: "Choosing blocks for each page"},
		)
	}
	if includeAssets {
		names = append(names, struct {
			name  string
			label string
		}{name: "assets.fetch", label: "Finding starter imagery"})
	}
	if kind != JobKindThemeRegenerate {
		names = append(names, struct {
			name  string
			label string
		}{name: "copy.write", label: "Writing copy"})
	}
	names = append(names,
		struct {
			name  string
			label string
		}{name: "validate.repair", label: "Checking and repairing"},
		struct {
			name  string
			label string
		}{name: "persist", label: "Saving your draft"},
	)
	steps := make([]ProgressStep, 0, len(names))
	total := len(names)
	for index, item := range names {
		steps = append(steps, ProgressStep{
			Name:  item.name,
			Label: item.label,
			Index: index + 1,
			Total: total,
		})
	}
	return steps
}

func StepForJob(kind JobKind, stepName string) *ProgressStep {
	step, ok := findProgressStep(ProgressStepsForKind(kind, false), stepName)
	if !ok {
		step, ok = findProgressStep(ProgressStepsForKind(kind, true), stepName)
	}
	if !ok {
		return nil
	}
	return &step
}

func (s *Service) pruneGenerationJobs(ctx context.Context) {
	if s == nil || s.db == nil {
		return
	}
	if _, err := s.db.Exec(ctx, `
		delete from generation_jobs
		where coalesce(completed_at, started_at, created_at) < now() - interval '1 hour'
		  and state in ('succeeded', 'failed', 'canceled')
	`); err != nil && s.logger != nil {
		s.logger.Warn("prune generation jobs", "error", err.Error())
	}
}

func (s *Service) LoadJob(ctx context.Context, workspaceID string, jobID string) (JobStatus, error) {
	var job JobStatus
	var kind string
	var siteID string
	var state string
	var currentStep string
	var errorReason string
	var startedAt *time.Time
	var completedAt *time.Time
	if err := s.db.QueryRow(ctx, `
		select id::text,
		       kind,
		       coalesce(site_id::text, ''),
		       state,
		       coalesce(current_step, ''),
		       coalesce(error_reason, ''),
		       started_at,
		       completed_at
		from generation_jobs
		where id = $1::uuid
		  and workspace_id = $2::uuid
	`, jobID, workspaceID).Scan(&job.ID, &kind, &siteID, &state, &currentStep, &errorReason, &startedAt, &completedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return JobStatus{}, sites.ErrNotFound
		}
		return JobStatus{}, fmt.Errorf("load generation job: %w", err)
	}
	job.Kind = JobKind(kind)
	job.State = state
	job.SiteID = siteID
	job.ErrorReason = errorReason
	job.StartedAt = startedAt
	job.CompletedAt = completedAt
	if currentStep != "" {
		job.CurrentStep = StepForJob(job.Kind, currentStep)
	}
	return job, nil
}

func generationFailureReason(err error) string {
	switch {
	case errors.Is(err, ErrPromptRequired):
		return "prompt_required"
	case errors.Is(err, ErrPromptTooLong):
		return "prompt_too_long"
	case errors.Is(err, ErrGenerationRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrSiteSlugInvalid):
		return "invalid_slug"
	case errors.Is(err, ErrSiteSlugConflict):
		return "slug_conflict"
	default:
		var validationErr siteconfig.ValidationError
		if errors.As(err, &validationErr) {
			return "model_invalid_output"
		}
		if strings.Contains(strings.ToLower(err.Error()), "timeout") {
			return "model_timeout"
		}
		return "generation_failed"
	}
}
