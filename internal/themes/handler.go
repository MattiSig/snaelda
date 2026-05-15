package themes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/MattiSig/snaelda/internal/authorization"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

type Handler struct {
	service    ThemeService
	authorizer Authorizer
}

type ThemeService interface {
	Load(ctx context.Context, siteID string) (ThemeState, error)
	Update(ctx context.Context, workspaceID string, siteID string, input UpdateInput) (ThemeState, error)
	Regenerate(ctx context.Context, workspaceID string, siteID string) (ThemeState, error)
}

type Authorizer interface {
	RequireSite(ctx context.Context, siteID string, allowedRoles ...string) (authorization.Scope, error)
}

type updateThemeRequest struct {
	Palette        *string `json:"palette,omitempty"`
	FontPreset     *string `json:"fontPreset,omitempty"`
	SectionSpacing *string `json:"sectionSpacing,omitempty"`
	Radius         *string `json:"radius,omitempty"`
	ButtonStyle    *string `json:"buttonStyle,omitempty"`
	ImageStyle     *string `json:"imageStyle,omitempty"`
}

type HandlerConfig struct {
	Regenerator themeRegenerator
}

func NewHandler(db DB, cfg HandlerConfig) *Handler {
	return &Handler{
		service:    NewService(db, ServiceConfig{Regenerator: cfg.Regenerator}),
		authorizer: authorization.New(db),
	}
}

func (h *Handler) Mount(mux *http.ServeMux, requireUser func(http.Handler) http.Handler) {
	mux.Handle("GET /api/sites/{siteId}/theme", requireUser(http.HandlerFunc(h.get)))
	mux.Handle("PATCH /api/sites/{siteId}/theme", requireUser(http.HandlerFunc(h.update)))
	mux.Handle("POST /api/sites/{siteId}/theme/regenerate", requireUser(http.HandlerFunc(h.regenerate)))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	siteID := strings.TrimSpace(r.PathValue("siteId"))
	if siteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
		return
	}
	if _, err := h.authorizer.RequireSite(r.Context(), siteID); err != nil {
		writeAuthorizationError(w, err)
		return
	}

	state, err := h.service.Load(r.Context(), siteID)
	if err != nil {
		writeThemeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	siteID := strings.TrimSpace(r.PathValue("siteId"))
	if siteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
		return
	}
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}

	var payload updateThemeRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	state, err := h.service.Update(r.Context(), scope.WorkspaceID, siteID, UpdateInput{
		Palette:        trimPointer(payload.Palette),
		FontPreset:     trimPointer(payload.FontPreset),
		SectionSpacing: trimPointer(payload.SectionSpacing),
		Radius:         trimPointer(payload.Radius),
		ButtonStyle:    trimPointer(payload.ButtonStyle),
		ImageStyle:     trimPointer(payload.ImageStyle),
	})
	if err != nil {
		writeThemeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (h *Handler) regenerate(w http.ResponseWriter, r *http.Request) {
	siteID := strings.TrimSpace(r.PathValue("siteId"))
	if siteID == "" {
		writeError(w, http.StatusBadRequest, "invalid_site_id", "site id is required")
		return
	}
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}

	state, err := h.service.Regenerate(r.Context(), scope.WorkspaceID, siteID)
	if err != nil {
		writeThemeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func trimPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func writeThemeError(w http.ResponseWriter, err error) {
	var validationErr siteconfig.ValidationError
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "site_not_found", "site was not found")
	case errors.Is(err, ErrNoThemeChanges):
		writeError(w, http.StatusBadRequest, "no_theme_changes", "at least one theme field must change")
	case errors.Is(err, ErrThemePaletteInvalid):
		writeError(w, http.StatusBadRequest, "invalid_theme_palette", "theme palette is not supported")
	case errors.Is(err, ErrThemeFontPresetInvalid):
		writeError(w, http.StatusBadRequest, "invalid_theme_font_preset", "theme font preset is not supported")
	case errors.Is(err, ErrThemeSpacingInvalid):
		writeError(w, http.StatusBadRequest, "invalid_theme_section_spacing", "theme section spacing is not supported")
	case errors.Is(err, ErrThemeRadiusInvalid):
		writeError(w, http.StatusBadRequest, "invalid_theme_radius", "theme radius is not supported")
	case errors.Is(err, ErrThemeButtonStyleInvalid):
		writeError(w, http.StatusBadRequest, "invalid_theme_button_style", "theme button style is not supported")
	case errors.Is(err, ErrThemeImageStyleInvalid):
		writeError(w, http.StatusBadRequest, "invalid_theme_image_style", "theme image style is not supported")
	case errors.Is(err, ErrThemeRegenerationOff):
		writeError(w, http.StatusServiceUnavailable, "theme_regeneration_unavailable", "theme regeneration is not configured")
	case errors.As(err, &validationErr):
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]string{
				"code":    "invalid_theme",
				"message": "theme changes failed validation",
			},
			"issues": validationErr.Issues,
		})
	default:
		writeError(w, http.StatusInternalServerError, "theme_write_failed", "could not save theme")
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
