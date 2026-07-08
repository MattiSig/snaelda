package forms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/email"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrSiteRequired            = errors.New("site id is required")
	ErrBlockRequired           = errors.New("block id is required")
	ErrSiteNotFound            = errors.New("site was not found")
	ErrSiteNotPublished        = errors.New("site has no published version")
	ErrFormBlockNotFound       = errors.New("contact form block was not found")
	ErrFormBlockInvalid        = errors.New("block is not a contact form")
	ErrSubmissionNotFound      = errors.New("form submission was not found")
	ErrSubmissionStatusInvalid = errors.New("submission status is not supported")
	ErrNoSubmissionChanges     = errors.New("submission update requires a change")
)

var (
	submissionPhonePattern  = regexp.MustCompile(`^[0-9+()./\-\s]{7,30}$`)
	allowedSubmissionStatus = map[string]bool{"new": true, "reviewed": true, "resolved": true, "spam": true}
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type Service struct {
	db               DB
	emailSender      email.Sender
	emailRateLimiter *email.RateLimiter
	logger           *slog.Logger
	productName      string
}

type Submission struct {
	ID          string         `json:"id"`
	SiteID      string         `json:"siteId"`
	PageID      string         `json:"pageId,omitempty"`
	BlockID     string         `json:"blockId,omitempty"`
	Status      string         `json:"status"`
	Payload     map[string]any `json:"payload"`
	CreatedAt   time.Time      `json:"createdAt"`
	PageTitle   string         `json:"pageTitle,omitempty"`
	SpamScore   *float64       `json:"spamScore,omitempty"`
	SpamSignals []string       `json:"spamSignals,omitempty"`
}

type SubmitInput struct {
	SiteID       string
	BlockID      string
	Payload      map[string]any
	ClientIPHash string
}

type SubmitResult struct {
	Submission     Submission `json:"submission"`
	SuccessMessage string     `json:"successMessage"`
}

type UpdateSubmissionInput struct {
	Status *string
}

type resolvedForm struct {
	PageID         string
	BlockID        string
	SiteName       string
	PageTitle      string
	Definition     siteconfig.FormDefinition
	SuccessMessage string
	Locale         string
}

type ServiceConfig struct {
	EmailSender      email.Sender
	EmailRateLimiter *email.RateLimiter
	Logger           *slog.Logger
	ProductName      string
}

func NewService(db DB) *Service {
	return NewServiceWithConfig(db, ServiceConfig{})
}

func NewServiceWithConfig(db DB, cfg ServiceConfig) *Service {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		db:               db,
		emailSender:      cfg.EmailSender,
		emailRateLimiter: cfg.EmailRateLimiter,
		logger:           logger,
		productName:      firstNonEmpty(strings.TrimSpace(cfg.ProductName), "Snaelda"),
	}
}

func (s *Service) Submit(ctx context.Context, input SubmitInput) (SubmitResult, error) {
	siteID := strings.TrimSpace(input.SiteID)
	if siteID == "" {
		return SubmitResult{}, ErrSiteRequired
	}
	blockID := strings.TrimSpace(input.BlockID)
	if blockID == "" {
		return SubmitResult{}, ErrBlockRequired
	}

	form, err := s.resolveForm(ctx, siteID, blockID)
	if err != nil {
		return SubmitResult{}, err
	}

	rawPayload, honeypotFields := extractHoneypotFields(input.Payload)

	payload, err := normalizeSubmissionPayload(form.Definition, rawPayload, form.Locale)
	if err != nil {
		return SubmitResult{}, err
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("encode form payload: %w", err)
	}

	assessment := assessPayload(payload, honeypotFields)
	status := "new"
	if assessment.IsSpam() {
		status = "spam"
	}
	score := assessment.Score
	signalsJSON, err := json.Marshal(assessment.Signals)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("encode spam signals: %w", err)
	}

	submission := Submission{
		SiteID:      siteID,
		PageID:      form.PageID,
		BlockID:     blockID,
		Status:      status,
		Payload:     payload,
		SpamScore:   &score,
		SpamSignals: assessment.Signals,
	}
	clientIPHash := strings.TrimSpace(input.ClientIPHash)
	if err := s.db.QueryRow(ctx, `
		insert into form_submissions (site_id, page_id, block_id, payload, status, spam_score, spam_signals, client_ip_hash)
		values ($1::uuid, nullif($2, '')::uuid, nullif($3, '')::uuid, $4, $5, $6, $7, nullif($8, ''))
		returning id::text, created_at
	`, siteID, form.PageID, blockID, payloadJSON, submission.Status, score, signalsJSON, clientIPHash).Scan(&submission.ID, &submission.CreatedAt); err != nil {
		return SubmitResult{}, fmt.Errorf("store form submission: %w", err)
	}

	s.forwardSubmission(ctx, form, submission)

	return SubmitResult{
		Submission:     submission,
		SuccessMessage: firstNonEmpty(strings.TrimSpace(form.SuccessMessage), formCopyForLocale(form.Locale).successMessage),
	}, nil
}

func (s *Service) ListBySite(ctx context.Context, siteID string) ([]Submission, error) {
	rows, err := s.db.Query(ctx, `
		select fs.id::text,
		       fs.site_id::text,
		       coalesce(fs.page_id::text, ''),
		       coalesce(fs.block_id::text, ''),
		       fs.status,
		       fs.payload,
		       fs.created_at,
		       coalesce(p.title, ''),
		       fs.spam_score,
		       coalesce(fs.spam_signals, '[]'::jsonb)
		from form_submissions fs
		left join pages p on p.id = fs.page_id
		where fs.site_id = $1::uuid
		order by fs.created_at desc, fs.id desc
	`, strings.TrimSpace(siteID))
	if err != nil {
		return nil, fmt.Errorf("list form submissions: %w", err)
	}
	defer rows.Close()

	submissions := []Submission{}
	for rows.Next() {
		submission, err := scanSubmission(rows)
		if err != nil {
			return nil, err
		}
		submissions = append(submissions, submission)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate form submissions: %w", err)
	}
	return submissions, nil
}

func (s *Service) UpdateStatus(ctx context.Context, submissionID string, input UpdateSubmissionInput) (Submission, error) {
	if input.Status == nil {
		return Submission{}, ErrNoSubmissionChanges
	}

	status := strings.ToLower(strings.TrimSpace(*input.Status))
	if !allowedSubmissionStatus[status] {
		return Submission{}, ErrSubmissionStatusInvalid
	}

	submission, err := scanSubmissionRow(s.db.QueryRow(ctx, `
		update form_submissions fs
		set status = $2
		where fs.id = $1::uuid
		returning fs.id::text,
		          fs.site_id::text,
		          coalesce(fs.page_id::text, ''),
		          coalesce(fs.block_id::text, ''),
		          fs.status,
		          fs.payload,
		          fs.created_at,
		          coalesce((select title from pages where id = fs.page_id), ''),
		          fs.spam_score,
		          coalesce(fs.spam_signals, '[]'::jsonb)
	`, strings.TrimSpace(submissionID), status))
	if errors.Is(err, pgx.ErrNoRows) {
		return Submission{}, ErrSubmissionNotFound
	}
	if err != nil {
		return Submission{}, fmt.Errorf("update form submission: %w", err)
	}
	return submission, nil
}

func (s *Service) resolveForm(ctx context.Context, siteID string, blockID string) (resolvedForm, error) {
	snapshot, ok, err := s.loadPublishedSnapshot(ctx, siteID)
	if err != nil {
		return resolvedForm{}, err
	}
	if !ok {
		return resolvedForm{}, ErrSiteNotPublished
	}

	form, found, err := findFormInPages(snapshot.Site.Name, snapshot.Site.DefaultLocale, snapshot.Pages, blockID)
	if err != nil {
		return resolvedForm{}, err
	}
	if !found {
		return resolvedForm{}, ErrFormBlockNotFound
	}
	return form, nil
}

func (s *Service) loadPublishedSnapshot(ctx context.Context, siteID string) (siteconfig.PublishedSnapshot, bool, error) {
	var versionID string
	var snapshotJSON []byte
	err := s.db.QueryRow(ctx, `
		select coalesce(s.published_version_id::text, ''),
		       coalesce(sv.snapshot, '{}'::jsonb)
		from sites s
		left join site_versions sv on sv.id = s.published_version_id
		where s.id = $1::uuid
	`, siteID).Scan(&versionID, &snapshotJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return siteconfig.PublishedSnapshot{}, false, ErrSiteNotFound
	}
	if err != nil {
		return siteconfig.PublishedSnapshot{}, false, fmt.Errorf("load published snapshot metadata: %w", err)
	}
	if versionID == "" {
		return siteconfig.PublishedSnapshot{}, false, nil
	}

	var snapshot siteconfig.PublishedSnapshot
	if err := json.Unmarshal(snapshotJSON, &snapshot); err != nil {
		return siteconfig.PublishedSnapshot{}, false, fmt.Errorf("decode published snapshot: %w", err)
	}
	if err := siteconfig.ValidatePublishedSnapshot(snapshot); err != nil {
		return siteconfig.PublishedSnapshot{}, false, fmt.Errorf("published snapshot is invalid: %w", err)
	}

	return snapshot, true, nil
}

func findFormInPages(siteName string, locale string, pages []siteconfig.PageDraft, blockID string) (resolvedForm, bool, error) {
	for _, page := range pages {
		for _, block := range page.Blocks {
			if block.ID != blockID {
				continue
			}
			if block.Type != "contact_form" {
				return resolvedForm{}, false, ErrFormBlockInvalid
			}
			definition, err := siteconfig.FormDefinitionFromProps(block.Props)
			if err != nil {
				return resolvedForm{}, false, err
			}
			successMessage, _ := block.Props["successMessage"].(string)
			return resolvedForm{
				PageID:         page.ID,
				BlockID:        block.ID,
				SiteName:       siteName,
				PageTitle:      page.Title,
				Definition:     definition,
				SuccessMessage: successMessage,
				Locale:         locale,
			}, true, nil
		}
	}
	return resolvedForm{}, false, nil
}

func normalizeSubmissionPayload(definition siteconfig.FormDefinition, payload map[string]any, locale string) (map[string]any, error) {
	if payload == nil {
		payload = map[string]any{}
	}

	msgs := formCopyForLocale(locale)
	issues := []siteconfig.Issue{}
	normalized := map[string]any{}
	knownFields := map[string]siteconfig.FormField{}

	for _, field := range definition.Fields {
		knownFields[field.Name] = field
		raw, exists := payload[field.Name]
		if !exists {
			if field.Required {
				issues = append(issues, siteconfig.Issue{
					Path:    "payload." + field.Name,
					Code:    "required",
					Message: msgs.required(field.Label),
				})
			}
			continue
		}

		text, ok := raw.(string)
		if !ok {
			issues = append(issues, siteconfig.Issue{
				Path:    "payload." + field.Name,
				Code:    "invalid_type",
				Message: msgs.invalidType(field.Label),
			})
			continue
		}

		trimmed := strings.TrimSpace(text)
		switch field.Type {
		case "name":
			if len(trimmed) < 1 || len(trimmed) > 120 {
				issues = append(issues, siteconfig.Issue{
					Path:    "payload." + field.Name,
					Code:    "invalid_length",
					Message: msgs.lengthRange(field.Label, 1, 120),
				})
				continue
			}
		case "email":
			if trimmed == "" && !field.Required {
				continue
			}
			if _, err := mail.ParseAddress(trimmed); err != nil {
				issues = append(issues, siteconfig.Issue{
					Path:    "payload." + field.Name,
					Code:    "invalid_email",
					Message: msgs.invalidEmail(field.Label),
				})
				continue
			}
		case "phone":
			if trimmed == "" && !field.Required {
				continue
			}
			if !submissionPhonePattern.MatchString(trimmed) {
				issues = append(issues, siteconfig.Issue{
					Path:    "payload." + field.Name,
					Code:    "invalid_phone",
					Message: msgs.invalidPhone(field.Label),
				})
				continue
			}
		case "message":
			if len(trimmed) < 1 || len(trimmed) > 4000 {
				issues = append(issues, siteconfig.Issue{
					Path:    "payload." + field.Name,
					Code:    "invalid_length",
					Message: msgs.lengthRange(field.Label, 1, 4000),
				})
				continue
			}
		case "select":
			if trimmed == "" && !field.Required {
				continue
			}
			if !contains(field.Options, trimmed) {
				issues = append(issues, siteconfig.Issue{
					Path:    "payload." + field.Name,
					Code:    "invalid_option",
					Message: msgs.invalidOption(field.Label),
				})
				continue
			}
		default:
			issues = append(issues, siteconfig.Issue{
				Path:    "payload." + field.Name,
				Code:    "unsupported_field",
				Message: msgs.unsupported(field.Label),
			})
			continue
		}

		if trimmed == "" && field.Required {
			issues = append(issues, siteconfig.Issue{
				Path:    "payload." + field.Name,
				Code:    "required",
				Message: msgs.required(field.Label),
			})
			continue
		}
		if trimmed != "" {
			normalized[field.Name] = trimmed
		}
	}

	for key := range payload {
		if _, ok := knownFields[key]; ok {
			continue
		}
		issues = append(issues, siteconfig.Issue{
			Path:    "payload." + key,
			Code:    "unknown_field",
			Message: msgs.unknownField,
		})
	}

	if len(issues) > 0 {
		return nil, siteconfig.ValidationError{Issues: issues}
	}
	return normalized, nil
}

func scanSubmission(scanner interface {
	Scan(dest ...any) error
}) (Submission, error) {
	var submission Submission
	var payloadJSON []byte
	var spamScore *float64
	var spamSignalsJSON []byte
	if err := scanner.Scan(
		&submission.ID,
		&submission.SiteID,
		&submission.PageID,
		&submission.BlockID,
		&submission.Status,
		&payloadJSON,
		&submission.CreatedAt,
		&submission.PageTitle,
		&spamScore,
		&spamSignalsJSON,
	); err != nil {
		return Submission{}, fmt.Errorf("scan form submission: %w", err)
	}
	if err := json.Unmarshal(payloadJSON, &submission.Payload); err != nil {
		return Submission{}, fmt.Errorf("decode form submission payload: %w", err)
	}
	if submission.Payload == nil {
		submission.Payload = map[string]any{}
	}
	submission.SpamScore = spamScore
	if len(spamSignalsJSON) > 0 {
		if err := json.Unmarshal(spamSignalsJSON, &submission.SpamSignals); err != nil {
			return Submission{}, fmt.Errorf("decode form submission spam signals: %w", err)
		}
	}
	return submission, nil
}

func scanSubmissionRow(row pgx.Row) (Submission, error) {
	return scanSubmission(row)
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func firstNonEmpty(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (s *Service) forwardSubmission(ctx context.Context, form resolvedForm, submission Submission) {
	if submission.Status == "spam" || s.emailSender.Mailer == nil {
		return
	}

	destination := strings.TrimSpace(form.Definition.NotificationEmail)
	if destination == "" {
		return
	}

	if s.emailRateLimiter != nil {
		allowed, err := s.emailRateLimiter.Allow(ctx, destination, "form_submission_forwarded",
			email.RateLimitRule{Limit: 30, Window: time.Hour},
		)
		if err != nil {
			s.logger.Warn("check form forwarding rate limit failed", "error", err, "siteId", submission.SiteID, "submissionId", submission.ID, "destination", destination)
			return
		}
		if !allowed {
			s.logger.Warn("form forwarding rate limited", "siteId", submission.SiteID, "submissionId", submission.ID, "destination", destination)
			return
		}
	}

	fields := make([]email.ForwardedField, 0, len(form.Definition.Fields))
	for _, field := range form.Definition.Fields {
		value, ok := submission.Payload[field.Name].(string)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		fields = append(fields, email.ForwardedField{
			Label: field.Label,
			Value: value,
		})
	}

	_, err := s.emailSender.SendFormSubmissionForwarded(ctx, email.Address{Email: destination}, email.FormSubmissionForwardedTemplateData{
		ProductName: s.productName,
		SiteName:    form.SiteName,
		PageTitle:   form.PageTitle,
		SubmittedAt: submission.CreatedAt.UTC().Format(time.RFC3339),
		Fields:      fields,
	}, "form-submission:"+submission.ID)
	if err != nil {
		s.logger.Warn("send form forwarding email failed", "error", err, "siteId", submission.SiteID, "submissionId", submission.ID, "destination", destination)
	}
}
