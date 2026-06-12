package generation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// PromptHistoryRecorder persists draft revisions and reprompt history entries
// for model-backed actions that live outside the generation.Service surface
// (notably the collection and entry draft handlers). Two captures and one
// reprompt_history insert run as separate statements; callers are responsible
// for not calling the recorder until after the draft mutation has committed.
type PromptHistoryRecorder struct {
	db     PromptActionDB
	logger *slog.Logger
}

// NewPromptHistoryRecorder wires a recorder against the shared DB pool.
// Returns nil when db is nil so callers can no-op when running without a
// database (e.g. in placeholder mode).
func NewPromptHistoryRecorder(db PromptActionDB, logger *slog.Logger) *PromptHistoryRecorder {
	if db == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &PromptHistoryRecorder{db: db, logger: logger}
}

// PromptHistoryInput describes a single prompt-action lifecycle that resulted
// in a mutated site draft. The recorder captures the pre/post draft revisions,
// writes a reprompt_history row tying them together, and returns the
// generated identifiers so callers can include them in API responses.
type PromptHistoryInput struct {
	WorkspaceID   string
	SiteID        string
	UserID        string
	JobID         string
	Scope         string
	TargetID      string
	Prompt        string
	ChangeSummary string
	PreviousDraft siteconfig.SiteDraft
	NextDraft     siteconfig.SiteDraft
	Summary       map[string]any
}

// PromptHistoryResult exposes the persisted identifiers so the caller can
// echo them to clients (and tests can assert against them).
type PromptHistoryResult struct {
	HistoryID          string
	PreviousRevisionID string
	ResultRevisionID   string
}

// Record captures a pre-revision of the draft as it was before the mutation,
// a post-revision of the draft as it was saved, and writes a reprompt_history
// row that points at the two revisions and the job that produced them. The
// caller must invoke Record after the draft mutation has been committed so
// the post-revision reflects the saved state.
func (r *PromptHistoryRecorder) Record(ctx context.Context, input PromptHistoryInput) (PromptHistoryResult, error) {
	if r == nil || r.db == nil {
		return PromptHistoryResult{}, nil
	}
	workspaceID := strings.TrimSpace(input.WorkspaceID)
	siteID := strings.TrimSpace(input.SiteID)
	if workspaceID == "" || siteID == "" {
		return PromptHistoryResult{}, fmt.Errorf("prompt history requires workspace and site")
	}
	scope := strings.TrimSpace(input.Scope)
	if scope == "" {
		return PromptHistoryResult{}, fmt.Errorf("prompt history scope is required")
	}

	summaryJSON, err := json.Marshal(orEmpty(input.Summary))
	if err != nil {
		return PromptHistoryResult{}, fmt.Errorf("encode prompt history summary: %w", err)
	}

	previousID, err := r.capture(ctx, draftRevisionParams{
		WorkspaceID: workspaceID,
		SiteID:      siteID,
		Scope:       scope,
		PageID:      revisionPageID(scope, input.TargetID),
		Prompt:      input.Prompt,
		Draft:       input.PreviousDraft,
		SummaryJSON: summaryJSON,
		CreatedBy:   input.UserID,
	})
	if err != nil {
		return PromptHistoryResult{}, err
	}
	resultID, err := r.capture(ctx, draftRevisionParams{
		WorkspaceID: workspaceID,
		SiteID:      siteID,
		Scope:       scope,
		PageID:      revisionPageID(scope, input.TargetID),
		Prompt:      input.Prompt,
		Draft:       input.NextDraft,
		SummaryJSON: summaryJSON,
		CreatedBy:   input.UserID,
	})
	if err != nil {
		return PromptHistoryResult{}, err
	}

	historyID, err := r.recordHistory(ctx, repromptHistoryParams{
		WorkspaceID:        workspaceID,
		SiteID:             siteID,
		Scope:              scope,
		TargetID:           input.TargetID,
		Prompt:             input.Prompt,
		ChangeSummary:      strings.TrimSpace(input.ChangeSummary),
		PreviousRevisionID: previousID,
		ResultRevisionID:   resultID,
		JobID:              input.JobID,
		CreatedBy:          input.UserID,
	})
	if err != nil {
		return PromptHistoryResult{}, err
	}

	return PromptHistoryResult{
		HistoryID:          historyID,
		PreviousRevisionID: previousID,
		ResultRevisionID:   resultID,
	}, nil
}

type draftRevisionParams struct {
	WorkspaceID string
	SiteID      string
	Scope       string
	PageID      string
	Prompt      string
	Draft       siteconfig.SiteDraft
	SummaryJSON []byte
	CreatedBy   string
}

func (r *PromptHistoryRecorder) capture(ctx context.Context, params draftRevisionParams) (string, error) {
	revisionJSON, err := json.Marshal(params.Draft)
	if err != nil {
		return "", fmt.Errorf("encode prompt history draft: %w", err)
	}
	summaryJSON := params.SummaryJSON
	if len(summaryJSON) == 0 {
		summaryJSON = []byte(`{}`)
	}
	var revisionID string
	if err := r.db.QueryRow(ctx, `
		insert into draft_revisions (
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
			$3,
			nullif($4, '')::uuid,
			$5,
			$6,
			$7,
			$8,
			nullif($9, '')::uuid
		)
		returning id::text
	`,
		params.SiteID,
		params.WorkspaceID,
		params.Scope,
		params.PageID,
		params.Prompt,
		revisionJSON,
		params.Prompt,
		summaryJSON,
		params.CreatedBy,
	).Scan(&revisionID); err != nil {
		return "", fmt.Errorf("capture prompt history draft: %w", err)
	}
	return revisionID, nil
}

type repromptHistoryParams struct {
	WorkspaceID        string
	SiteID             string
	Scope              string
	TargetID           string
	Prompt             string
	ChangeSummary      string
	PreviousRevisionID string
	ResultRevisionID   string
	JobID              string
	CreatedBy          string
}

func (r *PromptHistoryRecorder) recordHistory(ctx context.Context, params repromptHistoryParams) (string, error) {
	var historyID string
	if err := r.db.QueryRow(ctx, `
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
			gen_random_uuid(),
			$1::uuid,
			$2::uuid,
			$3,
			nullif($4, '')::uuid,
			$5,
			$6::uuid,
			$7::uuid,
			nullif($8, '')::uuid,
			$9,
			nullif($10, '')::uuid
		)
		returning id::text
	`,
		params.SiteID,
		params.WorkspaceID,
		params.Scope,
		params.TargetID,
		params.Prompt,
		params.PreviousRevisionID,
		params.ResultRevisionID,
		params.JobID,
		params.ChangeSummary,
		params.CreatedBy,
	).Scan(&historyID); err != nil {
		return "", fmt.Errorf("record prompt history: %w", err)
	}
	return historyID, nil
}

func orEmpty(summary map[string]any) map[string]any {
	if summary == nil {
		return map[string]any{}
	}
	return summary
}

// revisionPageID maps a scope onto the draft_revisions.page_id column. The
// column is page-specific so only page/block-scoped revisions populate it;
// collection/entry-scoped rows record their target via reprompt_history.target_id.
func revisionPageID(scope string, targetID string) string {
	switch scope {
	case "page", "block":
		return targetID
	default:
		return ""
	}
}
