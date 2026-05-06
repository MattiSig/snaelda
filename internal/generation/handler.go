package generation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/MattiSig/snaelda/internal/authorization"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

type Handler struct {
	service    Generator
	authorizer Authorizer
}

type Generator interface {
	Generate(ctx context.Context, workspaceID string, userID string, input GenerateInput) (GenerateResult, error)
}

type Authorizer interface {
	RequireWorkspaceMember(ctx context.Context, workspaceID string, allowedRoles ...string) (authorization.Scope, error)
}

type generateRequest struct {
	Name   string `json:"name,omitempty"`
	Slug   string `json:"slug,omitempty"`
	Prompt string `json:"prompt"`
}

func NewHandler(db DB) *Handler {
	return &Handler{
		service:    NewService(db),
		authorizer: authorization.New(db),
	}
}

func (h *Handler) Mount(mux *http.ServeMux, requireUser func(http.Handler) http.Handler) {
	mux.Handle("POST /api/sites/generate", requireUser(http.HandlerFunc(h.generate)))
}

func (h *Handler) generate(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication is required")
		return
	}
	workspaceID := user.WorkspaceID
	if workspaceID == "" {
		writeError(w, http.StatusForbidden, "forbidden", "workspace access is required")
		return
	}
	if _, err := h.authorizer.RequireWorkspaceMember(r.Context(), workspaceID, authorization.RoleOwner, authorization.RoleEditor); err != nil {
		writeAuthorizationError(w, err)
		return
	}

	var payload generateRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	result, err := h.service.Generate(r.Context(), workspaceID, user.ID, GenerateInput{
		Name:   strings.TrimSpace(payload.Name),
		Slug:   strings.TrimSpace(payload.Slug),
		Prompt: strings.TrimSpace(payload.Prompt),
	})
	if err != nil {
		writeGenerationError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func writeGenerationError(w http.ResponseWriter, err error) {
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
	default:
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
