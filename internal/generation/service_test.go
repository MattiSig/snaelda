package generation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
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

type fakeGenerationStore struct {
	drafts  map[string]siteconfig.SiteDraft
	jobs    map[string]fakeGenerationJob
	slugs   map[string]bool
	prompts map[string]string
	summary map[string]map[string]any
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
		drafts:  map[string]siteconfig.SiteDraft{},
		jobs:    map[string]fakeGenerationJob{},
		slugs:   map[string]bool{},
		prompts: map[string]string{},
		summary: map[string]map[string]any{},
	}
}

func (s *fakeGenerationStore) SaveDraft(_ context.Context, _ string, draft siteconfig.SiteDraft) error {
	if err := siteconfig.ValidateDraft(draft); err != nil {
		return err
	}
	s.drafts[draft.Site.ID] = draft
	s.slugs[draft.Site.Slug] = true
	return nil
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
	default:
		return fakeGenerationRow{err: pgx.ErrNoRows}
	}
}

func (s *fakeGenerationStore) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(sql, "update generation_jobs") && strings.Contains(sql, "status = 'completed'"):
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
		return pgconn.NewCommandTag("UPDATE 1"), nil
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
		default:
			return errors.New("unsupported scan target")
		}
	}
	return nil
}
