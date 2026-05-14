package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type UserStore interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type HandlerConfig struct {
	Store           UserStore
	Tokens          *TokenManager
	RefreshTokenTTL time.Duration
	CookieSecure    bool
}

type Handler struct {
	store           UserStore
	tokens          *TokenManager
	refreshTokenTTL time.Duration
	cookieSecure    bool
}

type loginRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type authResponse struct {
	User      User   `json:"user"`
	ExpiresAt int64  `json:"expiresAt,omitempty"`
	TokenType string `json:"tokenType,omitempty"`
}

func NewHandler(cfg HandlerConfig) *Handler {
	refreshTokenTTL := cfg.RefreshTokenTTL
	if refreshTokenTTL <= 0 {
		refreshTokenTTL = 30 * 24 * time.Hour
	}

	return &Handler{
		store:           cfg.Store,
		tokens:          cfg.Tokens,
		refreshTokenTTL: refreshTokenTTL,
		cookieSecure:    cfg.CookieSecure,
	}
}

func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/login", h.login)
	mux.HandleFunc("POST /api/auth/refresh", h.refresh)
	mux.Handle("GET /api/auth/me", h.RequireUser(http.HandlerFunc(h.me)))
	mux.HandleFunc("POST /api/auth/logout", h.logout)
}

func (h *Handler) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.tokens == nil {
			writeAuthError(w, http.StatusServiceUnavailable, "auth_unavailable", "authentication is not configured")
			return
		}

		rawToken, err := CookieFromRequest(r)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "authentication is required")
			return
		}

		claims, err := h.tokens.Validate(rawToken)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "authentication is required")
			return
		}
		if err := h.requireActiveSession(r.Context(), claims); err != nil {
			writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "authentication is required")
			return
		}

		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), UserFromClaims(claims))))
	})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if h.store == nil || h.tokens == nil {
		writeAuthError(w, http.StatusServiceUnavailable, "auth_unavailable", "authentication is not configured")
		return
	}

	var payload loginRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	email := strings.ToLower(strings.TrimSpace(payload.Email))
	if email == "" || !strings.Contains(email, "@") {
		writeAuthError(w, http.StatusBadRequest, "invalid_email", "email is required")
		return
	}

	user, err := h.upsertUser(r.Context(), email, strings.TrimSpace(payload.Name))
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "login_failed", "could not sign in")
		return
	}

	refreshToken, err := newRefreshToken()
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "login_failed", "could not sign in")
		return
	}
	csrfToken, err := newCSRFCookieToken()
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "login_failed", "could not sign in")
		return
	}
	sessionID, err := h.createSession(r.Context(), user.ID, refreshToken, h.refreshTokenTTL, r.UserAgent())
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "login_failed", "could not sign in")
		return
	}

	token, claims, err := h.tokens.IssueForSession(user, sessionID)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "login_failed", "could not sign in")
		return
	}

	http.SetCookie(w, h.accessCookie(token, int(h.tokens.TTL().Seconds())))
	http.SetCookie(w, h.refreshCookie(refreshToken, int(h.refreshTokenTTL.Seconds())))
	http.SetCookie(w, h.csrfCookie(csrfToken, int(h.refreshTokenTTL.Seconds())))
	writeAuthJSON(w, http.StatusOK, authResponse{
		User:      user,
		ExpiresAt: claims.ExpiresAt,
		TokenType: "Bearer",
	})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "authentication is required")
		return
	}
	writeAuthJSON(w, http.StatusOK, authResponse{User: user})
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	if h.store == nil || h.tokens == nil {
		writeAuthError(w, http.StatusServiceUnavailable, "auth_unavailable", "authentication is not configured")
		return
	}

	refreshToken, err := RefreshCookieFromRequest(r)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "authentication is required")
		return
	}

	user, sessionID, nextRefreshToken, err := h.rotateSession(r.Context(), refreshToken)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "authentication is required")
		return
	}
	csrfToken, err := newCSRFCookieToken()
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "refresh_failed", "could not refresh session")
		return
	}

	token, claims, err := h.tokens.IssueForSession(user, sessionID)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "refresh_failed", "could not refresh session")
		return
	}

	http.SetCookie(w, h.accessCookie(token, int(h.tokens.TTL().Seconds())))
	http.SetCookie(w, h.refreshCookie(nextRefreshToken, int(h.refreshTokenTTL.Seconds())))
	http.SetCookie(w, h.csrfCookie(csrfToken, int(h.refreshTokenTTL.Seconds())))
	writeAuthJSON(w, http.StatusOK, authResponse{
		User:      user,
		ExpiresAt: claims.ExpiresAt,
		TokenType: "Bearer",
	})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if h.store != nil {
		if refreshToken, err := RefreshCookieFromRequest(r); err == nil {
			_ = h.revokeSessionByRefreshToken(r.Context(), refreshToken)
		}
		if rawToken, err := CookieFromRequest(r); err == nil && h.tokens != nil {
			if claims, err := h.tokens.Validate(rawToken); err == nil {
				_ = h.revokeSessionByID(r.Context(), claims.SessionID)
			}
		}
	}

	http.SetCookie(w, h.accessCookie("", -1))
	http.SetCookie(w, h.refreshCookie("", -1))
	http.SetCookie(w, h.csrfCookie("", -1))
	writeAuthJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) upsertUser(ctx context.Context, email string, name string) (User, error) {
	var user User
	err := h.store.QueryRow(ctx, `
		insert into users (email, name)
		values ($1, nullif($2, ''))
		on conflict (email) do update
		set name = coalesce(nullif(excluded.name, ''), users.name),
		    updated_at = now()
		returning id::text, email, coalesce(name, '')
	`, email, name).Scan(&user.ID, &user.Email, &user.Name)
	if err != nil {
		return User{}, fmt.Errorf("upsert user: %w", err)
	}

	workspaceID, role, err := h.defaultWorkspace(ctx, user.ID, user.Email)
	if err != nil {
		return User{}, err
	}
	user.WorkspaceID = workspaceID
	user.WorkspaceRole = role
	return user, nil
}

func (h *Handler) defaultWorkspace(ctx context.Context, userID string, email string) (string, string, error) {
	var workspaceID string
	var role string
	err := h.store.QueryRow(ctx, `
		select w.id::text, wm.role
		from workspaces w
		join workspace_members wm on wm.workspace_id = w.id
		where wm.user_id = $1
		order by wm.created_at asc
		limit 1
	`, userID).Scan(&workspaceID, &role)
	if err == nil {
		return workspaceID, role, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", fmt.Errorf("load default workspace: %w", err)
	}

	workspaceName := workspaceNameFromEmail(email)
	err = h.store.QueryRow(ctx, `
		insert into workspaces (name, created_by)
		values ($1, $2)
		returning id::text
	`, workspaceName, userID).Scan(&workspaceID)
	if err != nil {
		return "", "", fmt.Errorf("create default workspace: %w", err)
	}

	if _, err := h.store.Exec(ctx, `
		insert into workspace_members (workspace_id, user_id, role)
		values ($1, $2, 'owner')
	`, workspaceID, userID); err != nil {
		return "", "", fmt.Errorf("create default workspace member: %w", err)
	}

	return workspaceID, "owner", nil
}

func workspaceNameFromEmail(email string) string {
	localPart, _, found := strings.Cut(email, "@")
	if !found || localPart == "" {
		return "Default Workspace"
	}
	return strings.ReplaceAll(localPart, ".", " ") + " Workspace"
}

func (h *Handler) createSession(ctx context.Context, userID string, refreshToken string, ttl time.Duration, userAgent string) (string, error) {
	var sessionID string
	err := h.store.QueryRow(ctx, `
		insert into auth_sessions (user_id, refresh_token_hash, user_agent, expires_at, last_used_at)
		values ($1, $2, $3, now() + ($4 * interval '1 second'), now())
		returning id::text
	`, userID, refreshTokenHash(refreshToken), userAgent, int64(ttl.Seconds())).Scan(&sessionID)
	if err != nil {
		return "", fmt.Errorf("create auth session: %w", err)
	}
	return sessionID, nil
}

func (h *Handler) rotateSession(ctx context.Context, refreshToken string) (User, string, string, error) {
	user, sessionID, err := h.userFromRefreshToken(ctx, refreshToken)
	if err != nil {
		return User{}, "", "", err
	}

	nextRefreshToken, err := newRefreshToken()
	if err != nil {
		return User{}, "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	tag, err := h.store.Exec(ctx, `
		update auth_sessions
		set refresh_token_hash = $1,
		    last_used_at = now(),
		    expires_at = now() + ($4 * interval '1 second')
		where id = $2
		  and refresh_token_hash = $3
		  and revoked_at is null
		  and expires_at > now()
	`, refreshTokenHash(nextRefreshToken), sessionID, refreshTokenHash(refreshToken), int64(h.refreshTokenTTL.Seconds()))
	if err != nil {
		return User{}, "", "", fmt.Errorf("rotate auth session: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return User{}, "", "", pgx.ErrNoRows
	}

	return user, sessionID, nextRefreshToken, nil
}

func (h *Handler) userFromRefreshToken(ctx context.Context, refreshToken string) (User, string, error) {
	var user User
	var sessionID string
	err := h.store.QueryRow(ctx, `
		select s.id::text,
		       u.id::text,
		       u.email,
		       coalesce(u.name, ''),
		       w.id::text,
		       wm.role
		from auth_sessions s
		join users u on u.id = s.user_id
		join workspace_members wm on wm.user_id = u.id
		join workspaces w on w.id = wm.workspace_id
		where s.refresh_token_hash = $1
		  and s.revoked_at is null
		  and s.expires_at > now()
		order by wm.created_at asc
		limit 1
	`, refreshTokenHash(refreshToken)).Scan(
		&sessionID,
		&user.ID,
		&user.Email,
		&user.Name,
		&user.WorkspaceID,
		&user.WorkspaceRole,
	)
	if err != nil {
		return User{}, "", fmt.Errorf("load auth session: %w", err)
	}
	return user, sessionID, nil
}

func (h *Handler) requireActiveSession(ctx context.Context, claims Claims) error {
	if h.store == nil {
		return nil
	}
	if claims.SessionID == "" {
		return ErrTokenInvalid
	}

	var sessionID string
	err := h.store.QueryRow(ctx, `
		select id::text
		from auth_sessions
		where id = $1
		  and user_id = $2
		  and revoked_at is null
		  and expires_at > now()
	`, claims.SessionID, claims.Subject).Scan(&sessionID)
	if err != nil {
		return fmt.Errorf("load active auth session: %w", err)
	}
	return nil
}

func (h *Handler) revokeSessionByRefreshToken(ctx context.Context, refreshToken string) error {
	_, err := h.store.Exec(ctx, `
		update auth_sessions
		set revoked_at = coalesce(revoked_at, now())
		where refresh_token_hash = $1
		  and revoked_at is null
	`, refreshTokenHash(refreshToken))
	return err
}

func (h *Handler) revokeSessionByID(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}

	_, err := h.store.Exec(ctx, `
		update auth_sessions
		set revoked_at = coalesce(revoked_at, now())
		where id = $1
		  and revoked_at is null
	`, sessionID)
	return err
}

func (h *Handler) accessCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     AccessTokenCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

func (h *Handler) refreshCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     RefreshTokenCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

func (h *Handler) csrfCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     CSRFCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: false,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

func newRefreshToken() (string, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(token[:]), nil
}

func newCSRFCookieToken() (string, error) {
	return newRefreshToken()
}

func refreshTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func writeAuthJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeAuthError(w http.ResponseWriter, status int, code string, message string) {
	writeAuthJSON(w, status, map[string]map[string]string{
		"error": {
			"code":    code,
			"message": message,
		},
	})
}
