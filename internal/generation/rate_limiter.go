package generation

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/platform/timestamps"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type GenerationRateLimitStore interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type GenerationRateLimitRule struct {
	Limit  int
	Window time.Duration
}

type GenerationRateLimiter struct {
	store  GenerationRateLimitStore
	clock  timestamps.Clock
	logger *slog.Logger
}

func NewGenerationRateLimiter(store GenerationRateLimitStore, logger *slog.Logger) *GenerationRateLimiter {
	if logger == nil {
		logger = slog.Default()
	}
	return &GenerationRateLimiter{
		store:  store,
		clock:  timestamps.SystemClock{},
		logger: logger,
	}
}

func (l *GenerationRateLimiter) WithClock(clock timestamps.Clock) *GenerationRateLimiter {
	if l == nil || clock == nil {
		return l
	}
	cloned := *l
	cloned.clock = clock
	return &cloned
}

func (l *GenerationRateLimiter) Allow(ctx context.Context, workspaceID string, userID string, scope string) bool {
	if l == nil || l.store == nil {
		return true
	}
	workspaceID = strings.TrimSpace(workspaceID)
	scope = strings.TrimSpace(scope)
	if workspaceID == "" || scope == "" {
		return true
	}

	now := l.clock.Now().UTC()
	if !l.allowWorkspace(ctx, workspaceID, scope, now) {
		return false
	}
	if trimmedUserID := strings.TrimSpace(userID); trimmedUserID != "" {
		if !l.allowUser(ctx, workspaceID, trimmedUserID, scope, now) {
			return false
		}
		userID = trimmedUserID
	} else {
		userID = ""
	}

	if _, err := l.store.Exec(ctx, `
		insert into generation_attempts (workspace_id, user_id, scope, attempted_at)
		values ($1::uuid, nullif($2, '')::uuid, $3, $4)
	`, workspaceID, userID, scope, now); err != nil {
		l.logger.Warn("record generation attempt failed", "workspaceId", workspaceID, "scope", scope, "error", err)
		return true
	}

	if _, err := l.store.Exec(ctx, `
		delete from generation_attempts
		where workspace_id = $1::uuid
		  and scope = $2
		  and attempted_at <= $3
	`, workspaceID, scope, now.Add(-24*time.Hour)); err != nil {
		l.logger.Debug("prune generation attempts failed", "workspaceId", workspaceID, "scope", scope, "error", err)
	}
	return true
}

func (l *GenerationRateLimiter) allowWorkspace(ctx context.Context, workspaceID string, scope string, now time.Time) bool {
	for _, rule := range defaultGenerationWorkspaceRules {
		var attempts int
		if err := l.store.QueryRow(ctx, `
			select count(*)
			from generation_attempts
			where workspace_id = $1::uuid
			  and scope = $2
			  and attempted_at > $3
		`, workspaceID, scope, now.Add(-rule.Window)).Scan(&attempts); err != nil {
			l.logger.Warn("count workspace generation attempts failed", "workspaceId", workspaceID, "scope", scope, "error", err)
			return true
		}
		if attempts >= rule.Limit {
			return false
		}
	}
	return true
}

func (l *GenerationRateLimiter) allowUser(ctx context.Context, workspaceID string, userID string, scope string, now time.Time) bool {
	for _, rule := range defaultGenerationUserRules {
		var attempts int
		if err := l.store.QueryRow(ctx, `
			select count(*)
			from generation_attempts
			where workspace_id = $1::uuid
			  and user_id = $2::uuid
			  and scope = $3
			  and attempted_at > $4
		`, workspaceID, userID, scope, now.Add(-rule.Window)).Scan(&attempts); err != nil {
			l.logger.Warn("count user generation attempts failed", "workspaceId", workspaceID, "userId", userID, "scope", scope, "error", err)
			return true
		}
		if attempts >= rule.Limit {
			return false
		}
	}
	return true
}

var defaultGenerationWorkspaceRules = []GenerationRateLimitRule{
	{Limit: 12, Window: 10 * time.Minute},
	{Limit: 60, Window: 24 * time.Hour},
}

var defaultGenerationUserRules = []GenerationRateLimitRule{
	{Limit: 6, Window: 10 * time.Minute},
	{Limit: 30, Window: 24 * time.Hour},
}
