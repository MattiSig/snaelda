package generation

import (
	"context"
	"encoding/json"
	"errors"
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
	authorizer Authorizer
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

type Authorizer interface {
	RequireWorkspaceMember(ctx context.Context, workspaceID string, allowedRoles ...string) (authorization.Scope, error)
	RequireSite(ctx context.Context, siteID string, allowedRoles ...string) (authorization.Scope, error)
}

type generateRequest struct {
	Name   string `json:"name,omitempty"`
	Slug   string `json:"slug,omitempty"`
	Prompt string `json:"prompt"`
}

type repromptRequest struct {
	Prompt string `json:"prompt"`
}

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
	return &Handler{
		billingDB:  db,
		service:    NewService(db, cfg.Planner, options...),
		authorizer: authorization.New(db),
		logger:     logger,
	}
}

func (h *Handler) Mount(mux *http.ServeMux, requireUser func(http.Handler) http.Handler) {
	mux.Handle("POST /api/sites/generate", requireUser(http.HandlerFunc(h.generate)))
	mux.Handle("POST /api/sites/{siteId}/reprompt", requireUser(http.HandlerFunc(h.repromptSite)))
	mux.Handle("POST /api/sites/{siteId}/pages/{pageId}/reprompt", requireUser(http.HandlerFunc(h.repromptPage)))
	mux.Handle("POST /api/sites/{siteId}/undo", requireUser(http.HandlerFunc(h.undoSite)))
}

func (h *Handler) generate(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		if user, userOK := auth.UserFromContext(r.Context()); userOK {
			session = auth.Session{
				Kind:          auth.SessionKindAuthenticated,
				WorkspaceID:   user.WorkspaceID,
				WorkspaceRole: user.WorkspaceRole,
				User:          &user,
			}
			ok = true
		}
	}
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
	result, err := h.service.Generate(r.Context(), workspaceID, userID, GenerateInput{
		Name:   strings.TrimSpace(payload.Name),
		Slug:   strings.TrimSpace(payload.Slug),
		Prompt: strings.TrimSpace(payload.Prompt),
	})
	if err != nil {
		h.writeGenerationError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
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
	session, _ := auth.SessionFromContext(r.Context())
	if session.User == nil {
		if user, ok := auth.UserFromContext(r.Context()); ok {
			session.User = &user
		}
	}
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
	result, err := h.service.RepromptSite(r.Context(), scope.WorkspaceID, userID, siteID, RepromptInput{
		Prompt: strings.TrimSpace(payload.Prompt),
	})
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
	session, _ := auth.SessionFromContext(r.Context())
	if session.User == nil {
		if user, ok := auth.UserFromContext(r.Context()); ok {
			session.User = &user
		}
	}
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
	result, err := h.service.RepromptPage(r.Context(), scope.WorkspaceID, userID, siteID, pageID, RepromptInput{
		Prompt: strings.TrimSpace(payload.Prompt),
	})
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

func (h *Handler) writeGenerationError(w http.ResponseWriter, r *http.Request, err error) {
	var validationErr siteconfig.ValidationError
	switch {
	case errors.Is(err, ErrPromptRequired):
		writeError(w, http.StatusBadRequest, "generation_prompt_required", "a prompt is required to generate a draft")
	case errors.Is(err, ErrSiteSlugInvalid):
		writeError(w, http.StatusBadRequest, "invalid_site_slug", "site slug must use lowercase words separated by hyphens")
	case errors.Is(err, ErrSiteSlugConflict):
		writeError(w, http.StatusConflict, "site_slug_conflict", "site slug is already in use")
	case errors.As(err, &validationErr):
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]string{
				"code":    "invalid_generated_draft",
				"message": "generated draft failed validation",
			},
			"issues": validationErr.Issues,
		})
	case errors.Is(err, sites.ErrNotFound), errors.Is(err, sites.ErrPageNotFound):
		writeError(w, http.StatusNotFound, "draft_scope_not_found", "the requested draft scope was not found")
	case errors.Is(err, ErrNoDraftRevision):
		writeError(w, http.StatusNotFound, "draft_revision_not_found", "there is no draft revision to restore")
	case errors.Is(err, billing.ErrPlanLimitExceeded):
		writeError(w, http.StatusForbidden, "plan_limit_exceeded", err.Error())
	default:
		h.logger.Error("generate site draft", "method", r.Method, "path", r.URL.Path, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "generate_failed", "could not generate site draft")
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
