package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/authorization"
	"github.com/MattiSig/snaelda/internal/billing"
	"github.com/MattiSig/snaelda/internal/platform/audit"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
)

type Handler struct {
	billingDB  billing.AccessStore
	service    Generator
	jobs       JobLoader
	authorizer Authorizer
	limiter    *GenerationRateLimiter
	logger     *slog.Logger
}

type HandlerConfig struct {
	Planner        generationPlanBuilder
	StarterImagery *StarterImagery
	AssetImporter  AssetImporter
	Logger         *slog.Logger
	AuditRecorder  *audit.Recorder
}

type Generator interface {
	Generate(ctx context.Context, workspaceID string, userID string, input GenerateInput) (GenerateResult, error)
	RepromptSite(ctx context.Context, workspaceID string, userID string, siteID string, input RepromptInput) (GenerateResult, error)
	RepromptPage(ctx context.Context, workspaceID string, userID string, siteID string, pageID string, input RepromptInput) (GenerateResult, error)
	UndoLastDraftRevision(ctx context.Context, workspaceID string, siteID string) (siteconfig.SiteDraft, error)
}

type ProgressGenerator interface {
	GenerateWithProgress(ctx context.Context, workspaceID string, userID string, input GenerateInput, sink ProgressSink) (GenerateResult, error)
	RepromptSiteWithProgress(ctx context.Context, workspaceID string, userID string, siteID string, input RepromptInput, sink ProgressSink) (GenerateResult, error)
	RepromptPageWithProgress(ctx context.Context, workspaceID string, userID string, siteID string, pageID string, input RepromptInput, sink ProgressSink) (GenerateResult, error)
}

type JobLoader interface {
	LoadJob(ctx context.Context, workspaceID string, jobID string) (JobStatus, error)
}

type Authorizer interface {
	RequireWorkspaceMember(ctx context.Context, workspaceID string, allowedRoles ...string) (authorization.Scope, error)
	RequireSite(ctx context.Context, siteID string, allowedRoles ...string) (authorization.Scope, error)
}

type generateRequest struct {
	Name              string                 `json:"name,omitempty"`
	Slug              string                 `json:"slug,omitempty"`
	Prompt            string                 `json:"prompt"`
	PreferredLanguage string                 `json:"preferredLanguage,omitempty"`
	OptionalHints     map[string]string      `json:"optionalHints,omitempty"`
	Brand             siteconfig.BrandConfig `json:"brand,omitempty"`
}

type repromptRequest struct {
	Prompt string `json:"prompt"`
}

const maxGenerationPromptCharacters = 4000

func NewHandler(db DB, cfg HandlerConfig) *Handler {
	options := []ServiceOption{}
	if cfg.StarterImagery != nil {
		options = append(options, WithStarterImagery(cfg.StarterImagery))
	}
	if cfg.AssetImporter != nil {
		options = append(options, WithAssetImporter(cfg.AssetImporter))
	}
	if cfg.Logger != nil {
		options = append(options, WithLogger(cfg.Logger))
	}
	if cfg.AuditRecorder != nil {
		options = append(options, WithAuditRecorder(cfg.AuditRecorder))
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	service := NewService(db, cfg.Planner, options...)
	return &Handler{
		billingDB:  db,
		service:    service,
		jobs:       service,
		authorizer: authorization.New(db),
		limiter:    NewGenerationRateLimiter(db, logger),
		logger:     logger,
	}
}

func (h *Handler) Mount(mux *http.ServeMux, requireUser func(http.Handler) http.Handler) {
	mux.Handle("POST /api/sites/generate", requireUser(http.HandlerFunc(h.generate)))
	mux.Handle("GET /api/generation/jobs/{jobId}", requireUser(http.HandlerFunc(h.getJob)))
	mux.Handle("POST /api/sites/{siteId}/reprompt", requireUser(http.HandlerFunc(h.repromptSite)))
	mux.Handle("POST /api/sites/{siteId}/pages/{pageId}/reprompt", requireUser(http.HandlerFunc(h.repromptPage)))
	mux.Handle("POST /api/sites/{siteId}/undo", requireUser(http.HandlerFunc(h.undoSite)))
}

func (h *Handler) generate(w http.ResponseWriter, r *http.Request) {
	session, ok := builderSessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	workspaceID := session.WorkspaceID
	if workspaceID == "" {
		writeError(w, http.StatusForbidden, "forbidden", "workspace access is required")
		return
	}
	if _, err := h.authorizer.RequireWorkspaceMember(r.Context(), workspaceID, authorization.RoleOwner, authorization.RoleEditor); err != nil {
		writeAuthorizationError(w, err)
		return
	}
	if h.billingDB != nil {
		if err := billing.EnforceSiteLimit(r.Context(), h.billingDB, workspaceID); err != nil {
			h.writeGenerationError(w, r, err)
			return
		}
		if err := billing.EnforcePromptLimit(r.Context(), h.billingDB, workspaceID); err != nil {
			h.writeGenerationError(w, r, err)
			return
		}
	}

	var payload generateRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	userID := ""
	if session.User != nil {
		userID = session.User.ID
	}
	if err := h.guardGenerationRequest(r.Context(), workspaceID, userID, "site", payload.Prompt); err != nil {
		h.writeGenerationError(w, r, err)
		return
	}
	input := GenerateInput{
		Name:              strings.TrimSpace(payload.Name),
		Slug:              strings.TrimSpace(payload.Slug),
		Prompt:            strings.TrimSpace(payload.Prompt),
		PreferredLanguage: strings.TrimSpace(payload.PreferredLanguage),
		OptionalHints:     trimOptionalHints(payload.OptionalHints),
		Brand:             payload.Brand,
	}
	if acceptsEventStream(r) {
		streamer, ok := h.service.(ProgressGenerator)
		if !ok {
			writeError(w, http.StatusNotImplemented, "generation_stream_unavailable", "generation progress streaming is not configured")
			return
		}
		h.streamGenerate(w, r, func(ctx context.Context, sink ProgressSink) (GenerateResult, error) {
			return streamer.GenerateWithProgress(ctx, workspaceID, userID, input, sink)
		})
		return
	}
	result, err := h.service.Generate(r.Context(), workspaceID, userID, input)
	if err != nil {
		h.writeGenerationError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func trimOptionalHints(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		output[key] = value
	}
	if len(output) == 0 {
		return nil
	}
	return output
}

func builderSessionFromContext(ctx context.Context) (auth.Session, bool) {
	if session, ok := auth.SessionFromContext(ctx); ok {
		if session.User == nil {
			if user, userOK := auth.UserFromContext(ctx); userOK {
				session.User = &user
			}
		}
		return session, true
	}
	if user, ok := auth.UserFromContext(ctx); ok {
		return auth.Session{
			Kind:          auth.SessionKindAuthenticated,
			WorkspaceID:   user.WorkspaceID,
			WorkspaceRole: user.WorkspaceRole,
			User:          &user,
		}, true
	}
	return auth.Session{}, false
}

func acceptsEventStream(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/event-stream")
}

func (h *Handler) repromptSite(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	if siteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
		return
	}
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	session, _ := builderSessionFromContext(r.Context())
	if h.billingDB != nil {
		if err := billing.EnforcePromptLimit(r.Context(), h.billingDB, scope.WorkspaceID); err != nil {
			h.writeGenerationError(w, r, err)
			return
		}
	}

	var payload repromptRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	userID := ""
	if session.User != nil {
		userID = session.User.ID
	}
	if err := h.guardGenerationRequest(r.Context(), scope.WorkspaceID, userID, "site_reprompt", payload.Prompt); err != nil {
		h.writeGenerationError(w, r, err)
		return
	}
	input := RepromptInput{Prompt: strings.TrimSpace(payload.Prompt)}
	if acceptsEventStream(r) {
		streamer, ok := h.service.(ProgressGenerator)
		if !ok {
			writeError(w, http.StatusNotImplemented, "generation_stream_unavailable", "generation progress streaming is not configured")
			return
		}
		h.streamGenerate(w, r, func(ctx context.Context, sink ProgressSink) (GenerateResult, error) {
			return streamer.RepromptSiteWithProgress(ctx, scope.WorkspaceID, userID, siteID, input, sink)
		})
		return
	}
	result, err := h.service.RepromptSite(r.Context(), scope.WorkspaceID, userID, siteID, input)
	if err != nil {
		h.writeGenerationError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) repromptPage(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	pageID := r.PathValue("pageId")
	if siteID == "" || pageID == "" {
		writeError(w, http.StatusBadRequest, "invalid_page_resource", "site and page ids are required")
		return
	}
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	session, _ := builderSessionFromContext(r.Context())
	if h.billingDB != nil {
		if err := billing.EnforcePromptLimit(r.Context(), h.billingDB, scope.WorkspaceID); err != nil {
			h.writeGenerationError(w, r, err)
			return
		}
	}

	var payload repromptRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	userID := ""
	if session.User != nil {
		userID = session.User.ID
	}
	if err := h.guardGenerationRequest(r.Context(), scope.WorkspaceID, userID, "page_reprompt", payload.Prompt); err != nil {
		h.writeGenerationError(w, r, err)
		return
	}
	input := RepromptInput{Prompt: strings.TrimSpace(payload.Prompt)}
	if acceptsEventStream(r) {
		streamer, ok := h.service.(ProgressGenerator)
		if !ok {
			writeError(w, http.StatusNotImplemented, "generation_stream_unavailable", "generation progress streaming is not configured")
			return
		}
		h.streamGenerate(w, r, func(ctx context.Context, sink ProgressSink) (GenerateResult, error) {
			return streamer.RepromptPageWithProgress(ctx, scope.WorkspaceID, userID, siteID, pageID, input, sink)
		})
		return
	}
	result, err := h.service.RepromptPage(r.Context(), scope.WorkspaceID, userID, siteID, pageID, input)
	if err != nil {
		h.writeGenerationError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) undoSite(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	if siteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
		return
	}
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}

	draft, err := h.service.UndoLastDraftRevision(r.Context(), scope.WorkspaceID, siteID)
	if err != nil {
		h.writeGenerationError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"draft": draft})
}

func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	if h.jobs == nil {
		writeError(w, http.StatusNotImplemented, "generation_jobs_unavailable", "generation jobs are not configured")
		return
	}
	session, ok := builderSessionFromContext(r.Context())
	if !ok || strings.TrimSpace(session.WorkspaceID) == "" {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "a session is required")
		return
	}
	jobID := strings.TrimSpace(r.PathValue("jobId"))
	if jobID == "" {
		writeError(w, http.StatusBadRequest, "invalid_job_id", "job id is required")
		return
	}
	job, err := h.jobs.LoadJob(r.Context(), session.WorkspaceID, jobID)
	if err != nil {
		if errors.Is(err, sites.ErrNotFound) {
			writeError(w, http.StatusNotFound, "generation_job_not_found", "generation job was not found")
			return
		}
		h.logger.Error("load generation job", "jobId", jobID, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "generation_job_failed", "could not load generation job")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": job})
}

func (h *Handler) streamGenerate(w http.ResponseWriter, r *http.Request, run func(context.Context, ProgressSink) (GenerateResult, error)) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusNotImplemented, "streaming_unsupported", "streaming is not supported by this server")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(": generation-progress\n\n"))
	flusher.Flush()

	progressEvents := make(chan ProgressStep, 16)
	jobIDCh := make(chan string, 1)
	resultCh := make(chan GenerateResult, 1)
	errCh := make(chan error, 1)
	runCtx := context.WithoutCancel(r.Context())
	drainPendingEvents := func() bool {
		for {
			select {
			case jobID := <-jobIDCh:
				if err := writeSSEEvent(w, "job", map[string]string{"jobId": jobID}); err != nil {
					return false
				}
				flusher.Flush()
			case step := <-progressEvents:
				if err := writeSSEEvent(w, "progress", step); err != nil {
					return false
				}
				flusher.Flush()
			default:
				return true
			}
		}
	}

	go func() {
		result, err := run(runCtx, progressSinkHandlers{
			onJobCreated: func(jobID string) {
				select {
				case jobIDCh <- jobID:
				case <-r.Context().Done():
				}
			},
			onProgress: func(step ProgressStep) {
				select {
				case progressEvents <- step:
				case <-r.Context().Done():
				}
			},
		})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case jobID := <-jobIDCh:
			if err := writeSSEEvent(w, "job", map[string]string{"jobId": jobID}); err != nil {
				return
			}
			flusher.Flush()
		case step := <-progressEvents:
			if err := writeSSEEvent(w, "progress", step); err != nil {
				return
			}
			flusher.Flush()
		case err := <-errCh:
			if !drainPendingEvents() {
				return
			}
			reason, message, status := generationErrorDetails(err)
			if status == http.StatusInternalServerError {
				h.logger.Error("generate site draft stream", "path", r.URL.Path, "reason", reason, "error", err.Error())
			}
			_ = writeSSEEvent(w, "failed", map[string]any{
				"reason":  reason,
				"message": message,
				"status":  status,
			})
			flusher.Flush()
			return
		case result := <-resultCh:
			if !drainPendingEvents() {
				return
			}
			_ = writeSSEEvent(w, "complete", map[string]string{
				"jobId":   result.JobID,
				"siteId":  result.Draft.Site.ID,
				"draftId": result.Draft.Site.ID,
			})
			flusher.Flush()
			return
		}
	}
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

func (h *Handler) guardGenerationRequest(ctx context.Context, workspaceID string, userID string, scope string, prompt string) error {
	prompt = strings.TrimSpace(prompt)
	if len(prompt) > maxGenerationPromptCharacters {
		return fmt.Errorf("%w: %d", ErrPromptTooLong, maxGenerationPromptCharacters)
	}
	if h.limiter != nil && !h.limiter.Allow(ctx, workspaceID, userID, scope) {
		return ErrGenerationRateLimited
	}
	return nil
}

func (h *Handler) writeGenerationError(w http.ResponseWriter, r *http.Request, err error) {
	code, message, status := generationErrorDetails(err)
	var validationErr siteconfig.ValidationError
	if errors.As(err, &validationErr) {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]string{
				"code":    code,
				"message": message,
			},
			"issues": validationErr.Issues,
		})
		return
	}
	if status == http.StatusInternalServerError {
		h.logger.Error("generate site draft", "method", r.Method, "path", r.URL.Path, "error", err.Error())
	}
	writeError(w, status, code, message)
}

func generationErrorDetails(err error) (code string, message string, status int) {
	var validationErr siteconfig.ValidationError
	switch {
	case errors.Is(err, ErrPromptRequired):
		return "generation_prompt_required", "a prompt is required to generate a draft", http.StatusBadRequest
	case errors.Is(err, ErrPromptTooLong):
		return "generation_prompt_too_long", "prompt is too long", http.StatusBadRequest
	case errors.Is(err, ErrSiteSlugInvalid):
		return "invalid_site_slug", "site slug must use lowercase words separated by hyphens", http.StatusBadRequest
	case errors.Is(err, ErrSiteSlugConflict):
		return "site_slug_conflict", "site slug is already in use", http.StatusConflict
	case errors.Is(err, ErrGenerationRateLimited):
		return "rate_limited", "too many generation requests; please wait before trying again", http.StatusTooManyRequests
	case errors.As(err, &validationErr):
		return "invalid_generated_draft", "generated draft failed validation", http.StatusBadRequest
	case errors.Is(err, sites.ErrNotFound), errors.Is(err, sites.ErrPageNotFound):
		return "draft_scope_not_found", "the requested draft scope was not found", http.StatusNotFound
	case errors.Is(err, ErrNoDraftRevision):
		return "draft_revision_not_found", "there is no draft revision to restore", http.StatusNotFound
	case errors.Is(err, billing.ErrPlanLimitExceeded):
		return "plan_limit_exceeded", err.Error(), http.StatusForbidden
	default:
		return generationFailureReason(err), "We could not finish. Please try again.", http.StatusInternalServerError
	}
}

func writeAuthorizationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, authorization.ErrUnauthenticated):
		writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication is required")
	case errors.Is(err, authorization.ErrInvalidResourceID):
		writeError(w, http.StatusBadRequest, "invalid_resource", "resource id is required")
	case errors.Is(err, authorization.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "access is not allowed")
	case errors.Is(err, authorization.ErrUnavailable):
		writeError(w, http.StatusServiceUnavailable, "authorization_unavailable", "authorization is not configured")
	default:
		writeError(w, http.StatusInternalServerError, "authorization_failed", "authorization failed")
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]map[string]string{
		"error": {
			"code":    code,
			"message": message,
		},
	})
}
