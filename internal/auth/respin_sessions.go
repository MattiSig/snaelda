package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// RespinDemoSession is the detached trial workspace + guest session created up
// front for an unclaimed re-spin demo import (Spec 21). No browser cookie is set
// at creation: the demo requires no signup, and the session becomes cookie-bound
// only when the visitor claims the result via AdoptRespinDemoSession.
type RespinDemoSession struct {
	WorkspaceID    string
	GuestSessionID string
	Locale         string
}

// StartRespinDemoSession creates a trial workspace and guest session that holds a
// re-spin demo's generated draft and ingested assets before the visitor commits.
// The returned ids are recorded on the respin import; the underlying guest
// session carries an unused cookie token hash so it cannot be resolved from the
// browser until the claim step rotates it in.
func (h *Handler) StartRespinDemoSession(ctx context.Context, locale string) (RespinDemoSession, error) {
	if h.store == nil {
		return RespinDemoSession{}, fmt.Errorf("auth store is not configured")
	}
	session, _, _, err := h.createTrialSession(ctx, locale)
	if err != nil {
		return RespinDemoSession{}, err
	}
	return RespinDemoSession{
		WorkspaceID:    session.WorkspaceID,
		GuestSessionID: session.GuestSessionID,
		Locale:         session.WorkspaceLocale,
	}, nil
}

// AdoptRespinDemoSession binds an up-front demo session to the visitor's browser
// on claim: it rotates the guest session's cookie token, writes the trial
// cookies, and returns the resolved Session. This is the "create (or reuse) an
// L0 cookie-bound trial session" step of the Spec 21 claim handoff — reuse,
// because the workspace and generated draft already exist.
func (h *Handler) AdoptRespinDemoSession(w http.ResponseWriter, r *http.Request, guestSessionID string) (Session, error) {
	if h.store == nil {
		return Session{}, fmt.Errorf("auth store is not configured")
	}
	guestSessionID = strings.TrimSpace(guestSessionID)
	if guestSessionID == "" {
		return Session{}, ErrTokenInvalid
	}

	cookieToken, err := newRefreshToken()
	if err != nil {
		return Session{}, err
	}
	csrfToken, err := newCSRFCookieToken()
	if err != nil {
		return Session{}, err
	}

	tag, err := h.store.Exec(r.Context(), `
		update guest_sessions
		set cookie_token_hash = $1,
		    last_seen_at = now()
		where id = $2
	`, tokenHash(cookieToken), guestSessionID)
	if err != nil {
		return Session{}, fmt.Errorf("adopt respin demo session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Session{}, ErrTokenInvalid
	}

	session, err := h.loadTrialSessionByCookie(r.Context(), cookieToken)
	if err != nil {
		return Session{}, err
	}

	maxAge := int(h.refreshTokenTTL.Seconds())
	// Adopting a demo session is an explicit identity choice; clear any stale
	// signed-in cookies so a later background refresh cannot resurrect a prior
	// identity over this trial, matching the fresh-trial and restore paths.
	http.SetCookie(w, h.accessCookie("", -1))
	http.SetCookie(w, h.refreshCookie("", -1))
	http.SetCookie(w, h.guestCookie(cookieToken, maxAge))
	http.SetCookie(w, h.csrfCookie(csrfToken, maxAge))
	return session, nil
}
