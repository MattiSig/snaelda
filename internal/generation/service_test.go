package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestGenerateCreatesDraftAndTracksCompletedJob(t *testing.T) {
	store := newFakeGenerationStore()
	store.slugs["north-light-studio"] = true

	service := Service{
		db:     store,
		writer: store,
	}

	result, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery, service overview, and booking CTA.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if result.JobID == "" {
		t.Fatal("expected generation job id")
	}
	if result.Draft.Site.Slug != "north-light-studio-2" {
		t.Fatalf("expected unique slug, got %q", result.Draft.Site.Slug)
	}
	if len(result.Draft.Pages) != 4 {
		t.Fatalf("expected four generated pages, got %d", len(result.Draft.Pages))
	}
	if result.Draft.Pages[2].Slug != "/gallery" {
		t.Fatalf("expected gallery page, got %#v", result.Draft.Pages[2])
	}
	if err := siteconfig.ValidateDraft(result.Draft); err != nil {
		t.Fatalf("expected valid draft, got %v", err)
	}

	job := store.jobs[result.JobID]
	if job.Status != "completed" {
		t.Fatalf("expected completed job, got %#v", job)
	}
	if job.SiteID != result.Draft.Site.ID {
		t.Fatalf("expected site id to be recorded on job, got %#v", job)
	}
	if store.prompts[result.Draft.Site.ID] == "" {
		t.Fatalf("expected prompt to be saved, got %#v", store.prompts)
	}
	if store.summary[result.Draft.Site.ID]["themePreset"] != "calm-nordic" {
		t.Fatalf("expected theme preset summary, got %#v", store.summary[result.Draft.Site.ID])
	}
}

func TestGenerateRejectsEmptyPrompt(t *testing.T) {
	service := Service{
		db:     newFakeGenerationStore(),
		writer: newFakeGenerationStore(),
	}

	_, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{})
	if !errors.Is(err, ErrPromptRequired) {
		t.Fatalf("expected prompt required error, got %v", err)
	}
}

func TestGenerateRejectsConflictingRequestedSlug(t *testing.T) {
	store := newFakeGenerationStore()
	store.slugs["quiet-room-practice"] = true

	service := Service{
		db:     store,
		writer: store,
	}

	_, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "Quiet Room Practice",
		Slug:   "quiet-room-practice",
		Prompt: "A yoga studio website with classes and bookings.",
	})
	if !errors.Is(err, ErrSiteSlugConflict) {
		t.Fatalf("expected slug conflict, got %v", err)
	}

	failedCount := 0
	for _, job := range store.jobs {
		if job.Status == "failed" {
			failedCount++
		}
	}
	if failedCount != 1 {
		t.Fatalf("expected one failed job, got %#v", store.jobs)
	}
}

func TestGenerateRetriesPlannerAfterValidationFailure(t *testing.T) {
	store := newFakeGenerationStore()
	store.saveDraftErrors = []error{
		siteconfig.ValidationError{Issues: []siteconfig.Issue{{
			Path:    "pages[0].blocks[0].props.headline",
			Code:    "required",
			Message: "headline is required",
		}}},
	}
	feedbacks := []generationPlanFeedback{}

	service := Service{
		db:     store,
		writer: store,
		planner: func(_ context.Context, input generationInputContext, feedback generationPlanFeedback) (generationPlan, error) {
			feedbacks = append(feedbacks, feedback)
			return buildGenerationPlan(input.NameHint, input.Prompt), nil
		},
	}

	result, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery, service overview, and booking CTA.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if result.Draft.Site.ID == "" {
		t.Fatalf("expected saved draft after retry, got %#v", result.Draft.Site)
	}
	if len(feedbacks) != 2 {
		t.Fatalf("expected planner to run twice, got %#v", feedbacks)
	}
	if feedbacks[0].Attempt != 1 || len(feedbacks[0].ValidationIssues) != 0 {
		t.Fatalf("expected first attempt to have no validation feedback, got %#v", feedbacks[0])
	}
	if feedbacks[1].Attempt != 2 || len(feedbacks[1].ValidationIssues) != 1 {
		t.Fatalf("expected second attempt to receive validation issues, got %#v", feedbacks[1])
	}
	if feedbacks[1].ValidationIssues[0].Code != "required" {
		t.Fatalf("expected validation issue code to be forwarded, got %#v", feedbacks[1].ValidationIssues)
	}
	if store.saveDraftCalls != 2 {
		t.Fatalf("expected draft save to retry once, got %d calls", store.saveDraftCalls)
	}
	if store.summary[result.Draft.Site.ID]["validationRetryCount"] != float64(1) {
		t.Fatalf("expected summary to track validation retry count, got %#v", store.summary[result.Draft.Site.ID])
	}
}

func TestGenerateUsesExtendedMVPBlocksWhenPromptCallsForThem(t *testing.T) {
	store := newFakeGenerationStore()

	service := Service{
		db:     store,
		writer: store,
	}

	result, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A photography studio website with a gallery, pricing, testimonials, team bios, and FAQ for new clients.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	blockTypes := map[string]bool{}
	for _, page := range result.Draft.Pages {
		for _, block := range page.Blocks {
			blockTypes[block.Type] = true
		}
	}

	for _, blockType := range []string{
		"gallery",
		"pricing_packages",
		"testimonials",
		"team_profile_cards",
		"faq",
		"contact_form",
		"footer",
	} {
		if !blockTypes[blockType] {
			t.Fatalf("expected generated draft to include %s, got %#v", blockType, blockTypes)
		}
	}
}

func TestRepromptSiteReplacesDraftAndCapturesUndoRevision(t *testing.T) {
	store := newFakeGenerationStore()
	service := Service{
		db:     store,
		reader: store,
		writer: store,
	}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	result, err := service.RepromptSite(context.Background(), "workspace-1", "user-1", initial.Draft.Site.ID, RepromptInput{
		Prompt: "Make it warmer, add pricing, and include a proper contact form.",
	})
	if err != nil {
		t.Fatalf("reprompt site: %v", err)
	}

	if result.Draft.Site.ID != initial.Draft.Site.ID {
		t.Fatalf("expected site identity to stay stable, got %#v", result.Draft.Site)
	}
	if len(store.revisions) != 1 {
		t.Fatalf("expected one captured draft revision, got %#v", store.revisions)
	}
	if store.revisions[0].Draft.Site.ID != initial.Draft.Site.ID {
		t.Fatalf("expected captured draft revision to match initial draft, got %#v", store.revisions[0])
	}
	if store.prompts[result.Draft.Site.ID] != "Make it warmer, add pricing, and include a proper contact form." {
		t.Fatalf("expected prompt metadata to update, got %#v", store.prompts)
	}
}

func TestRepromptPageReplacesOnlySelectedPage(t *testing.T) {
	store := newFakeGenerationStore()
	service := Service{
		db:     store,
		reader: store,
		writer: store,
	}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	targetPage := initial.Draft.Pages[1]
	unchangedPage := initial.Draft.Pages[0]

	result, err := service.RepromptPage(context.Background(), "workspace-1", "user-1", initial.Draft.Site.ID, targetPage.ID, RepromptInput{
		Prompt: "Turn this page into a pricing overview with clearer packages.",
	})
	if err != nil {
		t.Fatalf("reprompt page: %v", err)
	}

	if result.Draft.Pages[0].ID != unchangedPage.ID {
		t.Fatalf("expected non-target pages to stay in place, got %#v", result.Draft.Pages)
	}
	updatedPage := result.Draft.Pages[1]
	if updatedPage.ID != targetPage.ID || updatedPage.Slug != targetPage.Slug {
		t.Fatalf("expected targeted page identity to stay stable, got %#v", updatedPage)
	}
	if len(store.revisions) != 1 || store.revisions[0].PageID != targetPage.ID {
		t.Fatalf("expected page-scoped draft revision, got %#v", store.revisions)
	}
}

func TestUndoLastDraftRevisionRestoresPreviousDraft(t *testing.T) {
	store := newFakeGenerationStore()
	service := Service{
		db:     store,
		reader: store,
		writer: store,
	}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	_, err = service.RepromptSite(context.Background(), "workspace-1", "user-1", initial.Draft.Site.ID, RepromptInput{
		Prompt: "Make it darker and add pricing.",
	})
	if err != nil {
		t.Fatalf("reprompt site: %v", err)
	}

	restored, err := service.UndoLastDraftRevision(context.Background(), "workspace-1", initial.Draft.Site.ID)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}

	if restored.Site.ID != initial.Draft.Site.ID || restored.Site.Slug != initial.Draft.Site.Slug {
		t.Fatalf("expected restored draft identity to match initial draft, got %#v", restored.Site)
	}
	if len(store.revisions) != 0 {
		t.Fatalf("expected revision stack to pop after undo, got %#v", store.revisions)
	}
	if store.prompts[initial.Draft.Site.ID] != "A calm portfolio site for a photography studio that needs a gallery." {
		t.Fatalf("expected prompt metadata to restore, got %#v", store.prompts)
	}
}

func TestGenerateFailsAfterValidationRetryExhausted(t *testing.T) {
	store := newFakeGenerationStore()
	store.saveDraftErrors = []error{
		siteconfig.ValidationError{Issues: []siteconfig.Issue{{
			Path:    "pages[0].blocks[0].props.headline",
			Code:    "required",
			Message: "headline is required",
		}}},
		siteconfig.ValidationError{Issues: []siteconfig.Issue{{
			Path:    "pages[0].blocks[0].props.headline",
			Code:    "required",
			Message: "headline is required",
		}}},
	}
	planCalls := 0

	service := Service{
		db:     store,
		writer: store,
		planner: func(_ context.Context, input generationInputContext, feedback generationPlanFeedback) (generationPlan, error) {
			planCalls++
			return buildGenerationPlan(input.NameHint, input.Prompt), nil
		},
	}

	_, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery, service overview, and booking CTA.",
	})
	var validationErr siteconfig.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error after retry exhaustion, got %v", err)
	}
	if planCalls != 2 {
		t.Fatalf("expected two planner attempts, got %d", planCalls)
	}
	if store.saveDraftCalls != 2 {
		t.Fatalf("expected two draft save attempts, got %d", store.saveDraftCalls)
	}
	for _, job := range store.jobs {
		if job.Status != "failed" {
			continue
		}
		issues, ok := job.Error["issues"].([]any)
		if !ok || len(issues) != 1 {
			t.Fatalf("expected failed job to include validation issues, got %#v", job.Error)
		}
		return
	}
	t.Fatalf("expected failed generation job, got %#v", store.jobs)
}

func TestGenerateFailsWhenMetadataPersistenceFails(t *testing.T) {
	store := newFakeGenerationStore()
	store.siteUpdateErr = errors.New("write generation summary")
	service := Service{
		db:     store,
		writer: store,
	}

	_, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery.",
	})
	if err == nil || !strings.Contains(err.Error(), "write generation summary") {
		t.Fatalf("expected metadata persistence error, got %v", err)
	}
	if store.jobs["job-1"].Status != "failed" {
		t.Fatalf("expected job to be marked failed, got %#v", store.jobs["job-1"])
	}
}

func TestRepromptSiteFailsWhenJobCompletionFails(t *testing.T) {
	store := newFakeGenerationStore()
	service := Service{
		db:     store,
		reader: store,
		writer: store,
	}

	initial, err := service.Generate(context.Background(), "workspace-1", "user-1", GenerateInput{
		Name:   "North Light Studio",
		Prompt: "A calm portfolio site for a photography studio that needs a gallery.",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	store.completeJobErr = errors.New("update generation job")
	_, err = service.RepromptSite(context.Background(), "workspace-1", "user-1", initial.Draft.Site.ID, RepromptInput{
		Prompt: "Make it warmer and add pricing.",
	})
	if err == nil || !strings.Contains(err.Error(), "update generation job") {
		t.Fatalf("expected job completion error, got %v", err)
	}
	if store.jobs["job-2"].Status != "failed" {
		t.Fatalf("expected second job to be marked failed, got %#v", store.jobs["job-2"])
	}
}

func TestRepairGenerationPlanRepairsSafeIssues(t *testing.T) {
	pages := make([]generationPagePlan, 0, siteconfig.MaxPagesPerSite+2)
	for index := 0; index < siteconfig.MaxPagesPerSite+2; index++ {
		pages = append(pages, generationPagePlan{
			Title: fmt.Sprintf("Page %d", index+1),
			Slug:  "/duplicate",
			Goal:  "<script>alert(1)</script>Explain the offer clearly.",
			Blocks: []generationBlockPlan{
				{
					Type: "hero",
					Props: map[string]any{
						"headline":    fmt.Sprintf("<b>Headline %d</b>", index+1),
						"subheadline": "<p>Structured copy without raw markup.</p>",
						"primaryCta": map[string]any{
							"label": "<span>Get started</span>",
							"href":  "javascript:alert(1)",
						},
						"layout": "unknown",
					},
				},
				{
					Type: "script_embed",
					Props: map[string]any{
						"code": "<script>alert(1)</script>",
					},
				},
			},
		})
	}

	repaired := repairGenerationPlan(generationPlan{
		SiteName:    "<strong>North Light Studio</strong>",
		SiteGoal:    "<script>bad()</script>Turn visitors into confident inquiries.",
		ThemePreset: "unknown-theme",
		Theme: siteconfig.ThemeConfig{
			Version: "theme.v0",
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background": "warm",
				},
			},
		},
		Pages:        pages,
		AssetsNeeded: []string{"hero-image", "hero-image", "javascript:alert(1)", "supporting-image"},
		Assumptions: []string{
			"<p>Default locale is English.</p>",
			"<p>Default locale is English.</p>",
		},
	})

	if repaired.SiteName != "North Light Studio" {
		t.Fatalf("expected sanitized site name, got %q", repaired.SiteName)
	}
	if repaired.SiteGoal != "Turn visitors into confident inquiries." {
		t.Fatalf("expected sanitized site goal, got %q", repaired.SiteGoal)
	}
	if repaired.ThemePreset != siteconfig.ThemePaletteCalmNordic {
		t.Fatalf("expected fallback theme preset, got %q", repaired.ThemePreset)
	}
	if len(repaired.AssetsNeeded) != 2 {
		t.Fatalf("expected repaired assets list, got %#v", repaired.AssetsNeeded)
	}
	if len(repaired.Assumptions) != 1 || repaired.Assumptions[0] != "Default locale is English." {
		t.Fatalf("expected deduplicated assumptions, got %#v", repaired.Assumptions)
	}
	if len(repaired.Pages) != siteconfig.MaxPagesPerSite {
		t.Fatalf("expected repaired page count to be capped, got %d", len(repaired.Pages))
	}
	if repaired.Pages[0].Slug != "/" {
		t.Fatalf("expected homepage slug, got %q", repaired.Pages[0].Slug)
	}
	seenSlugs := map[string]bool{}
	for index, page := range repaired.Pages {
		if seenSlugs[page.Slug] {
			t.Fatalf("expected unique slugs, got duplicate %q", page.Slug)
		}
		seenSlugs[page.Slug] = true
		if strings.Contains(page.Goal, "<") {
			t.Fatalf("expected sanitized page goal, got %q", page.Goal)
		}
		if len(page.Blocks) == 0 {
			t.Fatalf("expected repaired blocks on page %d", index)
		}
		for _, block := range page.Blocks {
			if block.Type == "script_embed" {
				t.Fatalf("expected unsupported block to be removed, got %#v", block)
			}
			if strings.Contains(fmt.Sprint(block.Props), "<") {
				t.Fatalf("expected sanitized props, got %#v", block.Props)
			}
		}
	}

	draft, err := buildDraftFromPlan(repaired, "north-light-studio")
	if err != nil {
		t.Fatalf("expected repaired plan to build, got %v", err)
	}
	if err := siteconfig.ValidateDraft(draft); err != nil {
		t.Fatalf("expected repaired draft to validate, got %v", err)
	}
}

type fakeGenerationStore struct {
	drafts          map[string]siteconfig.SiteDraft
	jobs            map[string]fakeGenerationJob
	slugs           map[string]bool
	prompts         map[string]string
	summary         map[string]map[string]any
	summaryJSON     map[string][]byte
	revisions       []draftRevisionRecord
	saveDraftErrors []error
	saveDraftCalls  int
	completeJobErr  error
	siteUpdateErr   error
}

type fakeGenerationJob struct {
	ID     string
	SiteID string
	Status string
	Output generationPlan
	Error  map[string]any
}

func newFakeGenerationStore() *fakeGenerationStore {
	return &fakeGenerationStore{
		drafts:      map[string]siteconfig.SiteDraft{},
		jobs:        map[string]fakeGenerationJob{},
		slugs:       map[string]bool{},
		prompts:     map[string]string{},
		summary:     map[string]map[string]any{},
		summaryJSON: map[string][]byte{},
	}
}

func (s *fakeGenerationStore) SaveDraft(_ context.Context, _ string, draft siteconfig.SiteDraft) error {
	s.saveDraftCalls++
	if len(s.saveDraftErrors) > 0 {
		err := s.saveDraftErrors[0]
		s.saveDraftErrors = s.saveDraftErrors[1:]
		if err != nil {
			return err
		}
	}
	if err := siteconfig.ValidateDraft(draft); err != nil {
		return err
	}
	s.drafts[draft.Site.ID] = draft
	s.slugs[draft.Site.Slug] = true
	return nil
}

func (s *fakeGenerationStore) LoadDraft(_ context.Context, siteID string) (siteconfig.SiteDraft, error) {
	draft, ok := s.drafts[siteID]
	if !ok {
		return siteconfig.SiteDraft{}, sites.ErrNotFound
	}
	return draft, nil
}

func (s *fakeGenerationStore) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}

func (s *fakeGenerationStore) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "insert into generation_jobs"):
		jobID := "job-1"
		if len(s.jobs) > 0 {
			jobID = "job-2"
		}
		s.jobs[jobID] = fakeGenerationJob{ID: jobID, Status: "running"}
		return fakeGenerationRow{values: []any{jobID}}
	case strings.Contains(sql, "select exists("):
		return fakeGenerationRow{values: []any{s.slugs[args[1].(string)]}}
	case strings.Contains(sql, "select coalesce(generation_prompt, '')"):
		siteID := args[0].(string)
		if _, ok := s.drafts[siteID]; !ok {
			return fakeGenerationRow{err: pgx.ErrNoRows}
		}
		return fakeGenerationRow{values: []any{s.prompts[siteID], s.summaryJSON[siteID]}}
	case strings.Contains(sql, "from draft_revisions"):
		if len(s.revisions) == 0 {
			return fakeGenerationRow{err: pgx.ErrNoRows}
		}
		revision := s.revisions[len(s.revisions)-1]
		draftJSON, err := json.Marshal(revision.Draft)
		if err != nil {
			return fakeGenerationRow{err: err}
		}
		return fakeGenerationRow{values: []any{
			"revision-1",
			revision.Scope,
			revision.PageID,
			revision.Prompt,
			draftJSON,
			revision.GenerationPrompt,
			revision.GenerationSummaryJSON,
		}}
	default:
		return fakeGenerationRow{err: pgx.ErrNoRows}
	}
}

func (s *fakeGenerationStore) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(sql, "update generation_jobs") && strings.Contains(sql, "status = 'completed'"):
		if s.completeJobErr != nil {
			return pgconn.CommandTag{}, s.completeJobErr
		}
		job := s.jobs[args[2].(string)]
		job.SiteID = args[0].(string)
		job.Status = "completed"
		if err := json.Unmarshal(args[1].([]byte), &job.Output); err != nil {
			return pgconn.CommandTag{}, err
		}
		s.jobs[job.ID] = job
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(sql, "update generation_jobs") && strings.Contains(sql, "status = 'failed'"):
		job := s.jobs[args[1].(string)]
		job.Status = "failed"
		if err := json.Unmarshal(args[0].([]byte), &job.Error); err != nil {
			return pgconn.CommandTag{}, err
		}
		s.jobs[job.ID] = job
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(sql, "update sites"):
		if s.siteUpdateErr != nil {
			return pgconn.CommandTag{}, s.siteUpdateErr
		}
		siteID := args[2].(string)
		if _, ok := s.drafts[siteID]; !ok {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		s.prompts[siteID] = args[0].(string)
		var summary map[string]any
		if err := json.Unmarshal(args[1].([]byte), &summary); err != nil {
			return pgconn.CommandTag{}, err
		}
		s.summary[siteID] = summary
		s.summaryJSON[siteID] = append([]byte(nil), args[1].([]byte)...)
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(sql, "insert into draft_revisions"):
		draftJSON := args[5].([]byte)
		var draft siteconfig.SiteDraft
		if err := json.Unmarshal(draftJSON, &draft); err != nil {
			return pgconn.CommandTag{}, err
		}
		s.revisions = append(s.revisions, draftRevisionRecord{
			Scope:                 args[2].(string),
			PageID:                args[3].(string),
			Prompt:                args[4].(string),
			Draft:                 draft,
			GenerationPrompt:      args[6].(string),
			GenerationSummaryJSON: append([]byte(nil), args[7].([]byte)...),
		})
		return pgconn.NewCommandTag("INSERT 1"), nil
	case strings.Contains(sql, "delete from draft_revisions"):
		if len(s.revisions) > 0 {
			s.revisions = s.revisions[:len(s.revisions)-1]
		}
		return pgconn.NewCommandTag("DELETE 1"), nil
	default:
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
}

func (s *fakeGenerationStore) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return nil, errors.New("not implemented")
}

type fakeGenerationRow struct {
	values []any
	err    error
}

func (r fakeGenerationRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for index, value := range r.values {
		switch target := dest[index].(type) {
		case *string:
			*target = value.(string)
		case *bool:
			*target = value.(bool)
		case *[]byte:
			*target = append([]byte(nil), value.([]byte)...)
		default:
			return errors.New("unsupported scan target")
		}
	}
	return nil
}
