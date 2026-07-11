package respin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Fetch statuses form the re-spin import state machine (Spec 21). The pipeline
// advances queued -> fetching -> extracting -> composing -> succeeded; degraded
// and failed are terminal alternatives.
const (
	StatusQueued     = "queued"
	StatusFetching   = "fetching"
	StatusExtracting = "extracting"
	StatusComposing  = "composing"
	StatusSucceeded  = "succeeded"
	StatusDegraded   = "degraded"
	StatusFailed     = "failed"
)

// Fetch modes record how the source page was retrieved.
const (
	ModePlain    = "plain"
	ModeHeadless = "headless"
)

var validStatuses = map[string]bool{
	StatusQueued:     true,
	StatusFetching:   true,
	StatusExtracting: true,
	StatusComposing:  true,
	StatusSucceeded:  true,
	StatusDegraded:   true,
	StatusFailed:     true,
}

var validModes = map[string]bool{
	ModePlain:    true,
	ModeHeadless: true,
}

var (
	ErrNotFound              = errors.New("respin import was not found")
	ErrSourceURLRequired     = errors.New("source url is required")
	ErrNormalizedURLRequired = errors.New("normalized url is required")
	ErrInvalidStatus         = errors.New("respin fetch status is not supported")
	ErrInvalidMode           = errors.New("respin fetch mode is not supported")
	ErrShareSlugRequired     = errors.New("share slug is required")
	ErrWorkspaceRequired     = errors.New("workspace id is required")
	ErrAlreadyClaimed        = errors.New("respin import is already claimed")
)

// DB is the subset of the pgx pool used by the respin store.
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

// Service is the store layer for re-spin URL imports.
type Service struct {
	db DB
}

// NewService constructs a respin store over the given database handle.
func NewService(db DB) *Service {
	return &Service{db: db}
}

// Import is a single re-spin URL import record. The extracted content,
// classification, and error payloads are stored as raw JSON so the store does
// not pin their shape ahead of the extraction/classification stages.
type Import struct {
	ID                string          `json:"id"`
	WorkspaceID       string          `json:"workspaceId,omitempty"`
	GuestSessionID    string          `json:"guestSessionId,omitempty"`
	SourceURL         string          `json:"sourceUrl"`
	NormalizedURL     string          `json:"normalizedUrl"`
	FetchMode         string          `json:"fetchMode,omitempty"`
	FetchStatus       string          `json:"fetchStatus"`
	ExtractedContent  json.RawMessage `json:"extractedContent,omitempty"`
	Classification    json.RawMessage `json:"classification,omitempty"`
	PulledAssetIDs    []string        `json:"pulledAssetIds"`
	Degraded          bool            `json:"degraded"`
	DegradationReason string          `json:"degradationReason,omitempty"`
	ShareSlug         string          `json:"shareSlug,omitempty"`
	Error             json.RawMessage `json:"error,omitempty"`
	CreatedAt         time.Time       `json:"createdAt"`
	UpdatedAt         time.Time       `json:"updatedAt"`
}

// CreateInput describes a new re-spin import. A public-demo import exists before
// any workspace does, so both WorkspaceID and GuestSessionID are optional and
// are bound on claim (Spec 21).
type CreateInput struct {
	SourceURL      string
	NormalizedURL  string
	WorkspaceID    string
	GuestSessionID string
}

const importColumns = `
	id::text,
	coalesce(workspace_id::text, ''),
	coalesce(guest_session_id::text, ''),
	source_url,
	normalized_url,
	coalesce(fetch_mode, ''),
	fetch_status,
	extracted_content,
	classification,
	pulled_asset_ids,
	degraded,
	coalesce(degradation_reason, ''),
	coalesce(share_slug, ''),
	error,
	created_at,
	updated_at
`

// Create inserts a new re-spin import in the queued state and returns it.
func (s *Service) Create(ctx context.Context, input CreateInput) (Import, error) {
	sourceURL := strings.TrimSpace(input.SourceURL)
	if sourceURL == "" {
		return Import{}, ErrSourceURLRequired
	}
	normalizedURL := strings.TrimSpace(input.NormalizedURL)
	if normalizedURL == "" {
		return Import{}, ErrNormalizedURLRequired
	}

	row := s.db.QueryRow(ctx, `
		insert into respin_imports (workspace_id, guest_session_id, source_url, normalized_url)
		values (nullif($1, '')::uuid, nullif($2, '')::uuid, $3, $4)
		returning `+importColumns,
		strings.TrimSpace(input.WorkspaceID),
		strings.TrimSpace(input.GuestSessionID),
		sourceURL,
		normalizedURL,
	)
	return scanImportRow(row)
}

// Get loads a re-spin import by id.
func (s *Service) Get(ctx context.Context, id string) (Import, error) {
	row := s.db.QueryRow(ctx, `select `+importColumns+` from respin_imports where id = $1::uuid`, strings.TrimSpace(id))
	return scanImportRow(row)
}

// GetByShareSlug loads a re-spin import by its shareable slug.
func (s *Service) GetByShareSlug(ctx context.Context, slug string) (Import, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return Import{}, ErrShareSlugRequired
	}
	row := s.db.QueryRow(ctx, `select `+importColumns+` from respin_imports where share_slug = $1`, slug)
	return scanImportRow(row)
}

// FindCached returns the most recent successful or degraded import for a
// normalized URL that was created at or after notBefore, backing the
// per-normalized-URL result cache (Spec 21 security contract). It returns
// ErrNotFound when no fresh cached import exists.
func (s *Service) FindCached(ctx context.Context, normalizedURL string, notBefore time.Time) (Import, error) {
	normalizedURL = strings.TrimSpace(normalizedURL)
	if normalizedURL == "" {
		return Import{}, ErrNormalizedURLRequired
	}
	row := s.db.QueryRow(ctx, `
		select `+importColumns+`
		from respin_imports
		where normalized_url = $1
		  and fetch_status in ('succeeded', 'degraded')
		  and created_at >= $2
		order by created_at desc
		limit 1
	`, normalizedURL, notBefore)
	return scanImportRow(row)
}

// UpdateStatus transitions the fetch status. When mode is non-empty it is
// recorded as the fetch mode used.
func (s *Service) UpdateStatus(ctx context.Context, id, status, mode string) (Import, error) {
	status = strings.TrimSpace(status)
	if !validStatuses[status] {
		return Import{}, ErrInvalidStatus
	}
	mode = strings.TrimSpace(mode)
	if mode != "" && !validModes[mode] {
		return Import{}, ErrInvalidMode
	}
	row := s.db.QueryRow(ctx, `
		update respin_imports
		set fetch_status = $2,
		    fetch_mode = coalesce(nullif($3, ''), fetch_mode)
		where id = $1::uuid
		returning `+importColumns,
		strings.TrimSpace(id), status, mode)
	return scanImportRow(row)
}

// SaveExtraction records the structured extraction and classification payloads.
// Passing a nil payload leaves the corresponding column unchanged.
func (s *Service) SaveExtraction(ctx context.Context, id string, extractedContent, classification json.RawMessage) (Import, error) {
	row := s.db.QueryRow(ctx, `
		update respin_imports
		set extracted_content = coalesce($2, extracted_content),
		    classification = coalesce($3, classification)
		where id = $1::uuid
		returning `+importColumns,
		strings.TrimSpace(id),
		nullableJSON(extractedContent),
		nullableJSON(classification))
	return scanImportRow(row)
}

// SavePulledAssets records the ids of assets ingested from the source page.
func (s *Service) SavePulledAssets(ctx context.Context, id string, assetIDs []string) (Import, error) {
	if assetIDs == nil {
		assetIDs = []string{}
	}
	payload, err := json.Marshal(assetIDs)
	if err != nil {
		return Import{}, fmt.Errorf("encode pulled asset ids: %w", err)
	}
	row := s.db.QueryRow(ctx, `
		update respin_imports
		set pulled_asset_ids = $2
		where id = $1::uuid
		returning `+importColumns,
		strings.TrimSpace(id), payload)
	return scanImportRow(row)
}

// MarkDegraded flags the import as degraded with the given reason and moves it
// to the degraded terminal state (Spec 21 graceful degradation).
func (s *Service) MarkDegraded(ctx context.Context, id, reason string) (Import, error) {
	row := s.db.QueryRow(ctx, `
		update respin_imports
		set degraded = true,
		    degradation_reason = nullif($2, ''),
		    fetch_status = 'degraded'
		where id = $1::uuid
		returning `+importColumns,
		strings.TrimSpace(id), strings.TrimSpace(reason))
	return scanImportRow(row)
}

// Fail records a structured error payload and moves the import to the failed
// terminal state.
func (s *Service) Fail(ctx context.Context, id string, errPayload json.RawMessage) (Import, error) {
	row := s.db.QueryRow(ctx, `
		update respin_imports
		set error = $2,
		    fetch_status = 'failed'
		where id = $1::uuid
		returning `+importColumns,
		strings.TrimSpace(id), nullableJSON(errPayload))
	return scanImportRow(row)
}

// AssignShareSlug sets the shareable slug backing the before/after URL.
func (s *Service) AssignShareSlug(ctx context.Context, id, slug string) (Import, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return Import{}, ErrShareSlugRequired
	}
	row := s.db.QueryRow(ctx, `
		update respin_imports
		set share_slug = $2
		where id = $1::uuid
		returning `+importColumns,
		strings.TrimSpace(id), slug)
	return scanImportRow(row)
}

// Claim binds an unclaimed import to a workspace (and optional guest session)
// when the visitor signs up to keep the result (Spec 21). It returns
// ErrAlreadyClaimed when the import already belongs to a workspace and
// ErrNotFound when no such import exists.
func (s *Service) Claim(ctx context.Context, id, workspaceID, guestSessionID string) (Import, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return Import{}, ErrWorkspaceRequired
	}
	row := s.db.QueryRow(ctx, `
		update respin_imports
		set workspace_id = $2::uuid,
		    guest_session_id = coalesce(nullif($3, '')::uuid, guest_session_id)
		where id = $1::uuid and workspace_id is null
		returning `+importColumns,
		strings.TrimSpace(id), workspaceID, strings.TrimSpace(guestSessionID))
	imported, err := scanImportRow(row)
	if errors.Is(err, ErrNotFound) {
		// Distinguish an already-claimed import from a missing one.
		if _, getErr := s.Get(ctx, id); getErr == nil {
			return Import{}, ErrAlreadyClaimed
		}
		return Import{}, ErrNotFound
	}
	return imported, err
}

// DeleteUnclaimedBefore garbage-collects unclaimed demo imports created before
// the cutoff, returning the number of rows removed. Assets are cleaned up by
// their own lifecycle; this removes the import records themselves.
func (s *Service) DeleteUnclaimedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := s.db.Exec(ctx, `
		delete from respin_imports
		where workspace_id is null and created_at < $1
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete unclaimed respin imports: %w", err)
	}
	return tag.RowsAffected(), nil
}

// nullableJSON returns nil for an empty payload so a nil raw message maps to SQL
// NULL (and, via coalesce, leaves the column unchanged).
func nullableJSON(payload json.RawMessage) any {
	if len(payload) == 0 {
		return nil
	}
	return []byte(payload)
}

func scanImportRow(scanner interface {
	Scan(dest ...any) error
}) (Import, error) {
	var (
		imp            Import
		extracted      []byte
		classification []byte
		pulledAssets   []byte
		errPayload     []byte
	)
	if err := scanner.Scan(
		&imp.ID,
		&imp.WorkspaceID,
		&imp.GuestSessionID,
		&imp.SourceURL,
		&imp.NormalizedURL,
		&imp.FetchMode,
		&imp.FetchStatus,
		&extracted,
		&classification,
		&pulledAssets,
		&imp.Degraded,
		&imp.DegradationReason,
		&imp.ShareSlug,
		&errPayload,
		&imp.CreatedAt,
		&imp.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Import{}, ErrNotFound
		}
		return Import{}, fmt.Errorf("scan respin import: %w", err)
	}

	if len(extracted) > 0 {
		imp.ExtractedContent = json.RawMessage(extracted)
	}
	if len(classification) > 0 {
		imp.Classification = json.RawMessage(classification)
	}
	if len(errPayload) > 0 {
		imp.Error = json.RawMessage(errPayload)
	}

	imp.PulledAssetIDs = []string{}
	if len(pulledAssets) > 0 {
		if err := json.Unmarshal(pulledAssets, &imp.PulledAssetIDs); err != nil {
			return Import{}, fmt.Errorf("decode pulled asset ids: %w", err)
		}
		if imp.PulledAssetIDs == nil {
			imp.PulledAssetIDs = []string{}
		}
	}

	return imp, nil
}
