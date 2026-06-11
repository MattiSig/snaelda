package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/platform/ids"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/jackc/pgx/v5"
)

var ErrRepromptNotFound = errors.New("reprompt history entry not found")

type RepromptHistoryEntry struct {
	ID               string     `json:"id"`
	Scope            string     `json:"scope"`
	TargetID         string     `json:"targetId,omitempty"`
	Prompt           string     `json:"prompt"`
	ChangeSummary    string     `json:"changeSummary,omitempty"`
	PreviousRevision string     `json:"previousRevisionId"`
	ResultRevision   string     `json:"resultRevisionId"`
	JobID            string     `json:"jobId,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UndoneAt         *time.Time `json:"undoneAt,omitempty"`
}

type DraftRevision struct {
	ID        string               `json:"id"`
	Scope     string               `json:"scope"`
	TargetID  string               `json:"targetId,omitempty"`
	Prompt    string               `json:"prompt"`
	Draft     siteconfig.SiteDraft `json:"draft"`
	CreatedAt time.Time            `json:"createdAt"`
}

type repromptHistoryRecord struct {
	ID                 string
	Scope              string
	TargetID           string
	Prompt             string
	ChangeSummary      string
	PreviousRevisionID string
	ResultRevisionID   string
	JobID              string
	CreatedBy          string
	CreatedAt          time.Time
	UndoneAt           *time.Time
}

func (s *Service) ListRepromptHistory(ctx context.Context, workspaceID string, siteID string) ([]RepromptHistoryEntry, error) {
	rows, err := s.db.Query(ctx, `
		select id::text,
		       scope,
		       coalesce(target_id::text, ''),
		       prompt,
		       coalesce(change_summary, ''),
		       previous_revision_id::text,
		       result_revision_id::text,
		       coalesce(job_id::text, ''),
		       created_at,
		       undone_at
		from reprompt_history
		where workspace_id = $1::uuid
		  and site_id = $2::uuid
		order by created_at desc, id desc
	`, workspaceID, siteID)
	if err != nil {
		return nil, fmt.Errorf("list reprompt history: %w", err)
	}
	defer rows.Close()

	entries := make([]RepromptHistoryEntry, 0)
	for rows.Next() {
		var entry RepromptHistoryEntry
		var undoneAt *time.Time
		if err := rows.Scan(
			&entry.ID,
			&entry.Scope,
			&entry.TargetID,
			&entry.Prompt,
			&entry.ChangeSummary,
			&entry.PreviousRevision,
			&entry.ResultRevision,
			&entry.JobID,
			&entry.CreatedAt,
			&undoneAt,
		); err != nil {
			return nil, fmt.Errorf("scan reprompt history: %w", err)
		}
		entry.UndoneAt = undoneAt
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reprompt history: %w", err)
	}
	return entries, nil
}

func (s *Service) LoadDraftRevision(ctx context.Context, workspaceID string, siteID string, revisionID string) (DraftRevision, error) {
	revision, err := s.loadDraftRevision(ctx, workspaceID, siteID, revisionID)
	if err != nil {
		return DraftRevision{}, err
	}
	return DraftRevision{
		ID:        revision.ID,
		Scope:     revision.Scope,
		TargetID:  revision.PageID,
		Prompt:    revision.Prompt,
		Draft:     revision.Draft,
		CreatedAt: revision.CreatedAt,
	}, nil
}

func (s *Service) RevertReprompt(ctx context.Context, workspaceID string, siteID string, repromptID string) (siteconfig.SiteDraft, error) {
	entry, err := s.loadRepromptHistory(ctx, workspaceID, siteID, repromptID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	revision, err := s.loadDraftRevision(ctx, workspaceID, siteID, entry.PreviousRevision)
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
	if err := s.markRepromptUndone(ctx, repromptID); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	return s.reader.LoadDraft(ctx, siteID)
}

func (s *Service) loadLatestRepromptHistory(ctx context.Context, workspaceID string, siteID string) (RepromptHistoryEntry, error) {
	rows, err := s.ListRepromptHistory(ctx, workspaceID, siteID)
	if err != nil {
		return RepromptHistoryEntry{}, err
	}
	for _, entry := range rows {
		if entry.UndoneAt == nil {
			return entry, nil
		}
	}
	return RepromptHistoryEntry{}, ErrRepromptNotFound
}

func (s *Service) recordRepromptHistory(ctx context.Context, workspaceID string, siteID string, record repromptHistoryRecord) error {
	if record.ID == "" {
		record.ID = ids.MustNew()
	}
	if _, err := s.db.Exec(ctx, `
		insert into reprompt_history (
			id,
			site_id,
			workspace_id,
			scope,
			target_id,
			prompt,
			previous_revision_id,
			result_revision_id,
			job_id,
			change_summary,
			created_by
		)
		values (
			$1::uuid,
			$2::uuid,
			$3::uuid,
			$4,
			nullif($5, '')::uuid,
			$6,
			$7::uuid,
			$8::uuid,
			nullif($9, '')::uuid,
			$10,
			nullif($11, '')::uuid
		)
	`, record.ID, siteID, workspaceID, record.Scope, record.TargetID, record.Prompt, record.PreviousRevisionID, record.ResultRevisionID, record.JobID, record.ChangeSummary, record.CreatedBy); err != nil {
		return fmt.Errorf("record reprompt history: %w", err)
	}
	return nil
}

func (s *Service) loadRepromptHistory(ctx context.Context, workspaceID string, siteID string, repromptID string) (RepromptHistoryEntry, error) {
	var entry RepromptHistoryEntry
	var undoneAt *time.Time
	if err := s.db.QueryRow(ctx, `
		select id::text,
		       scope,
		       coalesce(target_id::text, ''),
		       prompt,
		       coalesce(change_summary, ''),
		       previous_revision_id::text,
		       result_revision_id::text,
		       coalesce(job_id::text, ''),
		       created_at,
		       undone_at
		from reprompt_history
		where id = $1::uuid
		  and site_id = $2::uuid
		  and workspace_id = $3::uuid
	`, repromptID, siteID, workspaceID).Scan(
		&entry.ID,
		&entry.Scope,
		&entry.TargetID,
		&entry.Prompt,
		&entry.ChangeSummary,
		&entry.PreviousRevision,
		&entry.ResultRevision,
		&entry.JobID,
		&entry.CreatedAt,
		&undoneAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RepromptHistoryEntry{}, ErrRepromptNotFound
		}
		return RepromptHistoryEntry{}, fmt.Errorf("load reprompt history: %w", err)
	}
	entry.UndoneAt = undoneAt
	return entry, nil
}

func (s *Service) markRepromptUndone(ctx context.Context, repromptID string) error {
	if _, err := s.db.Exec(ctx, `
		update reprompt_history
		set undone_at = now()
		where id = $1::uuid
	`, repromptID); err != nil {
		return fmt.Errorf("mark reprompt undone: %w", err)
	}
	return nil
}

func (s *Service) captureDraftRevision(ctx context.Context, workspaceID string, siteID string, revision draftRevisionRecord) (string, error) {
	revisionID := revision.ID
	if revisionID == "" {
		revisionID = ids.MustNew()
	}
	revisionJSON, err := json.Marshal(revision.Draft)
	if err != nil {
		return "", fmt.Errorf("encode draft revision: %w", err)
	}

	summaryJSON := revision.GenerationSummaryJSON
	if len(summaryJSON) == 0 {
		summaryJSON = []byte(`{}`)
	}

	if _, err := s.db.Exec(ctx, `
		insert into draft_revisions (
			id,
			site_id,
			workspace_id,
			scope,
			page_id,
			prompt,
			draft,
			generation_prompt,
			generation_summary,
			created_by
		)
		values (
			$1::uuid,
			$2::uuid,
			$3::uuid,
			$4,
			nullif($5, '')::uuid,
			$6,
			$7,
			$8,
			$9,
			nullif($10, '')::uuid
		)
	`, revisionID, siteID, workspaceID, revision.Scope, revision.PageID, revision.Prompt, revisionJSON, revision.GenerationPrompt, summaryJSON, revision.CreatedBy); err != nil {
		return "", fmt.Errorf("capture draft revision: %w", err)
	}
	return revisionID, nil
}

func (s *Service) loadDraftRevision(ctx context.Context, workspaceID string, siteID string, revisionID string) (draftRevisionRecord, error) {
	var revision draftRevisionRecord
	var pageID string
	var draftJSON []byte
	var summaryJSON []byte
	if err := s.db.QueryRow(ctx, `
		select id::text,
		       scope,
		       coalesce(page_id::text, ''),
		       coalesce(prompt, ''),
		       draft,
		       coalesce(generation_prompt, ''),
		       generation_summary,
		       created_at
		from draft_revisions
		where id = $1::uuid
		  and site_id = $2::uuid
		  and workspace_id = $3::uuid
	`, revisionID, siteID, workspaceID).Scan(
		&revision.ID,
		&revision.Scope,
		&pageID,
		&revision.Prompt,
		&draftJSON,
		&revision.GenerationPrompt,
		&summaryJSON,
		&revision.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return draftRevisionRecord{}, ErrNoDraftRevision
		}
		return draftRevisionRecord{}, fmt.Errorf("load draft revision: %w", err)
	}
	if err := json.Unmarshal(draftJSON, &revision.Draft); err != nil {
		return draftRevisionRecord{}, fmt.Errorf("decode draft revision: %w", err)
	}
	revision.PageID = pageID
	revision.GenerationSummaryJSON = summaryJSON
	return revision, nil
}

func (s *Service) loadLatestDraftRevision(ctx context.Context, workspaceID string, siteID string) (draftRevisionRecord, error) {
	var revisionID string
	if err := s.db.QueryRow(ctx, `
		select id::text
		from draft_revisions
		where site_id = $1::uuid
		  and workspace_id = $2::uuid
		order by created_at desc, id desc
		limit 1
	`, siteID, workspaceID).Scan(&revisionID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return draftRevisionRecord{}, ErrNoDraftRevision
		}
		return draftRevisionRecord{}, fmt.Errorf("load draft revision id: %w", err)
	}
	return s.loadDraftRevision(ctx, workspaceID, siteID, revisionID)
}

func summarizeReprompt(scope string, draft siteconfig.SiteDraft, targetID string) string {
	switch scope {
	case "page":
		for _, page := range draft.Pages {
			if page.ID == targetID {
				return fmt.Sprintf("Rewrote the %s page.", strings.TrimSpace(page.Title))
			}
		}
		return "Rewrote the selected page."
	default:
		pageCount := len(draft.Pages)
		if pageCount == 1 {
			return "Rebuilt the whole site direction across 1 page."
		}
		return fmt.Sprintf("Rebuilt the whole site direction across %d pages.", pageCount)
	}
}
