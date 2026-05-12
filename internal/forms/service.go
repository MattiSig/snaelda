package forms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrSiteRequired            = errors.New("site id is required")
	ErrBlockRequired           = errors.New("block id is required")
	ErrSiteNotFound            = errors.New("site was not found")
	ErrFormBlockNotFound       = errors.New("contact form block was not found")
	ErrFormBlockInvalid        = errors.New("block is not a contact form")
	ErrSubmissionNotFound      = errors.New("form submission was not found")
	ErrSubmissionStatusInvalid = errors.New("submission status is not supported")
	ErrNoSubmissionChanges     = errors.New("submission update requires a change")
)

var (
	submissionPhonePattern  = regexp.MustCompile(`^[0-9+()./\-\s]{7,30}$`)
	allowedSubmissionStatus = map[string]bool{"new": true, "reviewed": true, "resolved": true, "spam": true}
	defaultSuccessMessage   = "Thanks. Your message is on its way."
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type DraftReader interface {
	LoadDraft(ctx context.Context, siteID string) (siteconfig.SiteDraft, error)
}

type Service struct {
	db     DB
	reader DraftReader
}

type Submission struct {
	ID        string         `json:"id"`
	SiteID    string         `json:"siteId"`
	PageID    string         `json:"pageId,omitempty"`
	BlockID   string         `json:"blockId,omitempty"`
	Status    string         `json:"status"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"createdAt"`
	PageTitle string         `json:"pageTitle,omitempty"`
}

type SubmitInput struct {
	SiteID  string
	BlockID string
	Payload map[string]any
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
	Definition     siteconfig.FormDefinition
	SuccessMessage string
}

func NewService(db DB) *Service {
	return &Service{
		db:     db,
		reader: sites.NewPostgresReader(db),
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

	payload, err := normalizeSubmissionPayload(form.Definition, input.Payload)
	if err != nil {
		return SubmitResult{}, err
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("encode form payload: %w", err)
	}

	submission := Submission{
		SiteID:  siteID,
		PageID:  form.PageID,
		BlockID: blockID,
		Status:  "new",
		Payload: payload,
	}
	if err := s.db.QueryRow(ctx, `
		insert into form_submissions (site_id, page_id, block_id, payload, status)
		values ($1::uuid, nullif($2, '')::uuid, nullif($3, '')::uuid, $4, $5)
		returning id::text, created_at
	`, siteID, form.PageID, blockID, payloadJSON, submission.Status).Scan(&submission.ID, &submission.CreatedAt); err != nil {
		return SubmitResult{}, fmt.Errorf("store form submission: %w", err)
	}

	return SubmitResult{
		Submission:     submission,
		SuccessMessage: firstNonEmpty(strings.TrimSpace(form.SuccessMessage), defaultSuccessMessage),
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
		       coalesce(p.title, '')
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
		          coalesce((select title from pages where id = fs.page_id), '')
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
	if snapshot, ok, err := s.loadPublishedSnapshot(ctx, siteID); err != nil {
		return resolvedForm{}, err
	} else if ok {
		if form, found, err := findFormInPages(snapshot.Pages, blockID); err != nil {
			return resolvedForm{}, err
		} else if found {
			return form, nil
		}
	}

	draft, err := s.reader.LoadDraft(ctx, siteID)
	if errors.Is(err, sites.ErrNotFound) {
		return resolvedForm{}, ErrSiteNotFound
	}
	if err != nil {
		return resolvedForm{}, fmt.Errorf("load draft for form resolution: %w", err)
	}

	form, found, err := findFormInPages(draft.Pages, blockID)
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

func findFormInPages(pages []siteconfig.PageDraft, blockID string) (resolvedForm, bool, error) {
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
				Definition:     definition,
				SuccessMessage: successMessage,
			}, true, nil
		}
	}
	return resolvedForm{}, false, nil
}

func normalizeSubmissionPayload(definition siteconfig.FormDefinition, payload map[string]any) (map[string]any, error) {
	if payload == nil {
		payload = map[string]any{}
	}

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
					Message: field.Label + " is required",
				})
			}
			continue
		}

		text, ok := raw.(string)
		if !ok {
			issues = append(issues, siteconfig.Issue{
				Path:    "payload." + field.Name,
				Code:    "invalid_type",
				Message: field.Label + " must be a string",
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
					Message: field.Label + " must be between 1 and 120 characters",
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
					Message: field.Label + " must be a valid email address",
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
					Message: field.Label + " must be a valid phone number",
				})
				continue
			}
		case "message":
			if len(trimmed) < 1 || len(trimmed) > 4000 {
				issues = append(issues, siteconfig.Issue{
					Path:    "payload." + field.Name,
					Code:    "invalid_length",
					Message: field.Label + " must be between 1 and 4000 characters",
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
					Message: field.Label + " must use one of the configured options",
				})
				continue
			}
		default:
			issues = append(issues, siteconfig.Issue{
				Path:    "payload." + field.Name,
				Code:    "unsupported_field",
				Message: field.Label + " is not supported",
			})
			continue
		}

		if trimmed == "" && field.Required {
			issues = append(issues, siteconfig.Issue{
				Path:    "payload." + field.Name,
				Code:    "required",
				Message: field.Label + " is required",
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
			Message: "field is not supported by this form",
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
	if err := scanner.Scan(
		&submission.ID,
		&submission.SiteID,
		&submission.PageID,
		&submission.BlockID,
		&submission.Status,
		&payloadJSON,
		&submission.CreatedAt,
		&submission.PageTitle,
	); err != nil {
		return Submission{}, fmt.Errorf("scan form submission: %w", err)
	}
	if err := json.Unmarshal(payloadJSON, &submission.Payload); err != nil {
		return Submission{}, fmt.Errorf("decode form submission payload: %w", err)
	}
	if submission.Payload == nil {
		submission.Payload = map[string]any{}
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
