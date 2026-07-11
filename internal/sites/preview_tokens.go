package sites

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/jackc/pgx/v5"
)

var (
	ErrPreviewTokenNotFound = errors.New("preview token was not found")
	ErrPreviewTokenInvalid  = errors.New("preview token is invalid")
)

const DefaultPreviewTokenTTL = 7 * 24 * time.Hour

type PreviewToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type PreviewTokenService interface {
	Issue(ctx context.Context, siteID string, userID string) (PreviewToken, error)
	Revoke(ctx context.Context, siteID string) error
	LoadDraft(ctx context.Context, token string) (siteconfig.SiteDraft, error)
}

type PostgresPreviewTokenService struct {
	db        DB
	reader    Reader
	ttl       time.Duration
	now       func() time.Time
	newToken  func() (string, error)
	hashToken func(string) string
}

func NewPostgresPreviewTokenService(db DB, ttl time.Duration) *PostgresPreviewTokenService {
	if ttl <= 0 {
		ttl = DefaultPreviewTokenTTL
	}
	return &PostgresPreviewTokenService{
		db:        db,
		reader:    NewPostgresReader(db),
		ttl:       ttl,
		now:       time.Now,
		newToken:  newPreviewToken,
		hashToken: previewTokenHash,
	}
}

func (s *PostgresPreviewTokenService) Issue(ctx context.Context, siteID string, userID string) (PreviewToken, error) {
	siteID = strings.TrimSpace(siteID)
	userID = strings.TrimSpace(userID)
	if siteID == "" {
		return PreviewToken{}, ErrPreviewTokenInvalid
	}

	token, err := s.newToken()
	if err != nil {
		return PreviewToken{}, fmt.Errorf("generate preview token: %w", err)
	}

	if _, err := s.db.Exec(ctx, `
		update site_preview_tokens
		set revoked_at = coalesce(revoked_at, now())
		where site_id = $1
		  and revoked_at is null
		  and expires_at > now()
	`, siteID); err != nil {
		return PreviewToken{}, fmt.Errorf("revoke existing preview tokens: %w", err)
	}

	expiresAt := s.now().UTC().Add(s.ttl)
	if _, err := s.db.Exec(ctx, `
		insert into site_preview_tokens (site_id, created_by, token_hash, expires_at)
		values ($1, $2, $3, $4)
	`, siteID, nullableUUID(userID), s.hashToken(token), expiresAt); err != nil {
		return PreviewToken{}, fmt.Errorf("store preview token: %w", err)
	}

	return PreviewToken{
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

func nullableUUID(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (s *PostgresPreviewTokenService) Revoke(ctx context.Context, siteID string) error {
	siteID = strings.TrimSpace(siteID)
	if siteID == "" {
		return ErrPreviewTokenInvalid
	}

	if _, err := s.db.Exec(ctx, `
		update site_preview_tokens
		set revoked_at = coalesce(revoked_at, now())
		where site_id = $1
		  and revoked_at is null
		  and expires_at > now()
	`, siteID); err != nil {
		return fmt.Errorf("revoke preview tokens: %w", err)
	}
	return nil
}

// ResolveSiteID validates an unexpired, unrevoked preview token and returns
// the site it grants access to. It backs token-scoped public reads such as
// draft asset downloads.
func (s *PostgresPreviewTokenService) ResolveSiteID(ctx context.Context, token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", ErrPreviewTokenNotFound
	}

	var siteID string
	err := s.db.QueryRow(ctx, `
		select site_id::text
		from site_preview_tokens
		where token_hash = $1
		  and revoked_at is null
		  and expires_at > now()
	`, s.hashToken(token)).Scan(&siteID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrPreviewTokenNotFound
	}
	if err != nil {
		return "", fmt.Errorf("load preview token: %w", err)
	}
	return siteID, nil
}

func (s *PostgresPreviewTokenService) LoadDraft(ctx context.Context, token string) (siteconfig.SiteDraft, error) {
	token = strings.TrimSpace(token)
	siteID, err := s.ResolveSiteID(ctx, token)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}

	if _, err := s.db.Exec(ctx, `
		update site_preview_tokens
		set last_used_at = now()
		where token_hash = $1
		  and revoked_at is null
		  and expires_at > now()
	`, s.hashToken(token)); err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("touch preview token: %w", err)
	}

	return s.reader.LoadDraft(ctx, siteID)
}

func newPreviewToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func previewTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
