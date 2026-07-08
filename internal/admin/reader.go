package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// QueryStore is the minimal database surface for admin reads.
type QueryStore interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Overview aggregates platform-wide counters for the operator control room.
type Overview struct {
	GenerationJobs GenerationJobStats  `json:"generationJobs"`
	Sites          SiteStats           `json:"sites"`
	Users          UserStats           `json:"users"`
	Publishes      PublishStats        `json:"publishes"`
	Forms          FormSubmissionStats `json:"forms"`
	GeneratedAt    time.Time           `json:"generatedAt"`
}

type GenerationJobStats struct {
	Last24Hours         int64            `json:"last24Hours"`
	Previous24Hours     int64            `json:"previous24Hours"`
	Last7Days           int64            `json:"last7Days"`
	Total               int64            `json:"total"`
	ByStatusLast24Hours map[string]int64 `json:"byStatusLast24Hours"`
}

type SiteStats struct {
	Total       int64 `json:"total"`
	Published   int64 `json:"published"`
	Last24Hours int64 `json:"last24Hours"`
	Last7Days   int64 `json:"last7Days"`
}

type UserStats struct {
	Total     int64 `json:"total"`
	Last7Days int64 `json:"last7Days"`
}

type PublishStats struct {
	Last24Hours int64 `json:"last24Hours"`
	Last7Days   int64 `json:"last7Days"`
}

type FormSubmissionStats struct {
	Last24Hours int64 `json:"last24Hours"`
}

// GenerationJobSummary is one row in the recent generation activity list.
type GenerationJobSummary struct {
	ID             string    `json:"id"`
	Status         string    `json:"status"`
	Prompt         string    `json:"prompt"`
	SiteID         string    `json:"siteId,omitempty"`
	SiteName       string    `json:"siteName,omitempty"`
	WorkspaceName  string    `json:"workspaceName,omitempty"`
	CreatedByEmail string    `json:"createdByEmail,omitempty"`
	HasError       bool      `json:"hasError"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// SiteSummary is one row in the recent sites list.
type SiteSummary struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	Status        string    `json:"status"`
	Published     bool      `json:"published"`
	WorkspaceName string    `json:"workspaceName,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Reader runs the read-only control-room queries.
type Reader struct {
	db    QueryStore
	clock func() time.Time
}

// NewReader creates a Reader with a system clock.
func NewReader(db QueryStore) *Reader {
	return &Reader{db: db, clock: func() time.Time { return time.Now().UTC() }}
}

// LoadOverview returns platform-wide counters across the core tables.
func (r *Reader) LoadOverview(ctx context.Context) (Overview, error) {
	overview := Overview{GeneratedAt: r.clock().UTC()}

	err := r.db.QueryRow(ctx, `
		select
			count(*) filter (where created_at > now() - interval '24 hours'),
			count(*) filter (where created_at > now() - interval '48 hours'
				and created_at <= now() - interval '24 hours'),
			count(*) filter (where created_at > now() - interval '7 days'),
			count(*)
		from generation_jobs
	`).Scan(
		&overview.GenerationJobs.Last24Hours,
		&overview.GenerationJobs.Previous24Hours,
		&overview.GenerationJobs.Last7Days,
		&overview.GenerationJobs.Total,
	)
	if err != nil {
		return Overview{}, fmt.Errorf("query generation job stats: %w", err)
	}

	byStatus, err := r.loadJobStatusCounts(ctx)
	if err != nil {
		return Overview{}, err
	}
	overview.GenerationJobs.ByStatusLast24Hours = byStatus

	err = r.db.QueryRow(ctx, `
		select
			count(*),
			count(*) filter (where published_version_id is not null),
			count(*) filter (where created_at > now() - interval '24 hours'),
			count(*) filter (where created_at > now() - interval '7 days')
		from sites
	`).Scan(
		&overview.Sites.Total,
		&overview.Sites.Published,
		&overview.Sites.Last24Hours,
		&overview.Sites.Last7Days,
	)
	if err != nil {
		return Overview{}, fmt.Errorf("query site stats: %w", err)
	}

	err = r.db.QueryRow(ctx, `
		select
			count(*),
			count(*) filter (where created_at > now() - interval '7 days')
		from users
	`).Scan(&overview.Users.Total, &overview.Users.Last7Days)
	if err != nil {
		return Overview{}, fmt.Errorf("query user stats: %w", err)
	}

	err = r.db.QueryRow(ctx, `
		select
			count(*) filter (where created_at > now() - interval '24 hours'),
			count(*) filter (where created_at > now() - interval '7 days')
		from site_versions
	`).Scan(&overview.Publishes.Last24Hours, &overview.Publishes.Last7Days)
	if err != nil {
		return Overview{}, fmt.Errorf("query publish stats: %w", err)
	}

	err = r.db.QueryRow(ctx, `
		select count(*)
		from form_submissions
		where created_at > now() - interval '24 hours'
	`).Scan(&overview.Forms.Last24Hours)
	if err != nil {
		return Overview{}, fmt.Errorf("query form submission stats: %w", err)
	}

	return overview, nil
}

func (r *Reader) loadJobStatusCounts(ctx context.Context) (map[string]int64, error) {
	rows, err := r.db.Query(ctx, `
		select status, count(*)
		from generation_jobs
		where created_at > now() - interval '24 hours'
		group by status
	`)
	if err != nil {
		return nil, fmt.Errorf("query generation job status counts: %w", err)
	}
	defer rows.Close()

	counts := map[string]int64{}
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan generation job status row: %w", err)
		}
		counts[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate generation job status rows: %w", err)
	}
	return counts, nil
}

// ListRecentGenerationJobs returns the newest generation jobs with site,
// workspace, and creator context for the activity feed.
func (r *Reader) ListRecentGenerationJobs(ctx context.Context, limit int) ([]GenerationJobSummary, error) {
	rows, err := r.db.Query(ctx, `
		select
			j.id::text,
			j.status,
			left(j.prompt, 200),
			coalesce(j.site_id::text, ''),
			coalesce(s.name, ''),
			coalesce(w.name, ''),
			coalesce(u.email, ''),
			j.error is not null,
			j.created_at,
			j.updated_at
		from generation_jobs j
		left join sites s on s.id = j.site_id
		left join workspaces w on w.id = j.workspace_id
		left join users u on u.id = j.created_by
		order by j.created_at desc
		limit $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent generation jobs: %w", err)
	}
	defer rows.Close()

	var result []GenerationJobSummary
	for rows.Next() {
		var job GenerationJobSummary
		if err := rows.Scan(
			&job.ID,
			&job.Status,
			&job.Prompt,
			&job.SiteID,
			&job.SiteName,
			&job.WorkspaceName,
			&job.CreatedByEmail,
			&job.HasError,
			&job.CreatedAt,
			&job.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan generation job row: %w", err)
		}
		result = append(result, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate generation job rows: %w", err)
	}
	return result, nil
}

// ListRecentSites returns the newest sites with workspace context.
func (r *Reader) ListRecentSites(ctx context.Context, limit int) ([]SiteSummary, error) {
	rows, err := r.db.Query(ctx, `
		select
			s.id::text,
			s.name,
			s.slug,
			s.status,
			s.published_version_id is not null,
			coalesce(w.name, ''),
			s.created_at,
			s.updated_at
		from sites s
		left join workspaces w on w.id = s.workspace_id
		order by s.created_at desc
		limit $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent sites: %w", err)
	}
	defer rows.Close()

	var result []SiteSummary
	for rows.Next() {
		var site SiteSummary
		if err := rows.Scan(
			&site.ID,
			&site.Name,
			&site.Slug,
			&site.Status,
			&site.Published,
			&site.WorkspaceName,
			&site.CreatedAt,
			&site.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan site row: %w", err)
		}
		result = append(result, site)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate site rows: %w", err)
	}
	return result, nil
}
