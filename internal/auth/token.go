package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	AccessTokenCookieName  = "snaelda_access_token"
	RefreshTokenCookieName = "snaelda_refresh_token"
	CSRFCookieName         = "snaelda_csrf_token"
)

var (
	ErrTokenMalformed = errors.New("token is malformed")
	ErrTokenInvalid   = errors.New("token is invalid")
	ErrTokenExpired   = errors.New("token is expired")
)

type TokenConfig struct {
	Secret   string
	Issuer   string
	Audience string
	TTL      time.Duration
}

type User struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	WorkspaceID   string `json:"workspaceId"`
	WorkspaceRole string `json:"workspaceRole"`
}

type Claims struct {
	Issuer        string `json:"iss"`
	Audience      string `json:"aud"`
	Subject       string `json:"sub"`
	SessionID     string `json:"sid,omitempty"`
	Email         string `json:"email"`
	Name          string `json:"name,omitempty"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceRole string `json:"workspace_role"`
	IssuedAt      int64  `json:"iat"`
	NotBefore     int64  `json:"nbf"`
	ExpiresAt     int64  `json:"exp"`
}

type TokenManager struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
	now      func() time.Time
}

func NewTokenManager(cfg TokenConfig) (*TokenManager, error) {
	if cfg.Secret == "" {
		return nil, fmt.Errorf("auth token secret is required")
	}
	if cfg.Issuer == "" {
		return nil, fmt.Errorf("auth token issuer is required")
	}
	if cfg.Audience == "" {
		return nil, fmt.Errorf("auth token audience is required")
	}
	if cfg.TTL <= 0 {
		return nil, fmt.Errorf("auth token ttl must be positive")
	}

	return &TokenManager{
		secret:   []byte(cfg.Secret),
		issuer:   cfg.Issuer,
		audience: cfg.Audience,
		ttl:      cfg.TTL,
		now:      time.Now,
	}, nil
}

func (m *TokenManager) TTL() time.Duration {
	return m.ttl
}

func (m *TokenManager) Issue(user User) (string, Claims, error) {
	return m.IssueForSession(user, "")
}

func (m *TokenManager) IssueForSession(user User, sessionID string) (string, Claims, error) {
	now := m.now().UTC()
	claims := Claims{
		Issuer:        m.issuer,
		Audience:      m.audience,
		Subject:       user.ID,
		SessionID:     sessionID,
		Email:         user.Email,
		Name:          user.Name,
		WorkspaceID:   user.WorkspaceID,
		WorkspaceRole: user.WorkspaceRole,
		IssuedAt:      now.Unix(),
		NotBefore:     now.Unix(),
		ExpiresAt:     now.Add(m.ttl).Unix(),
	}

	token, err := m.sign(claims)
	return token, claims, err
}

func (m *TokenManager) Validate(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrTokenMalformed
	}

	signingInput := parts[0] + "." + parts[1]
	expected := signHMAC([]byte(signingInput), m.secret)
	actual, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrTokenMalformed
	}
	if !hmac.Equal(actual, expected) {
		return Claims{}, ErrTokenInvalid
	}

	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if err := decodeSegment(parts[0], &header); err != nil {
		return Claims{}, ErrTokenMalformed
	}
	if header.Algorithm != "HS256" || header.Type != "JWT" {
		return Claims{}, ErrTokenInvalid
	}

	var claims Claims
	if err := decodeSegment(parts[1], &claims); err != nil {
		return Claims{}, ErrTokenMalformed
	}
	if claims.Issuer != m.issuer || claims.Audience != m.audience {
		return Claims{}, ErrTokenInvalid
	}
	if claims.Subject == "" || claims.Email == "" || claims.WorkspaceID == "" || claims.WorkspaceRole == "" {
		return Claims{}, ErrTokenInvalid
	}

	now := m.now().UTC().Unix()
	if claims.NotBefore > now {
		return Claims{}, ErrTokenInvalid
	}
	if claims.ExpiresAt <= now {
		return Claims{}, ErrTokenExpired
	}

	return claims, nil
}

func (m *TokenManager) sign(claims Claims) (string, error) {
	header, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	signature := signHMAC([]byte(signingInput), m.secret)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func signHMAC(message []byte, secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(message)
	return mac.Sum(nil)
}

func decodeSegment(segment string, dest any) error {
	data, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

type contextKey string

const userContextKey contextKey = "auth.user"

func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(userContextKey).(User)
	return user, ok
}

func UserFromClaims(claims Claims) User {
	return User{
		ID:            claims.Subject,
		Email:         claims.Email,
		Name:          claims.Name,
		WorkspaceID:   claims.WorkspaceID,
		WorkspaceRole: claims.WorkspaceRole,
	}
}

func CookieFromRequest(r *http.Request) (string, error) {
	cookie, err := r.Cookie(AccessTokenCookieName)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

func RefreshCookieFromRequest(r *http.Request) (string, error) {
	cookie, err := r.Cookie(RefreshTokenCookieName)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

func CSRFCookieFromRequest(r *http.Request) (string, error) {
	cookie, err := r.Cookie(CSRFCookieName)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}
