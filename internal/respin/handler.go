package respin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/billing"
	"github.com/MattiSig/snaelda/internal/sites"
)

// DefaultCacheTTL is the per-normalized-URL result-cache window (Spec 21): a
// link that travels in a Facebook group serves the cached import rather than
// re-running fetch + generation.
const DefaultCacheTTL = 24 * time.Hour

// RespinDemoStartRules bound the expensive-unauthenticated-compute tier: a few
// starts per hour with a small daily cap, durable per Spec 12's rate-limit
// posture.
var RespinDemoStartRules = []auth.IPRateLimitRule{
	{Limit: 3, Window: time.Hour},
	{Limit: 10, Window: 24 * time.Hour},
}

// DemoSessionManager is the auth seam for the demo trial workspace + claim
// handoff (Spec 21). *auth.Handler satisfies it.
type DemoSessionManager interface {
	StartRespinDemoSession(ctx context.Context, locale string) (auth.RespinDemoSession, error)
	AdoptRespinDemoSession(w http.ResponseWriter, r *http.Request, guestSessionID string) (auth.Session, error)
}

// IPLimiter is the durable per-IP rate limiter seam. *auth.IPRateLimiter
// satisfies it.
type IPLimiter interface {
	Allow(ctx context.Context, purpose string, ip string, rules ...auth.IPRateLimitRule) bool
}

// PreviewIssuer mints demo-scoped preview tokens for the generated "after"
// draft. *sites.PostgresPreviewTokenService satisfies it.
type PreviewIssuer interface {
	Issue(ctx context.Context, siteID string, userID string) (sites.PreviewToken, error)
}

// HandlerConfig wires the re-spin endpoints.
type HandlerConfig struct {
	Store           handlerStore
	Runner          *Runner
	Fetcher         *Fetcher
	Previews        PreviewIssuer
	Sessions        DemoSessionManager
	IPLimiter       IPLimiter
	Budget          *Budget // public demo daily LLM budget; nil disables gating
	BillingStore    billing.AccessStore
	PublicPipeline  *Pipeline // budgeted analyzer, for the unauthenticated demo
	SessionPipeline *Pipeline // unbudgeted analyzer, for session-bound re-spins
	CacheTTL        time.Duration
	Logger          *slog.Logger
	clock           func() time.Time
}

// Handler serves the re-spin URL-import endpoints: the public before/after demo
// (no session) and the session-bound re-spin into an existing workspace.
type Handler struct {
	store           handlerStore
	runner          *Runner
	fetcher         *Fetcher
	previews        PreviewIssuer
	sessions        DemoSessionManager
	ipLimiter       IPLimiter
	budget          *Budget
	billing         billing.AccessStore
	publicPipeline  *Pipeline
	sessionPipeline *Pipeline
	cacheTTL        time.Duration
	logger          *slog.Logger
	clock           func() time.Time
}

// NewHandler builds the re-spin HTTP handler.
func NewHandler(cfg HandlerConfig) *Handler {
	cacheTTL := cfg.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = DefaultCacheTTL
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	clock := cfg.clock
	if clock == nil {
		clock = time.Now
	}
	return &Handler{
		store:           cfg.Store,
		runner:          cfg.Runner,
		fetcher:         cfg.Fetcher,
		previews:        cfg.Previews,
		sessions:        cfg.Sessions,
		ipLimiter:       cfg.IPLimiter,
		budget:          cfg.Budget,
		billing:         cfg.BillingStore,
		publicPipeline:  cfg.PublicPipeline,
		sessionPipeline: cfg.SessionPipeline,
		cacheTTL:        cacheTTL,
		logger:          logger,
		clock:           clock,
	}
}

// Mount registers the re-spin routes. The public demo routes carry no session;
// the session-bound re-spin is wrapped in requireUser.
func (h *Handler) Mount(mux *http.ServeMux, requireUser func(http.Handler) http.Handler) {
	// Import-scoped routes live under a literal /imports/ collection so the
	// {importId} wildcard cannot collide with the literal /share/ route under
	// net/http's pattern precedence (both second segments stay literal).
	mux.Handle("POST /api/respin", http.HandlerFunc(h.startDemo))
	mux.Handle("GET /api/respin/imports/{importId}", http.HandlerFunc(h.status))
	mux.Handle("GET /api/respin/imports/{importId}/preview", http.HandlerFunc(h.preview))
	mux.Handle("POST /api/respin/imports/{importId}/claim", http.HandlerFunc(h.claim))
	mux.Handle("GET /api/respin/share/{shareSlug}", http.HandlerFunc(h.share))
	mux.Handle("POST /api/sites/respin", requireUser(http.HandlerFunc(h.sessionRespin)))
}

type startRequest struct {
	URL    string `json:"url"`
	Locale string `json:"locale,omitempty"`
}

type startResponse struct {
	ImportID  string `json:"importId"`
	Status    string `json:"status"`
	ShareSlug string `json:"shareSlug,omitempty"`
	Cached    bool   `json:"cached,omitempty"`
}

func (h *Handler) startDemo(w http.ResponseWriter, r *http.Request) {
	if h.store == nil || h.runner == nil || h.sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "respin_unavailable", "re-spin is not configured")
		return
	}

	var payload startRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}
	normalized, err := NormalizeURL(payload.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_url", "a valid website URL is required")
		return
	}

	// Per-IP abuse limit on demo starts (durable, Spec 12/21).
	if h.ipLimiter != nil && !h.ipLimiter.Allow(r.Context(), "respin_demo_start", auth.ClientIPFromRequest(r), RespinDemoStartRules...) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many re-spins from your network; please try again shortly")
		return
	}

	// SSRF pre-check before spending a slot (the dial-time guard remains
	// authoritative on every connection).
	if err := h.fetcher.ValidatePublicURL(r.Context(), normalized); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_url", "that URL can't be fetched")
		return
	}

	// Daily public LLM budget: pause new demo starts when exhausted; session-bound
	// flows are unaffected.
	if h.budget != nil {
		if err := h.budget.Check(r.Context()); err != nil {
			if errors.Is(err, ErrBudgetExhausted) {
				writeError(w, http.StatusServiceUnavailable, "respin_busy", "re-spin is at capacity for today; please try again tomorrow")
				return
			}
			h.logger.Warn("respin budget check", "error", err.Error())
		}
	}

	// Result cache: repeated pastes of the same URL serve the cached import.
	if cached, err := h.store.FindCached(r.Context(), normalized, h.clock().Add(-h.cacheTTL)); err == nil {
		writeJSON(w, http.StatusOK, startResponse{
			ImportID:  cached.ID,
			Status:    cached.FetchStatus,
			ShareSlug: cached.ShareSlug,
			Cached:    true,
		})
		return
	} else if !errors.Is(err, ErrNotFound) {
		h.logger.Warn("respin cache lookup", "error", err.Error())
	}

	// Reserve a concurrency slot before creating any workspace/import.
	slot, ok := h.runner.TryAcquire()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "respin_busy", "re-spin is busy right now; please try again shortly")
		return
	}

	demo, err := h.sessions.StartRespinDemoSession(r.Context(), payload.Locale)
	if err != nil {
		slot.Release()
		h.logger.Error("respin start demo session", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "respin_failed", "could not start a re-spin")
		return
	}

	imp, err := h.store.Create(r.Context(), CreateInput{
		SourceURL:      payload.URL,
		NormalizedURL:  normalized,
		GuestSessionID: demo.GuestSessionID,
	})
	if err != nil {
		slot.Release()
		h.logger.Error("respin create import", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "respin_failed", "could not start a re-spin")
		return
	}

	params := RunParams{
		ImportID:         imp.ID,
		WorkspaceID:      demo.WorkspaceID,
		SourceURL:        normalized,
		LanguageOverride: strings.TrimSpace(payload.Locale),
	}
	pipeline := h.publicPipeline
	h.runner.Start(imp.ID, func(ctx context.Context, sink ProgressSink) (RunResult, error) {
		return pipeline.Run(ctx, params, sink)
	}, slot)

	writeJSON(w, http.StatusCreated, startResponse{ImportID: imp.ID, Status: StatusQueued})
}

func (h *Handler) sessionRespin(w http.ResponseWriter, r *http.Request) {
	if h.store == nil || h.runner == nil || h.sessionPipeline == nil {
		writeError(w, http.StatusServiceUnavailable, "respin_unavailable", "re-spin is not configured")
		return
	}
	session, ok := auth.SessionFromContext(r.Context())
	if !ok || strings.TrimSpace(session.WorkspaceID) == "" {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}

	if h.billing != nil {
		if err := billing.EnforceSiteLimit(r.Context(), h.billing, session.WorkspaceID); err != nil {
			h.writeBillingError(w, err)
			return
		}
		if err := billing.EnforcePromptLimit(r.Context(), h.billing, session.WorkspaceID); err != nil {
			h.writeBillingError(w, err)
			return
		}
	}

	var payload startRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}
	normalized, err := NormalizeURL(payload.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_url", "a valid website URL is required")
		return
	}
	if err := h.fetcher.ValidatePublicURL(r.Context(), normalized); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_url", "that URL can't be fetched")
		return
	}

	slot, ok := h.runner.TryAcquire()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "respin_busy", "re-spin is busy right now; please try again shortly")
		return
	}

	imp, err := h.store.Create(r.Context(), CreateInput{
		SourceURL:      payload.URL,
		NormalizedURL:  normalized,
		WorkspaceID:    session.WorkspaceID,
		GuestSessionID: session.GuestSessionID,
	})
	if err != nil {
		slot.Release()
		h.logger.Error("respin create import", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "respin_failed", "could not start a re-spin")
		return
	}

	userID := ""
	if session.User != nil {
		userID = session.User.ID
	}
	locale := strings.TrimSpace(payload.Locale)
	if locale == "" {
		locale = session.WorkspaceLocale
	}
	params := RunParams{
		ImportID:         imp.ID,
		WorkspaceID:      session.WorkspaceID,
		UserID:           userID,
		SourceURL:        normalized,
		LanguageOverride: locale,
	}
	pipeline := h.sessionPipeline
	h.runner.Start(imp.ID, func(ctx context.Context, sink ProgressSink) (RunResult, error) {
		return pipeline.Run(ctx, params, sink)
	}, slot)

	writeJSON(w, http.StatusCreated, startResponse{ImportID: imp.ID, Status: StatusQueued})
}

type statusResponse struct {
	ImportID          string `json:"importId"`
	Status            string `json:"status"`
	FetchMode         string `json:"fetchMode,omitempty"`
	Degraded          bool   `json:"degraded"`
	DegradationReason string `json:"degradationReason,omitempty"`
	ShareSlug         string `json:"shareSlug,omitempty"`
}

// status returns the import status. With Accept: text/event-stream it streams
// live progress via SSE, replaying any buffered events for a client that
// connects after the start POST (Spec 21 demo UI).
func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	importID := r.PathValue("importId")
	imp, err := h.store.Get(r.Context(), importID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "re-spin was not found")
		return
	}

	if acceptsEventStream(r) {
		h.streamStatus(w, r, imp)
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		ImportID:          imp.ID,
		Status:            imp.FetchStatus,
		FetchMode:         imp.FetchMode,
		Degraded:          imp.Degraded,
		DegradationReason: imp.DegradationReason,
		ShareSlug:         imp.ShareSlug,
	})
}

func (h *Handler) streamStatus(w http.ResponseWriter, r *http.Request, imp Import) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusNotImplemented, "streaming_unsupported", "streaming is not supported by this server")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(": respin-progress\n\n"))
	flusher.Flush()

	events, live := h.runner.Subscribe(imp.ID)
	if !live {
		// No active run (cache hit, evicted, or a restart): emit the durable
		// status and a terminal event so the client stops waiting.
		_ = writeSSEEvent(w, string(EventStatus), map[string]string{"status": imp.FetchStatus})
		if isTerminal(imp.FetchStatus) {
			h.emitTerminal(w, imp)
		}
		flusher.Flush()
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			if err := writeSSEEvent(w, string(ev.Type), ev); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (h *Handler) emitTerminal(w http.ResponseWriter, imp Import) {
	switch imp.FetchStatus {
	case StatusFailed:
		_ = writeSSEEvent(w, string(EventFailed), map[string]any{"status": imp.FetchStatus})
	default:
		_ = writeSSEEvent(w, string(EventComplete), map[string]any{
			"status":    imp.FetchStatus,
			"degraded":  imp.Degraded,
			"shareSlug": imp.ShareSlug,
		})
	}
}

type previewResponse struct {
	ImportID      string        `json:"importId"`
	Status        string        `json:"status"`
	Degraded      bool          `json:"degraded"`
	PromptPrefill string        `json:"promptPrefill,omitempty"`
	Source        previewSource `json:"source"`
	After         *previewAfter `json:"after,omitempty"`
}

type previewSource struct {
	URL string `json:"url"`
}

type previewAfter struct {
	SiteID       string    `json:"siteId"`
	PreviewToken string    `json:"previewToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// preview returns the before/after payload: the source URL plus a demo-scoped
// preview token for the generated draft (Spec 21). For a degraded import with no
// generated draft, it returns the salvaged prompt prefill instead.
func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	importID := r.PathValue("importId")
	imp, err := h.store.Get(r.Context(), importID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "re-spin was not found")
		return
	}

	resp := previewResponse{
		ImportID:      imp.ID,
		Status:        imp.FetchStatus,
		Degraded:      imp.Degraded,
		PromptPrefill: promptPrefillFromExtraction(imp.ExtractedContent),
		Source:        previewSource{URL: imp.SourceURL},
	}

	link, err := h.store.LinkedGeneration(r.Context(), imp.ID)
	if err == nil && strings.TrimSpace(link.SiteID) != "" && h.previews != nil {
		token, err := h.previews.Issue(r.Context(), link.SiteID, "")
		if err != nil {
			h.logger.Warn("respin issue preview token", "importId", imp.ID, "error", err.Error())
		} else {
			resp.After = &previewAfter{
				SiteID:       link.SiteID,
				PreviewToken: token.Token,
				ExpiresAt:    token.ExpiresAt,
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

type claimResponse struct {
	Session auth.Session `json:"session"`
	SiteID  string       `json:"siteId,omitempty"`
}

// claim binds the demo import to the visitor's browser: it adopts the up-front
// demo trial session (setting cookies), binds the workspace on the import, and
// lands the visitor in the builder. The generation already booked the prompt, so
// no additional prompt is charged here (Spec 21 claim handoff).
func (h *Handler) claim(w http.ResponseWriter, r *http.Request) {
	if h.sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "respin_unavailable", "re-spin is not configured")
		return
	}
	importID := r.PathValue("importId")
	imp, err := h.store.Get(r.Context(), importID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "re-spin was not found")
		return
	}
	if strings.TrimSpace(imp.WorkspaceID) != "" {
		writeError(w, http.StatusConflict, "already_claimed", "this re-spin has already been claimed")
		return
	}
	if strings.TrimSpace(imp.GuestSessionID) == "" {
		writeError(w, http.StatusConflict, "not_claimable", "this re-spin cannot be claimed")
		return
	}

	session, err := h.sessions.AdoptRespinDemoSession(w, r, imp.GuestSessionID)
	if err != nil {
		h.logger.Error("respin adopt demo session", "importId", imp.ID, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "respin_failed", "could not claim this re-spin")
		return
	}

	if _, err := h.store.Claim(r.Context(), imp.ID, session.WorkspaceID, imp.GuestSessionID); err != nil {
		if errors.Is(err, ErrAlreadyClaimed) {
			writeError(w, http.StatusConflict, "already_claimed", "this re-spin has already been claimed")
			return
		}
		h.logger.Error("respin claim", "importId", imp.ID, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "respin_failed", "could not claim this re-spin")
		return
	}

	siteID := ""
	if link, err := h.store.LinkedGeneration(r.Context(), imp.ID); err == nil {
		siteID = link.SiteID
	}
	writeJSON(w, http.StatusOK, claimResponse{Session: session, SiteID: siteID})
}

type shareResponse struct {
	ShareSlug string        `json:"shareSlug"`
	Status    string        `json:"status"`
	Degraded  bool          `json:"degraded"`
	Source    previewSource `json:"source"`
	After     *previewAfter `json:"after,omitempty"`
}

// share serves the frozen before/after snapshot for a shared demo (Spec 21).
func (h *Handler) share(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("shareSlug")
	imp, err := h.store.GetByShareSlug(r.Context(), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "shared re-spin was not found")
		return
	}
	resp := shareResponse{
		ShareSlug: imp.ShareSlug,
		Status:    imp.FetchStatus,
		Degraded:  imp.Degraded,
		Source:    previewSource{URL: imp.SourceURL},
	}
	if link, err := h.store.LinkedGeneration(r.Context(), imp.ID); err == nil && strings.TrimSpace(link.SiteID) != "" && h.previews != nil {
		if token, err := h.previews.Issue(r.Context(), link.SiteID, ""); err == nil {
			resp.After = &previewAfter{SiteID: link.SiteID, PreviewToken: token.Token, ExpiresAt: token.ExpiresAt}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) writeBillingError(w http.ResponseWriter, err error) {
	var limitErr *billing.LimitExceededError
	if errors.As(err, &limitErr) {
		writeJSON(w, http.StatusPaymentRequired, map[string]any{
			"error": map[string]string{"code": "plan_limit_exceeded", "message": limitErr.Error()},
		})
		return
	}
	h.logger.Error("respin billing enforce", "error", err.Error())
	writeError(w, http.StatusInternalServerError, "respin_failed", "could not start a re-spin")
}

func isTerminal(status string) bool {
	switch status {
	case StatusSucceeded, StatusDegraded, StatusFailed:
		return true
	default:
		return false
	}
}

func acceptsEventStream(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream")
}

func promptPrefillFromExtraction(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var payload struct {
		PromptPrefill string `json:"promptPrefill"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return payload.PromptPrefill
}

func writeSSEEvent(w http.ResponseWriter, event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
