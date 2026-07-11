package respin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// GenerationLink is the generation job produced for a re-spin import, resolved
// through the generation_jobs.respin_import_id back-reference (Spec 21). It lets
// the endpoints recover the generated site and its (demo or caller) workspace
// without a dedicated column on the import record.
type GenerationLink struct {
	JobID       string
	SiteID      string
	WorkspaceID string
}

// LinkGenerationJob stamps a generation job with its originating re-spin import.
// The generation service creates the job itself (Spec 07 owns generation), so
// the linkage is written after the fact; it backs both the claim lookup and the
// publish gate's re-spin-origin detection (Spec 21).
func (s *Service) LinkGenerationJob(ctx context.Context, jobID, importID string) error {
	jobID = strings.TrimSpace(jobID)
	importID = strings.TrimSpace(importID)
	if jobID == "" || importID == "" {
		return fmt.Errorf("job id and import id are required")
	}
	if _, err := s.db.Exec(ctx, `
		update generation_jobs
		set respin_import_id = $2::uuid
		where id = $1::uuid
	`, jobID, importID); err != nil {
		return fmt.Errorf("link generation job to respin import: %w", err)
	}
	return nil
}

// LinkedGeneration returns the most recent generation job linked to the import,
// including the generated site and the workspace that owns it. It returns
// ErrNotFound when no generation has completed for the import yet.
func (s *Service) LinkedGeneration(ctx context.Context, importID string) (GenerationLink, error) {
	importID = strings.TrimSpace(importID)
	if importID == "" {
		return GenerationLink{}, ErrNotFound
	}
	var link GenerationLink
	err := s.db.QueryRow(ctx, `
		select id::text,
		       coalesce(site_id::text, ''),
		       workspace_id::text
		from generation_jobs
		where respin_import_id = $1::uuid
		order by created_at desc
		limit 1
	`, importID).Scan(&link.JobID, &link.SiteID, &link.WorkspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return GenerationLink{}, ErrNotFound
	}
	if err != nil {
		return GenerationLink{}, fmt.Errorf("load linked generation: %w", err)
	}
	return link, nil
}
