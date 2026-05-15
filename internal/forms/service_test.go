package forms

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestSubmitStoresValidatedSubmissionFromPublishedSnapshot(t *testing.T) {
	store := newFakeFormStore()
	store.siteSnapshots["site-1"] = publishedContactSnapshot()
	service := Service{db: store}

	result, err := service.Submit(context.Background(), SubmitInput{
		SiteID:  "site-1",
		BlockID: "block-contact",
		Payload: map[string]any{
			"name":    "Ada Lovelace",
			"email":   "ada@example.com",
			"message": "Looking for a calmer studio site.",
		},
	})
	if err != nil {
		t.Fatalf("submit form: %v", err)
	}

	if result.Submission.ID == "" {
		t.Fatal("expected stored submission id")
	}
	if got := result.SuccessMessage; got != "Thanks. Your message is on its way." {
		t.Fatalf("expected success message, got %q", got)
	}
	if len(store.submissions) != 1 {
		t.Fatalf("expected one stored submission, got %d", len(store.submissions))
	}
	if got := store.submissions[0].Payload["email"]; got != "ada@example.com" {
		t.Fatalf("expected normalized payload to be stored, got %#v", store.submissions[0].Payload)
	}
}

func TestSubmitRejectsUnpublishedSite(t *testing.T) {
	service := Service{db: newFakeFormStore()}

	_, err := service.Submit(context.Background(), SubmitInput{
		SiteID:  "site-1",
		BlockID: "block-contact",
		Payload: map[string]any{
			"name":    "Ada Lovelace",
			"email":   "ada@example.com",
			"message": "Need a warmer brand direction.",
		},
	})
	if !errors.Is(err, ErrSiteNotPublished) {
		t.Fatalf("expected ErrSiteNotPublished, got %v", err)
	}
}

func TestSubmitRejectsBlockOnlyInDraft(t *testing.T) {
	store := newFakeFormStore()
	snapshot := publishedContactSnapshot()
	snapshot.Pages = []siteconfig.PageDraft{{
		ID:     "page-home",
		Title:  "Home",
		Slug:   "/",
		SEO:    siteconfig.SEOConfig{Title: "Loom & Light", Description: "Calm studio sites."},
		Blocks: []siteconfig.BlockInstance{},
	}}
	store.siteSnapshots["site-1"] = snapshot
	service := Service{db: store}

	_, err := service.Submit(context.Background(), SubmitInput{
		SiteID:  "site-1",
		BlockID: "block-contact",
		Payload: map[string]any{
			"name":    "Ada Lovelace",
			"email":   "ada@example.com",
			"message": "Need a warmer brand direction.",
		},
	})
	if !errors.Is(err, ErrFormBlockNotFound) {
		t.Fatalf("expected ErrFormBlockNotFound, got %v", err)
	}
}

func TestSubmitRejectsInvalidPayload(t *testing.T) {
	store := newFakeFormStore()
	store.siteSnapshots["site-1"] = publishedContactSnapshot()
	service := Service{db: store}

	_, err := service.Submit(context.Background(), SubmitInput{
		SiteID:  "site-1",
		BlockID: "block-contact",
		Payload: map[string]any{
			"name":  "Ada Lovelace",
			"email": "not-an-email",
		},
	})

	var validationErr siteconfig.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if !validationErr.Has("invalid_email") || !validationErr.Has("required") {
		t.Fatalf("expected invalid email and missing required issues, got %#v", validationErr.Issues)
	}
}

func TestListBySiteReturnsStoredSubmissions(t *testing.T) {
	store := newFakeFormStore()
	store.submissions = append(store.submissions, storedSubmission{
		Submission: Submission{
			ID:        "submission-1",
			SiteID:    "site-1",
			PageID:    "page-home",
			BlockID:   "block-contact",
			Status:    "new",
			Payload:   map[string]any{"email": "ada@example.com"},
			CreatedAt: time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC),
			PageTitle: "Home",
		},
	})
	service := Service{db: store}

	submissions, err := service.ListBySite(context.Background(), "site-1")
	if err != nil {
		t.Fatalf("list submissions: %v", err)
	}
	if len(submissions) != 1 {
		t.Fatalf("expected one submission, got %d", len(submissions))
	}
	if submissions[0].PageTitle != "Home" {
		t.Fatalf("expected page title to be returned, got %#v", submissions[0])
	}
}

func TestUpdateStatusPersistsSubmissionStatus(t *testing.T) {
	store := newFakeFormStore()
	store.submissions = append(store.submissions, storedSubmission{
		Submission: Submission{
			ID:        "submission-1",
			SiteID:    "site-1",
			PageID:    "page-home",
			BlockID:   "block-contact",
			Status:    "new",
			Payload:   map[string]any{"email": "ada@example.com"},
			CreatedAt: time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC),
			PageTitle: "Home",
		},
	})
	service := Service{db: store}
	nextStatus := "reviewed"

	submission, err := service.UpdateStatus(context.Background(), "submission-1", UpdateSubmissionInput{
		Status: &nextStatus,
	})
	if err != nil {
		t.Fatalf("update status: %v", err)
	}
	if submission.Status != "reviewed" || store.submissions[0].Status != "reviewed" {
		t.Fatalf("expected reviewed status, got %#v / %#v", submission, store.submissions[0])
	}
}

type fakeFormStore struct {
	siteSnapshots map[string]siteconfig.PublishedSnapshot
	submissions   []storedSubmission
	nextID        int
}

type storedSubmission struct {
	Submission
}

func newFakeFormStore() *fakeFormStore {
	return &fakeFormStore{
		siteSnapshots: map[string]siteconfig.PublishedSnapshot{},
		nextID:        1,
	}
}

func (s *fakeFormStore) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	if !strings.Contains(sql, "from form_submissions") {
		return nil, errors.New("unexpected query")
	}
	siteID := args[0].(string)
	rows := []storedSubmission{}
	for _, submission := range s.submissions {
		if submission.SiteID == siteID {
			rows = append(rows, submission)
		}
	}
	return &fakeFormRows{rows: rows}, nil
}

func (s *fakeFormStore) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "left join site_versions"):
		siteID := args[0].(string)
		snapshot, ok := s.siteSnapshots[siteID]
		if !ok {
			return fakeFormRow{values: []any{"", []byte("{}")}}
		}
		payload, err := json.Marshal(snapshot)
		if err != nil {
			return fakeFormRow{err: err}
		}
		return fakeFormRow{values: []any{"version-1", payload}}
	case strings.Contains(sql, "insert into form_submissions"):
		payloadJSON := args[3].([]byte)
		payload := map[string]any{}
		if err := json.Unmarshal(payloadJSON, &payload); err != nil {
			return fakeFormRow{err: err}
		}
		submission := storedSubmission{
			Submission: Submission{
				ID:        "submission-" + string(rune('0'+s.nextID)),
				SiteID:    args[0].(string),
				PageID:    args[1].(string),
				BlockID:   args[2].(string),
				Status:    args[4].(string),
				Payload:   payload,
				CreatedAt: time.Date(2026, 5, 12, 12, s.nextID, 0, 0, time.UTC),
			},
		}
		s.nextID++
		s.submissions = append(s.submissions, submission)
		return fakeFormRow{values: []any{submission.ID, submission.CreatedAt}}
	case strings.Contains(sql, "update form_submissions fs"):
		submissionID := args[0].(string)
		status := args[1].(string)
		for index := range s.submissions {
			if s.submissions[index].ID != submissionID {
				continue
			}
			s.submissions[index].Status = status
			payloadJSON, err := json.Marshal(s.submissions[index].Payload)
			if err != nil {
				return fakeFormRow{err: err}
			}
			return fakeFormRow{values: []any{
				s.submissions[index].ID,
				s.submissions[index].SiteID,
				s.submissions[index].PageID,
				s.submissions[index].BlockID,
				s.submissions[index].Status,
				payloadJSON,
				s.submissions[index].CreatedAt,
				s.submissions[index].PageTitle,
			}}
		}
		return fakeFormRow{err: pgx.ErrNoRows}
	default:
		return fakeFormRow{err: errors.New("unexpected query")}
	}
}

func (s *fakeFormStore) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (s *fakeFormStore) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return nil, errors.New("transactions are not implemented in fakeFormStore")
}

type fakeFormRows struct {
	rows  []storedSubmission
	index int
}

func (r *fakeFormRows) Close()                                       {}
func (r *fakeFormRows) Err() error                                   { return nil }
func (r *fakeFormRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeFormRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeFormRows) Conn() *pgx.Conn                              { return nil }
func (r *fakeFormRows) RawValues() [][]byte                          { return nil }
func (r *fakeFormRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeFormRows) Next() bool {
	if r.index >= len(r.rows) {
		return false
	}
	r.index++
	return true
}
func (r *fakeFormRows) Scan(dest ...any) error {
	row := r.rows[r.index-1]
	payloadJSON, err := json.Marshal(row.Payload)
	if err != nil {
		return err
	}
	values := []any{
		row.ID,
		row.SiteID,
		row.PageID,
		row.BlockID,
		row.Status,
		payloadJSON,
		row.CreatedAt,
		row.PageTitle,
	}
	for index, value := range values {
		switch target := dest[index].(type) {
		case *string:
			*target = value.(string)
		case *[]byte:
			*target = value.([]byte)
		case *time.Time:
			*target = value.(time.Time)
		}
	}
	return nil
}

type fakeFormRow struct {
	values []any
	err    error
}

func (r fakeFormRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for index, value := range r.values {
		switch target := dest[index].(type) {
		case *string:
			*target = value.(string)
		case *[]byte:
			*target = value.([]byte)
		case *time.Time:
			*target = value.(time.Time)
		default:
			return errors.New("unexpected scan destination")
		}
	}
	return nil
}

func draftWithContactForm() siteconfig.SiteDraft {
	return siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:     "site-1",
			Name:   "Loom & Light",
			Slug:   "loom-light",
			Status: "draft",
		},
		Theme:      siteconfig.ThemePreset(siteconfig.ThemePaletteMeanerDark),
		Navigation: siteconfig.NavigationConfig{Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page-home"}}},
		Pages: []siteconfig.PageDraft{{
			ID:    "page-home",
			Title: "Home",
			Slug:  "/",
			SEO: siteconfig.SEOConfig{
				Title:       "Loom & Light",
				Description: "Reach out about a new site project.",
			},
			Blocks: []siteconfig.BlockInstance{{
				ID:      "block-contact",
				Type:    "contact_form",
				Version: siteconfig.BlockVersionV1,
				Props: map[string]any{
					"heading":     "Reach out",
					"submitLabel": "Send",
					"fields": []any{
						map[string]any{"name": "name", "label": "Name", "type": "name", "required": true},
						map[string]any{"name": "email", "label": "Email", "type": "email", "required": true},
						map[string]any{"name": "message", "label": "Message", "type": "message", "required": true},
					},
				},
			}},
		}},
	}
}

func publishedContactSnapshot() siteconfig.PublishedSnapshot {
	return siteconfig.PublishedSnapshot{
		SchemaVersion: siteconfig.SiteConfigVersionV1,
		Site: siteconfig.PublishedSite{
			ID:            "site-1",
			Name:          "Loom & Light",
			DefaultLocale: "en",
			SEO: siteconfig.SEOConfig{
				Title:       "Loom & Light",
				Description: "Prompt-built sites for warm small businesses.",
			},
		},
		Theme:      siteconfig.ThemePreset(siteconfig.ThemePaletteMeanerDark),
		Navigation: siteconfig.NavigationConfig{Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page-home"}}},
		Pages:      draftWithContactForm().Pages,
	}
}
