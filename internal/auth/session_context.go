package auth

import (
	"context"
	"time"
)

const GuestSessionCookieName = "snaelda_guest_session"

type SessionKind string

const (
	SessionKindAuthenticated SessionKind = "authenticated"
	SessionKindTrial         SessionKind = "trial"
)

type Session struct {
	Kind             SessionKind `json:"kind"`
	WorkspaceID      string      `json:"workspaceId"`
	WorkspaceRole    string      `json:"workspaceRole"`
	WorkspaceLocale  string      `json:"workspaceLocale,omitempty"`
	IsOperator       bool        `json:"isOperator,omitempty"`
	User             *User       `json:"user,omitempty"`
	GuestSessionID   string      `json:"guestSessionId,omitempty"`
	PromptsUsed      int         `json:"promptsUsed,omitempty"`
	PromptLimit      int         `json:"promptLimit,omitempty"`
	TrialStartedAt   *time.Time  `json:"trialStartedAt,omitempty"`
	TrialExpiresAt   *time.Time  `json:"trialExpiresAt,omitempty"`
	TrialExpired     bool        `json:"trialExpired,omitempty"`
	ClaimedAt        *time.Time  `json:"claimedAt,omitempty"`
	ClaimedByUserID  string      `json:"claimedByUserId,omitempty"`
	HasRecoveryKey   bool        `json:"hasRecoveryKey,omitempty"`
	SubscriptionLive bool        `json:"subscriptionLive,omitempty"`
}

func (s Session) IsAuthenticated() bool {
	return s.Kind == SessionKindAuthenticated && s.User != nil && s.User.ID != ""
}

func (s Session) IsTrial() bool {
	return !s.SubscriptionLive && s.TrialStartedAt != nil
}

func (s Session) IsClaimed() bool {
	return s.ClaimedByUserID != ""
}

// HasClaimedIdentity reports whether an email-backed identity is attached to the
// session: either an authenticated (logged-in) user or a claimed trial whose
// owner added an email. This is the Spec 17 L2 gate the re-spin publish guard
// enforces before a draft scraped from a third-party site can be published
// (Spec 21).
func (s Session) HasClaimedIdentity() bool {
	return s.IsAuthenticated() || s.IsClaimed()
}

type sessionContextKey string

const sessionContextKeyValue sessionContextKey = "auth.session"

func WithSession(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, sessionContextKeyValue, session)
}

func SessionFromContext(ctx context.Context) (Session, bool) {
	session, ok := ctx.Value(sessionContextKeyValue).(Session)
	return session, ok
}
