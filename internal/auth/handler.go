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
	"net/url"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/email"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	trialPromptLimit      = 25
	trialWindow           = 4 * 24 * time.Hour
	magicLinkLifetime     = 15 * time.Minute
	magicLinkLoginPurpose = "login"
	magicLinkVerify       = "verify_email"
)

type UserStore interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type HandlerConfig struct {
	Store            UserStore
	Tokens           *TokenManager
	RefreshTokenTTL  time.Duration
	CookieSecure     bool
	AppBaseURL       string
	APIBaseURL       string
	EmailSender      email.Sender
	EmailRateLimiter *email.RateLimiter
	IPRateLimiter    *IPRateLimiter
}

type Handler struct {
	store            UserStore
	tokens           *TokenManager
	refreshTokenTTL  time.Duration
	cookieSecure     bool
	appBaseURL       string
	apiBaseURL       string
	emailSender      email.Sender
	emailRateLimiter *email.RateLimiter
	ipRateLimiter    *IPRateLimiter
}

type genericEmailRequest struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type loginRequest = genericEmailRequest

type restoreSessionRequest struct {
	Key string `json:"key"`
}

type authResponse struct {
	User      User   `json:"user"`
	ExpiresAt int64  `json:"expiresAt,omitempty"`
	TokenType string `json:"tokenType,omitempty"`
}

type sessionResponse struct {
	Session Session `json:"session"`
}

func NewHandler(cfg HandlerConfig) *Handler {
	refreshTokenTTL := cfg.RefreshTokenTTL
	if refreshTokenTTL <= 0 {
		refreshTokenTTL = 30 * 24 * time.Hour
	}

	return &Handler{
		store:            cfg.Store,
		tokens:           cfg.Tokens,
		refreshTokenTTL:  refreshTokenTTL,
		cookieSecure:     cfg.CookieSecure,
		appBaseURL:       strings.TrimRight(strings.TrimSpace(cfg.AppBaseURL), "/"),
		apiBaseURL:       strings.TrimRight(strings.TrimSpace(cfg.APIBaseURL), "/"),
		emailSender:      cfg.EmailSender,
		emailRateLimiter: cfg.EmailRateLimiter,
		ipRateLimiter:    cfg.IPRateLimiter,
	}
}

func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/magic-link", h.requestMagicLink)
	mux.HandleFunc("POST /api/auth/login", h.requestMagicLink)
	mux.HandleFunc("GET /api/auth/magic", h.consumeMagicLink)
	mux.HandleFunc("POST /api/auth/refresh", h.refresh)
	mux.Handle("GET /api/auth/me", h.RequireUser(http.HandlerFunc(h.me)))
	mux.HandleFunc("POST /api/auth/logout", h.logout)

	mux.HandleFunc("POST /api/sessions/anonymous", h.startAnonymousSession)
	mux.Handle("GET /api/sessions/me", h.RequireSession(http.HandlerFunc(h.sessionMe)))
	mux.HandleFunc("POST /api/sessions/restore", h.restoreSession)
	mux.Handle("POST /api/sessions/recovery-key", h.RequireSession(http.HandlerFunc(h.issueRecoveryKey)))
	mux.Handle("DELETE /api/sessions/recovery-key", h.RequireSession(http.HandlerFunc(h.revokeRecoveryKey)))
	mux.Handle("POST /api/sessions/claim", h.RequireSession(http.HandlerFunc(h.claimSession)))
}

func (h *Handler) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := h.resolveAuthenticatedSession(r)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "authentication is required")
			return
		}

		ctx := WithSession(r.Context(), session)
		ctx = WithUser(ctx, *session.User)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := h.resolveSession(r)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
			return
		}
		if status, code, message, blocked := h.blockedTrialRequest(r, session); blocked {
			writeAuthError(w, status, code, message)
			return
		}

		ctx := WithSession(r.Context(), session)
		if session.User != nil {
			ctx = WithUser(ctx, *session.User)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) blockedTrialRequest(r *http.Request, session Session) (int, string, string, bool) {
	if !session.IsTrial() {
		return 0, "", "", false
	}

	if session.TrialExpired && trialProtectedWrite(r) {
		return http.StatusForbidden, "subscription_required", "your trial has expired; subscribe to keep editing", true
	}
	if generationRoute(r) && session.PromptsUsed >= trialPromptLimit {
		return http.StatusForbidden, "trial_exhausted", "your trial has reached its prompt limit", true
	}
	return 0, "", "", false
}

func trialProtectedWrite(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return !strings.HasPrefix(r.URL.Path, "/api/sessions/") && !strings.HasPrefix(r.URL.Path, "/api/auth/")
}

func generationRoute(r *http.Request) bool {
	return r.URL.Path == "/api/sites/generate" || strings.Contains(r.URL.Path, "/reprompt")
}

func (h *Handler) startAnonymousSession(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAuthError(w, http.StatusServiceUnavailable, "auth_unavailable", "sessions are not configured")
		return
	}

	if session, err := h.resolveSession(r); err == nil {
		freshIfBlocked := r.URL.Query().Get("freshIfBlocked") == "true"
		// Preserve active workspaces, but let a landing-page prompt replace a
		// guest trial that can no longer generate.
		if !freshIfBlocked || !trialGenerationBlocked(session) {
			h.writeSessionCookies(w, r, session)
			writeAuthJSON(w, http.StatusOK, sessionResponse{Session: session})
			return
		}
	}

	session, token, csrfToken, err := h.createTrialSession(r.Context())
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "session_create_failed", "could not start a session")
		return
	}

	http.SetCookie(w, h.guestCookie(token, int(h.refreshTokenTTL.Seconds())))
	http.SetCookie(w, h.csrfCookie(csrfToken, int(h.refreshTokenTTL.Seconds())))
	writeAuthJSON(w, http.StatusCreated, sessionResponse{Session: session})
}

func trialGenerationBlocked(session Session) bool {
	return session.Kind == SessionKindTrial &&
		!session.SubscriptionLive &&
		(session.TrialExpired || session.PromptsUsed >= trialPromptLimit)
}

func (h *Handler) sessionMe(w http.ResponseWriter, r *http.Request) {
	session, ok := SessionFromContext(r.Context())
	if !ok {
		writeAuthError(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	h.writeSessionCookies(w, r, session)
	writeAuthJSON(w, http.StatusOK, sessionResponse{Session: session})
}

func (h *Handler) restoreSession(w http.ResponseWriter, r *http.Request) {
	if !h.ipRateLimiter.Allow(r.Context(), RateLimitPurposeRecoveryRestore, ClientIPFromRequest(r), DefaultRecoveryRestoreRules...) {
		writeAuthError(w, http.StatusTooManyRequests, "rate_limited", "too many recovery attempts; please try again later")
		return
	}
	if h.store == nil {
		writeAuthError(w, http.StatusServiceUnavailable, "auth_unavailable", "sessions are not configured")
		return
	}

	var payload restoreSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	session, token, csrfToken, err := h.loadTrialSessionByRecoveryKey(r.Context(), strings.TrimSpace(payload.Key))
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "invalid_recovery_key", "workspace recovery link is invalid")
		return
	}

	http.SetCookie(w, h.accessCookie("", -1))
	http.SetCookie(w, h.refreshCookie("", -1))
	http.SetCookie(w, h.guestCookie(token, int(h.refreshTokenTTL.Seconds())))
	http.SetCookie(w, h.csrfCookie(csrfToken, int(h.refreshTokenTTL.Seconds())))
	writeAuthJSON(w, http.StatusOK, sessionResponse{Session: session})
}

func (h *Handler) issueRecoveryKey(w http.ResponseWriter, r *http.Request) {
	session, ok := SessionFromContext(r.Context())
	if !ok || !session.IsTrial() {
		writeAuthError(w, http.StatusForbidden, "forbidden", "workspace recovery is only available for trial sessions")
		return
	}
	if session.IsClaimed() {
		writeAuthError(w, http.StatusConflict, "already_claimed", "claimed workspaces use magic-link login instead")
		return
	}

	if !h.ipRateLimiter.Allow(r.Context(), RateLimitPurposeRecoveryIssue, ClientIPFromRequest(r), DefaultRecoveryIssueRules...) {
		writeAuthError(w, http.StatusTooManyRequests, "rate_limited", "too many recovery-key requests; please try again later")
		return
	}

	recoveryKey, err := newRefreshToken()
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "recovery_key_failed", "could not create a recovery link")
		return
	}
	if _, err := h.store.Exec(r.Context(), `
		update guest_sessions
		set recovery_key_hash = $1,
		    last_seen_at = now()
		where id = $2
	`, tokenHash(recoveryKey), session.GuestSessionID); err != nil {
		writeAuthError(w, http.StatusInternalServerError, "recovery_key_failed", "could not create a recovery link")
		return
	}

	writeAuthJSON(w, http.StatusCreated, map[string]any{
		"recoveryUrl": h.recoveryURL(recoveryKey),
	})
}

func (h *Handler) revokeRecoveryKey(w http.ResponseWriter, r *http.Request) {
	session, ok := SessionFromContext(r.Context())
	if !ok || !session.IsTrial() {
		writeAuthError(w, http.StatusForbidden, "forbidden", "workspace recovery is only available for trial sessions")
		return
	}

	if _, err := h.store.Exec(r.Context(), `
		update guest_sessions
		set recovery_key_hash = null,
		    last_seen_at = now()
		where id = $1
	`, session.GuestSessionID); err != nil {
		writeAuthError(w, http.StatusInternalServerError, "recovery_key_failed", "could not revoke the recovery link")
		return
	}

	writeAuthJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) claimSession(w http.ResponseWriter, r *http.Request) {
	session, ok := SessionFromContext(r.Context())
	if !ok || !session.IsTrial() {
		writeAuthError(w, http.StatusForbidden, "forbidden", "only trial sessions can be claimed")
		return
	}

	var payload genericEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	emailAddress := normalizeEmail(payload.Email)
	if emailAddress == "" {
		writeAuthError(w, http.StatusBadRequest, "invalid_email", "email is required")
		return
	}

	user, nextSession, err := h.claimTrialSession(r.Context(), session, emailAddress, strings.TrimSpace(payload.Name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeAuthError(w, http.StatusNotFound, "session_not_found", "trial session was not found")
			return
		}
		writeAuthError(w, http.StatusInternalServerError, "claim_failed", "could not save your workspace")
		return
	}

	if err := h.sendMagicLink(r.Context(), user, magicLinkVerify, "magic_link_verify"); err != nil {
		writeAuthError(w, http.StatusInternalServerError, "claim_failed", "could not save your workspace")
		return
	}

	h.writeSessionCookies(w, r, nextSession)
	writeAuthJSON(w, http.StatusOK, map[string]any{
		"session": nextSession,
		"status":  "magic_link_sent",
	})
}

func (h *Handler) requestMagicLink(w http.ResponseWriter, r *http.Request) {
	if !h.ipRateLimiter.Allow(r.Context(), RateLimitPurposeMagicLinkRequest, ClientIPFromRequest(r), DefaultMagicLinkRequestRules...) {
		writeAuthError(w, http.StatusTooManyRequests, "rate_limited", "too many magic-link requests from this address; please try again later")
		return
	}
	if h.store == nil {
		writeAuthError(w, http.StatusServiceUnavailable, "auth_unavailable", "authentication is not configured")
		return
	}

	var payload genericEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	user, err := h.findUserByEmail(r.Context(), normalizeEmail(payload.Email))
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeAuthError(w, http.StatusInternalServerError, "magic_link_failed", "could not send login email")
		return
	}

	if err == nil {
		if sendErr := h.sendMagicLink(r.Context(), user, magicLinkLoginPurpose, "magic_link_login"); sendErr != nil {
			writeAuthError(w, http.StatusInternalServerError, "magic_link_failed", "could not send login email")
			return
		}
	}

	writeAuthJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "If that email exists, a magic link is on the way.",
	})
}

func (h *Handler) consumeMagicLink(w http.ResponseWriter, r *http.Request) {
	if !h.ipRateLimiter.Allow(r.Context(), RateLimitPurposeMagicLinkVerify, ClientIPFromRequest(r), DefaultMagicLinkVerifyRules...) {
		writeAuthError(w, http.StatusTooManyRequests, "rate_limited", "too many magic-link verification attempts; please try again later")
		return
	}
	if h.store == nil || h.tokens == nil {
		writeAuthError(w, http.StatusServiceUnavailable, "auth_unavailable", "authentication is not configured")
		return
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		writeAuthError(w, http.StatusBadRequest, "invalid_magic_link", "magic link token is required")
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
	user, sessionID, err := h.consumeMagicLinkToken(r.Context(), token, refreshToken, h.refreshTokenTTL, r.UserAgent())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeAuthError(w, http.StatusUnauthorized, "invalid_magic_link", "magic link is invalid or expired")
			return
		}
		writeAuthError(w, http.StatusInternalServerError, "login_failed", "could not sign in")
		return
	}

	tokenValue, claims, err := h.tokens.IssueForSession(user, sessionID)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "login_failed", "could not sign in")
		return
	}

	http.SetCookie(w, h.accessCookie(tokenValue, int(h.tokens.TTL().Seconds())))
	http.SetCookie(w, h.refreshCookie(refreshToken, int(h.refreshTokenTTL.Seconds())))
	http.SetCookie(w, h.guestCookie("", -1))
	http.SetCookie(w, h.csrfCookie(csrfToken, int(h.refreshTokenTTL.Seconds())))

	redirectTarget := strings.TrimSpace(r.URL.Query().Get("redirect"))
	if redirectTarget == "" || !strings.HasPrefix(redirectTarget, "/") || strings.HasPrefix(redirectTarget, "//") {
		redirectTarget = "/app"
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		writeAuthJSON(w, http.StatusOK, authResponse{
			User:      user,
			ExpiresAt: claims.ExpiresAt,
			TokenType: "Bearer",
		})
		return
	}

	http.Redirect(w, r, h.appURL(redirectTarget), http.StatusSeeOther)
}

// login is kept as a compatibility path for existing tests and local fixtures.
// The public route now sends magic links instead.
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

	emailAddress := normalizeEmail(payload.Email)
	if emailAddress == "" {
		writeAuthError(w, http.StatusBadRequest, "invalid_email", "email is required")
		return
	}

	user, err := h.upsertUser(r.Context(), emailAddress, strings.TrimSpace(payload.Name))
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
	http.SetCookie(w, h.guestCookie("", -1))
	http.SetCookie(w, h.csrfCookie("", -1))
	writeAuthJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) resolveSession(r *http.Request) (Session, error) {
	if session, err := h.resolveAuthenticatedSession(r); err == nil {
		return session, nil
	}
	return h.resolveTrialSession(r)
}

func (h *Handler) resolveAuthenticatedSession(r *http.Request) (Session, error) {
	if h.tokens == nil {
		return Session{}, ErrTokenInvalid
	}

	rawToken, err := CookieFromRequest(r)
	if err != nil {
		return Session{}, err
	}
	claims, err := h.tokens.Validate(rawToken)
	if err != nil {
		return Session{}, err
	}
	if err := h.requireActiveSession(r.Context(), claims); err != nil {
		return Session{}, err
	}
	user := UserFromClaims(claims)
	session := Session{
		Kind:          SessionKindAuthenticated,
		WorkspaceID:   user.WorkspaceID,
		WorkspaceRole: user.WorkspaceRole,
		User:          &user,
	}
	h.attachWorkspaceTrialState(r.Context(), &session)
	session.SubscriptionLive = h.lookupSubscriptionLive(r.Context(), user.WorkspaceID)
	return session, nil
}

func (h *Handler) resolveTrialSession(r *http.Request) (Session, error) {
	if h.store == nil {
		return Session{}, ErrTokenInvalid
	}
	cookie, err := r.Cookie(GuestSessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return Session{}, ErrTokenInvalid
	}
	return h.loadTrialSessionByCookie(r.Context(), cookie.Value)
}

func (h *Handler) createTrialSession(ctx context.Context) (Session, string, string, error) {
	cookieToken, err := newRefreshToken()
	if err != nil {
		return Session{}, "", "", err
	}
	csrfToken, err := newCSRFCookieToken()
	if err != nil {
		return Session{}, "", "", err
	}

	tx, err := beginStoreTx(ctx, h.store)
	if err != nil {
		return Session{}, "", "", fmt.Errorf("begin trial session tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var workspaceID string
	if err := tx.QueryRow(ctx, `
		insert into workspaces (name)
		values ('Trial Workspace')
		returning id::text
	`).Scan(&workspaceID); err != nil {
		return Session{}, "", "", fmt.Errorf("create trial workspace: %w", err)
	}

	var sessionID string
	var trialStartedAt time.Time
	var trialExpiresAt time.Time
	if err := tx.QueryRow(ctx, `
		insert into guest_sessions (workspace_id, cookie_token_hash)
		values ($1, $2)
		returning id::text, trial_started_at, trial_expires_at
	`, workspaceID, tokenHash(cookieToken)).Scan(&sessionID, &trialStartedAt, &trialExpiresAt); err != nil {
		return Session{}, "", "", fmt.Errorf("create guest session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Session{}, "", "", fmt.Errorf("commit trial session: %w", err)
	}

	return Session{
		Kind:           SessionKindTrial,
		WorkspaceID:    workspaceID,
		WorkspaceRole:  "owner",
		GuestSessionID: sessionID,
		PromptsUsed:    0,
		PromptLimit:    trialPromptLimit,
		TrialStartedAt: &trialStartedAt,
		TrialExpiresAt: &trialExpiresAt,
	}, cookieToken, csrfToken, nil
}

func (h *Handler) loadTrialSessionByCookie(ctx context.Context, token string) (Session, error) {
	return h.loadTrialSession(ctx, "cookie_token_hash", tokenHash(token))
}

func (h *Handler) loadTrialSessionByRecoveryKey(ctx context.Context, recoveryKey string) (Session, string, string, error) {
	if strings.TrimSpace(recoveryKey) == "" {
		return Session{}, "", "", ErrTokenInvalid
	}
	session, err := h.loadTrialSession(ctx, "recovery_key_hash", tokenHash(recoveryKey))
	if err != nil {
		return Session{}, "", "", err
	}
	cookieToken, err := newRefreshToken()
	if err != nil {
		return Session{}, "", "", err
	}
	if _, err := h.store.Exec(ctx, `
		update guest_sessions
		set cookie_token_hash = $1,
		    last_seen_at = now()
		where id = $2
	`, tokenHash(cookieToken), session.GuestSessionID); err != nil {
		return Session{}, "", "", err
	}
	csrfToken, err := newCSRFCookieToken()
	if err != nil {
		return Session{}, "", "", err
	}
	return session, cookieToken, csrfToken, nil
}

func (h *Handler) loadTrialSession(ctx context.Context, column string, hash string) (Session, error) {
	query := fmt.Sprintf(`
		select gs.id::text,
		       gs.workspace_id::text,
		       gs.prompts_used,
		       gs.trial_started_at,
		       gs.trial_expires_at,
		       gs.claimed_at,
		       coalesce(gs.claimed_by_user_id::text, ''),
		       (gs.recovery_key_hash is not null) as has_recovery_key,
		       coalesce(u.id::text, ''),
		       coalesce(u.email, ''),
		       coalesce(u.name, '')
		from guest_sessions gs
		left join users u on u.id = gs.claimed_by_user_id
		where gs.%s = $1
	`, column)

	var (
		session        Session
		claimedAt      *time.Time
		userID         string
		userEmail      string
		userName       string
		trialStartedAt time.Time
		trialExpiresAt time.Time
	)
	err := h.store.QueryRow(ctx, query, hash).Scan(
		&session.GuestSessionID,
		&session.WorkspaceID,
		&session.PromptsUsed,
		&trialStartedAt,
		&trialExpiresAt,
		&claimedAt,
		&session.ClaimedByUserID,
		&session.HasRecoveryKey,
		&userID,
		&userEmail,
		&userName,
	)
	if err != nil {
		return Session{}, err
	}

	session.Kind = SessionKindTrial
	session.WorkspaceRole = "owner"
	session.PromptLimit = trialPromptLimit
	session.TrialStartedAt = &trialStartedAt
	session.TrialExpiresAt = &trialExpiresAt
	session.TrialExpired = time.Now().UTC().After(trialExpiresAt)
	session.ClaimedAt = claimedAt
	session.SubscriptionLive = h.lookupSubscriptionLive(ctx, session.WorkspaceID)
	if userID != "" {
		user := User{
			ID:            userID,
			Email:         userEmail,
			Name:          userName,
			WorkspaceID:   session.WorkspaceID,
			WorkspaceRole: "owner",
		}
		session.User = &user
	}

	_, _ = h.store.Exec(ctx, `
		update guest_sessions
		set last_seen_at = now()
		where id = $1
	`, session.GuestSessionID)

	return session, nil
}

func (h *Handler) attachWorkspaceTrialState(ctx context.Context, session *Session) {
	if h.store == nil || session == nil || session.WorkspaceID == "" {
		return
	}

	var (
		guestSessionID  string
		promptsUsed     int
		trialStartedAt  time.Time
		trialExpiresAt  time.Time
		claimedAt       *time.Time
		claimedByUserID string
		hasRecoveryKey  bool
	)
	err := h.store.QueryRow(ctx, `
		select id::text,
		       prompts_used,
		       trial_started_at,
		       trial_expires_at,
		       claimed_at,
		       coalesce(claimed_by_user_id::text, ''),
		       (recovery_key_hash is not null) as has_recovery_key
		from guest_sessions
		where workspace_id = $1
		order by created_at asc
		limit 1
	`, session.WorkspaceID).Scan(
		&guestSessionID,
		&promptsUsed,
		&trialStartedAt,
		&trialExpiresAt,
		&claimedAt,
		&claimedByUserID,
		&hasRecoveryKey,
	)
	if err != nil {
		return
	}
	session.GuestSessionID = guestSessionID
	session.PromptsUsed = promptsUsed
	session.PromptLimit = trialPromptLimit
	session.TrialStartedAt = &trialStartedAt
	session.TrialExpiresAt = &trialExpiresAt
	session.TrialExpired = time.Now().UTC().After(trialExpiresAt)
	session.ClaimedAt = claimedAt
	session.ClaimedByUserID = claimedByUserID
	session.HasRecoveryKey = hasRecoveryKey
	session.SubscriptionLive = h.lookupSubscriptionLive(ctx, session.WorkspaceID)
}

func (h *Handler) lookupSubscriptionLive(ctx context.Context, workspaceID string) bool {
	if h.store == nil || strings.TrimSpace(workspaceID) == "" {
		return false
	}
	var subscriptionLive bool
	if err := h.store.QueryRow(ctx, `
		select subscription_live
		from billing_entitlements
		where workspace_id = $1
	`, workspaceID).Scan(&subscriptionLive); err != nil {
		return false
	}
	return subscriptionLive
}

func (h *Handler) claimTrialSession(ctx context.Context, session Session, emailAddress string, name string) (User, Session, error) {
	tx, err := beginStoreTx(ctx, h.store)
	if err != nil {
		return User{}, Session{}, err
	}
	defer tx.Rollback(ctx)

	var user User
	if err := tx.QueryRow(ctx, `
		insert into users (email, name)
		values ($1, nullif($2, ''))
		on conflict (email) do update
		set name = coalesce(nullif(excluded.name, ''), users.name),
		    updated_at = now()
		returning id::text, email, coalesce(name, '')
	`, emailAddress, name).Scan(&user.ID, &user.Email, &user.Name); err != nil {
		return User{}, Session{}, fmt.Errorf("upsert claim user: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		insert into workspace_members (workspace_id, user_id, role)
		values ($1, $2, 'owner')
		on conflict (workspace_id, user_id) do update
		set role = excluded.role
	`, session.WorkspaceID, user.ID); err != nil {
		return User{}, Session{}, fmt.Errorf("attach claim user workspace: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		update workspaces
		set created_by = coalesce(created_by, $2),
		    updated_at = now()
		where id = $1
	`, session.WorkspaceID, user.ID); err != nil {
		return User{}, Session{}, fmt.Errorf("set workspace owner: %w", err)
	}

	var claimedAt time.Time
	if err := tx.QueryRow(ctx, `
		update guest_sessions
		set claimed_by_user_id = $1,
		    claimed_at = coalesce(claimed_at, now()),
		    recovery_key_hash = null,
		    last_seen_at = now()
		where id = $2
		returning claimed_at
	`, user.ID, session.GuestSessionID).Scan(&claimedAt); err != nil {
		return User{}, Session{}, fmt.Errorf("claim guest session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, Session{}, err
	}

	user.WorkspaceID = session.WorkspaceID
	user.WorkspaceRole = "owner"
	session.User = &user
	session.ClaimedByUserID = user.ID
	session.ClaimedAt = &claimedAt
	session.HasRecoveryKey = false
	return user, session, nil
}

func (h *Handler) findUserByEmail(ctx context.Context, emailAddress string) (User, error) {
	if emailAddress == "" {
		return User{}, pgx.ErrNoRows
	}

	var user User
	err := h.store.QueryRow(ctx, `
		select u.id::text,
		       u.email,
		       coalesce(u.name, ''),
		       w.id::text,
		       wm.role
		from users u
		join workspace_members wm on wm.user_id = u.id
		join workspaces w on w.id = wm.workspace_id
		where u.email = $1
		order by wm.created_at asc
		limit 1
	`, emailAddress).Scan(&user.ID, &user.Email, &user.Name, &user.WorkspaceID, &user.WorkspaceRole)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (h *Handler) sendMagicLink(ctx context.Context, user User, purpose string, templateName string) error {
	if h.emailSender.Mailer == nil {
		return fmt.Errorf("email sender is not configured")
	}

	if h.emailRateLimiter != nil {
		allowed, err := h.emailRateLimiter.Allow(ctx, user.Email, templateName,
			email.RateLimitRule{Limit: 5, Window: 15 * time.Minute},
			email.RateLimitRule{Limit: 20, Window: 24 * time.Hour},
		)
		if err != nil {
			return err
		}
		if !allowed {
			return email.ErrRateLimited
		}
	}

	token, err := newRefreshToken()
	if err != nil {
		return err
	}
	expiresAt := time.Now().UTC().Add(magicLinkLifetime)
	if err := h.store.QueryRow(ctx, `
		insert into magic_links (user_id, token_hash, purpose, expires_at)
		values ($1, $2, $3, $4)
		returning id::text
	`, user.ID, tokenHash(token), purpose, expiresAt).Scan(new(string)); err != nil {
		return fmt.Errorf("create magic link: %w", err)
	}

	linkURL := h.magicLinkURL(token)
	templateData := email.MagicLinkTemplateData{
		ProductName: "Snaelda",
		ActionLabel: map[bool]string{true: "Verify email", false: "Log in"}[purpose == magicLinkVerify],
		Email:       user.Email,
		MagicURL:    linkURL,
		ExpiresIn:   "15 minutes",
	}

	var sendErr error
	switch purpose {
	case magicLinkVerify:
		_, sendErr = h.emailSender.SendMagicLinkVerify(ctx, email.Address{Email: user.Email, Name: user.Name}, templateData)
	default:
		_, sendErr = h.emailSender.SendMagicLinkLogin(ctx, email.Address{Email: user.Email, Name: user.Name}, templateData)
	}
	return sendErr
}

func (h *Handler) consumeMagicLinkToken(ctx context.Context, token string, refreshToken string, ttl time.Duration, userAgent string) (User, string, error) {
	tx, err := beginStoreTx(ctx, h.store)
	if err != nil {
		return User{}, "", err
	}
	defer tx.Rollback(ctx)

	var (
		user        User
		magicLinkID string
	)
	err = tx.QueryRow(ctx, `
		select ml.id::text,
		       u.id::text,
		       u.email,
		       coalesce(u.name, ''),
		       w.id::text,
		       wm.role
		from magic_links ml
		join users u on u.id = ml.user_id
		join workspace_members wm on wm.user_id = u.id
		join workspaces w on w.id = wm.workspace_id
		where ml.token_hash = $1
		  and ml.consumed_at is null
		  and ml.expires_at > now()
		  and ml.purpose in ($2, $3)
		order by wm.created_at asc
		limit 1
		for update
	`, tokenHash(token), magicLinkLoginPurpose, magicLinkVerify).Scan(&magicLinkID, &user.ID, &user.Email, &user.Name, &user.WorkspaceID, &user.WorkspaceRole)
	if err != nil {
		return User{}, "", err
	}

	tag, err := tx.Exec(ctx, `
		update magic_links
		set consumed_at = now()
		where id = $1
		  and consumed_at is null
	`, magicLinkID)
	if err != nil {
		return User{}, "", err
	}
	if tag.RowsAffected() != 1 {
		return User{}, "", pgx.ErrNoRows
	}

	sessionID, err := createSessionWithStore(ctx, tx, user.ID, refreshToken, ttl, userAgent)
	if err != nil {
		return User{}, "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, "", err
	}
	return user, sessionID, nil
}

func normalizeEmail(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || !strings.Contains(value, "@") {
		return ""
	}
	return value
}

func (h *Handler) appURL(path string) string {
	if h.appBaseURL == "" {
		return path
	}
	return h.appBaseURL + path
}

func (h *Handler) magicLinkURL(token string) string {
	base := h.apiBaseURL
	if base == "" {
		base = h.appBaseURL
	}
	if base == "" {
		return "/api/auth/magic?token=" + url.QueryEscape(token)
	}
	return base + "/api/auth/magic?token=" + url.QueryEscape(token) + "&redirect=" + url.QueryEscape("/app")
}

func (h *Handler) recoveryURL(token string) string {
	return h.appURL("/restore?k=" + url.QueryEscape(token))
}

// writeSessionCookies refreshes the cookies that back an active trial session
// so the rolling TTL keeps pace with usage. For authenticated sessions the
// access/refresh cookies are already rotated on /api/auth/refresh, so this is a
// no-op there.
func (h *Handler) writeSessionCookies(w http.ResponseWriter, r *http.Request, session Session) {
	if !session.IsTrial() {
		return
	}
	guest, err := r.Cookie(GuestSessionCookieName)
	if err != nil || strings.TrimSpace(guest.Value) == "" {
		return
	}
	maxAge := int(h.refreshTokenTTL.Seconds())
	http.SetCookie(w, h.guestCookie(guest.Value, maxAge))
	if csrf, err := r.Cookie(CSRFCookieName); err == nil && strings.TrimSpace(csrf.Value) != "" {
		http.SetCookie(w, h.csrfCookie(csrf.Value, maxAge))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
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
	return createSessionWithStore(ctx, h.store, userID, refreshToken, ttl, userAgent)
}

type sessionWriter interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func createSessionWithStore(ctx context.Context, store sessionWriter, userID string, refreshToken string, ttl time.Duration, userAgent string) (string, error) {
	var sessionID string
	err := store.QueryRow(ctx, `
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

func (h *Handler) guestCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     GuestSessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

func (h *Handler) csrfCookie(value string, maxAge int) *http.Cookie {
	cookie := &http.Cookie{
		Name:     CSRFCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: false,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
	if domain := h.csrfCookieDomain(); domain != "" {
		cookie.Domain = domain
	}
	return cookie
}

func (h *Handler) csrfCookieDomain() string {
	appURL, err := url.Parse(strings.TrimSpace(h.appBaseURL))
	if err != nil {
		return ""
	}
	hostname := strings.ToLower(appURL.Hostname())
	if hostname == "" || hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return ""
	}
	if strings.Contains(hostname, ".") {
		return strings.TrimPrefix(hostname, "www.")
	}
	return ""
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

func tokenHash(token string) string {
	return refreshTokenHash(token)
}

type txBeginner interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

func beginStoreTx(ctx context.Context, store UserStore) (pgx.Tx, error) {
	beginner, ok := store.(txBeginner)
	if !ok {
		return nil, fmt.Errorf("transaction support is not configured")
	}
	return beginner.BeginTx(ctx, pgx.TxOptions{})
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
