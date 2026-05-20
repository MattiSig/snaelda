package forms

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/email"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestSubmitStoresValidatedSubmissionFromPublishedSnapshot(t *testing.T) {
	store := newFakeFormStore()
	store.siteSnapshots["site-1"] = publishedContactSnapshot()
	service := NewService(store)

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
	service := NewService(newFakeFormStore())

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
	service := NewService(store)

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

func TestSubmitMarksHoneypotSubmissionsAsSpam(t *testing.T) {
	store := newFakeFormStore()
	store.siteSnapshots["site-1"] = publishedContactSnapshot()
	service := NewService(store)

	result, err := service.Submit(context.Background(), SubmitInput{
		SiteID:  "site-1",
		BlockID: "block-contact",
		Payload: map[string]any{
			"name":    "Ada Lovelace",
			"email":   "ada@example.com",
			"message": "Hello",
			"hp_url":  "http://spammer.example",
		},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if result.Submission.Status != "spam" {
		t.Fatalf("expected submission to be classified as spam, got %q", result.Submission.Status)
	}
	if result.Submission.SpamScore == nil || *result.Submission.SpamScore < 1.0 {
		t.Fatalf("expected high spam score, got %#v", result.Submission.SpamScore)
	}
	if len(store.submissions) != 1 {
		t.Fatalf("expected the submission to be stored, got %d", len(store.submissions))
	}
	if got := store.submissions[0].Payload["hp_url"]; got != nil {
		t.Fatalf("expected honeypot field to be stripped from stored payload, got %#v", got)
	}
}

func TestSubmitRecordsSpamScoreOnAcceptedSubmission(t *testing.T) {
	store := newFakeFormStore()
	store.siteSnapshots["site-1"] = publishedContactSnapshot()
	service := NewService(store)

	result, err := service.Submit(context.Background(), SubmitInput{
		SiteID:  "site-1",
		BlockID: "block-contact",
		Payload: map[string]any{
			"name":    "Ada Lovelace",
			"email":   "ada@example.com",
			"message": "Hi, just checking in about a new website.",
		},
		ClientIPHash: "ip-hash-1",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if result.Submission.Status != "new" {
		t.Fatalf("expected clean submission to remain new, got %q", result.Submission.Status)
	}
	if result.Submission.SpamScore == nil {
		t.Fatalf("expected spam score to be populated, got nil")
	}
	if store.submissions[0].clientIPHash != "ip-hash-1" {
		t.Fatalf("expected client ip hash to be persisted, got %q", store.submissions[0].clientIPHash)
	}
}

func TestSubmitRejectsInvalidPayload(t *testing.T) {
	store := newFakeFormStore()
	store.siteSnapshots["site-1"] = publishedContactSnapshot()
	service := NewService(store)

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

func TestSubmitForwardsCleanSubmissionToNotificationEmail(t *testing.T) {
	store := newFakeFormStore()
	store.siteSnapshots["site-1"] = publishedContactSnapshotWithNotification("owner@example.com")
	mailer := email.NewMemoryMailer()
	service := NewServiceWithConfig(store, ServiceConfig{
		EmailSender: email.Sender{
			Mailer:      mailer,
			DefaultFrom: email.Address{Email: "hi@snaelda.app", Name: "Snaelda"},
		},
		EmailRateLimiter: email.NewRateLimiter(store),
		Logger:           slog.New(slog.DiscardHandler),
		ProductName:      "Snaelda",
	})

	_, err := service.Submit(context.Background(), SubmitInput{
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

	if len(mailer.Messages) != 1 {
		t.Fatalf("expected one forwarded email, got %d", len(mailer.Messages))
	}
	if got := mailer.Messages[0].To[0].Email; got != "owner@example.com" {
		t.Fatalf("expected owner destination, got %q", got)
	}
	if got := mailer.Messages[0].IdempotencyKey; got != "form-submission:submission-1" {
		t.Fatalf("expected idempotency key, got %q", got)
	}
	if got := mailer.Messages[0].Tags["template"]; got != "form_submission_forwarded" {
		t.Fatalf("expected forwarded template tag, got %#v", mailer.Messages[0].Tags)
	}
}

func TestSubmitDoesNotForwardSpamSubmission(t *testing.T) {
	store := newFakeFormStore()
	store.siteSnapshots["site-1"] = publishedContactSnapshotWithNotification("owner@example.com")
	mailer := email.NewMemoryMailer()
	service := NewServiceWithConfig(store, ServiceConfig{
		EmailSender: email.Sender{
			Mailer:      mailer,
			DefaultFrom: email.Address{Email: "hi@snaelda.app", Name: "Snaelda"},
		},
		Logger:      slog.New(slog.DiscardHandler),
		ProductName: "Snaelda",
	})

	result, err := service.Submit(context.Background(), SubmitInput{
		SiteID:  "site-1",
		BlockID: "block-contact",
		Payload: map[string]any{
			"name":    "Ada Lovelace",
			"email":   "ada@example.com",
			"message": "Hello",
			"hp_url":  "http://spammer.example",
		},
	})
	if err != nil {
		t.Fatalf("submit form: %v", err)
	}
	if result.Submission.Status != "spam" {
		t.Fatalf("expected spam status, got %q", result.Submission.Status)
	}
	if len(mailer.Messages) != 0 {
		t.Fatalf("expected no forwarded email for spam, got %d", len(mailer.Messages))
	}
}

func TestSubmitKeepsWorkingWhenForwardingMailerFails(t *testing.T) {
	store := newFakeFormStore()
	store.siteSnapshots["site-1"] = publishedContactSnapshotWithNotification("owner@example.com")
	service := NewServiceWithConfig(store, ServiceConfig{
		EmailSender: email.Sender{
			Mailer:      failingMailer{err: email.ErrProviderUnavailable},
			DefaultFrom: email.Address{Email: "hi@snaelda.app", Name: "Snaelda"},
		},
		Logger:      slog.New(slog.DiscardHandler),
		ProductName: "Snaelda",
	})

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
		t.Fatal("expected submission to persist despite forwarding failure")
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
	emailAttempts []emailAttempt
	nextID        int
}

type storedSubmission struct {
	Submission
	clientIPHash string
}

type emailAttempt struct {
	addressHash string
	purpose     string
	occurredAt  time.Time
}

type failingMailer struct {
	err error
}

func (m failingMailer) Send(context.Context, email.Message) (email.SendResult, error) {
	return email.SendResult{}, m.err
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
		score, _ := args[5].(float64)
		signals := []string{}
		if signalsJSON, ok := args[6].([]byte); ok && len(signalsJSON) > 0 {
			if err := json.Unmarshal(signalsJSON, &signals); err != nil {
				return fakeFormRow{err: err}
			}
		}
		clientIPHash := ""
		if hash, ok := args[7].(string); ok {
			clientIPHash = hash
		}
		spamScore := score
		submission := storedSubmission{
			Submission: Submission{
				ID:          "submission-" + string(rune('0'+s.nextID)),
				SiteID:      args[0].(string),
				PageID:      args[1].(string),
				BlockID:     args[2].(string),
				Status:      args[4].(string),
				Payload:     payload,
				SpamScore:   &spamScore,
				SpamSignals: signals,
				CreatedAt:   time.Date(2026, 5, 12, 12, s.nextID, 0, 0, time.UTC),
			},
			clientIPHash: clientIPHash,
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
			signalsJSON, err := json.Marshal(s.submissions[index].SpamSignals)
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
				s.submissions[index].SpamScore,
				signalsJSON,
			}}
		}
		return fakeFormRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "from email_send_attempts"):
		hash := args[0].(string)
		purpose := args[1].(string)
		cutoff := args[2].(time.Time)
		attempts := 0
		for _, attempt := range s.emailAttempts {
			if attempt.addressHash == hash && attempt.purpose == purpose && attempt.occurredAt.After(cutoff) {
				attempts++
			}
		}
		return fakeFormRow{values: []any{attempts}}
	default:
		return fakeFormRow{err: errors.New("unexpected query")}
	}
}

func (s *fakeFormStore) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "insert into email_send_attempts") {
		s.emailAttempts = append(s.emailAttempts, emailAttempt{
			addressHash: args[0].(string),
			purpose:     args[1].(string),
			occurredAt:  args[2].(time.Time),
		})
	}
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
	signalsJSON, err := json.Marshal(row.SpamSignals)
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
		row.SpamScore,
		signalsJSON,
	}
	return assignScanValues(dest, values)
}

type fakeFormRow struct {
	values []any
	err    error
}

func (r fakeFormRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assignScanValues(dest, r.values)
}

func assignScanValues(dest []any, values []any) error {
	for index, value := range values {
		switch target := dest[index].(type) {
		case *string:
			*target = value.(string)
		case *[]byte:
			if value == nil {
				*target = nil
				continue
			}
			*target = value.([]byte)
		case *time.Time:
			*target = value.(time.Time)
		case *int:
			switch typed := value.(type) {
			case int:
				*target = typed
			case int64:
				*target = int(typed)
			default:
				return errors.New("unexpected int source")
			}
		case **float64:
			if value == nil {
				*target = nil
				continue
			}
			switch typed := value.(type) {
			case *float64:
				*target = typed
			case float64:
				v := typed
				*target = &v
			default:
				return errors.New("unexpected float source")
			}
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
	return publishedContactSnapshotWithNotification("")
}

func publishedContactSnapshotWithNotification(notificationEmail string) siteconfig.PublishedSnapshot {
	draft := draftWithContactForm()
	if notificationEmail != "" {
		draft.Pages[0].Blocks[0].Props["notificationEmail"] = notificationEmail
	}

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
		Brand: siteconfig.BrandConfig{
			BusinessName: "Loom & Light",
			PrimaryColor: "#86d8cf",
		},
		Theme:      siteconfig.ThemePreset(siteconfig.ThemePaletteMeanerDark),
		Navigation: siteconfig.NavigationConfig{Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page-home"}}},
		Pages:      draft.Pages,
	}
}
