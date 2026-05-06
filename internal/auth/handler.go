package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type UserStore interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type HandlerConfig struct {
	Store        UserStore
	Tokens       *TokenManager
	CookieSecure bool
}

type Handler struct {
	store        UserStore
	tokens       *TokenManager
	cookieSecure bool
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
	return &Handler{
		store:        cfg.Store,
		tokens:       cfg.Tokens,
		cookieSecure: cfg.CookieSecure,
	}
}

func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/login", h.login)
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

	token, claims, err := h.tokens.Issue(user)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "login_failed", "could not sign in")
		return
	}

	http.SetCookie(w, h.accessCookie(token, int(h.tokens.TTL().Seconds())))
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

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, h.accessCookie("", -1))
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
