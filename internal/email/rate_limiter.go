package email

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type RateLimitStore interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type RateLimiter struct {
	store RateLimitStore
	now   func() time.Time
}

type RateLimitRule struct {
	Limit  int
	Window time.Duration
}

func NewRateLimiter(store RateLimitStore) *RateLimiter {
	return &RateLimiter{
		store: store,
		now:   time.Now,
	}
}

func (l *RateLimiter) WithClock(now func() time.Time) *RateLimiter {
	if now == nil {
		return l
	}
	cloned := *l
	cloned.now = now
	return &cloned
}

func (l *RateLimiter) Allow(ctx context.Context, emailAddress string, purpose string, rules ...RateLimitRule) (bool, error) {
	if l == nil || l.store == nil {
		return true, nil
	}
	if strings.TrimSpace(emailAddress) == "" {
		return false, fmt.Errorf("email address is required")
	}
	if strings.TrimSpace(purpose) == "" {
		return false, fmt.Errorf("purpose is required")
	}

	hash := HashAddress(emailAddress)
	now := l.now().UTC()
	for _, rule := range rules {
		if rule.Limit <= 0 || rule.Window <= 0 {
			continue
		}
		cutoff := now.Add(-rule.Window)
		var attempts int
		if err := l.store.QueryRow(ctx, `
			select count(*)
			from email_send_attempts
			where address_hash = $1
			  and purpose = $2
			  and occurred_at > $3
		`, hash, purpose, cutoff).Scan(&attempts); err != nil {
			return false, err
		}
		if attempts >= rule.Limit {
			return false, nil
		}
	}

	if _, err := l.store.Exec(ctx, `
		insert into email_send_attempts (address_hash, purpose, occurred_at)
		values ($1, $2, $3)
	`, hash, purpose, now); err != nil {
		return false, err
	}
	return true, nil
}

func HashAddress(address string) string {
	normalized := strings.ToLower(strings.TrimSpace(address))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
