package themes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/MattiSig/snaelda/internal/authorization"
	"github.com/MattiSig/snaelda/internal/generation"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
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

type ProgressThemeService interface {
	RegenerateWithProgress(ctx context.Context, workspaceID string, siteID string, sink generation.ProgressSink) (ThemeState, error)
}

type Authorizer interface {
	RequireSite(ctx context.Context, siteID string, allowedRoles ...string) (authorization.Scope, error)
}

type updateThemeRequest struct {
	Palette        *string `json:"palette,omitempty"`
	FontPreset     *string `json:"fontPreset,omitempty"`
	TypeScale      *string `json:"typeScale,omitempty"`
	SectionSpacing *string `json:"sectionSpacing,omitempty"`
	ContentWidth   *string `json:"contentWidth,omitempty"`
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
		TypeScale:      trimPointer(payload.TypeScale),
		SectionSpacing: trimPointer(payload.SectionSpacing),
		ContentWidth:   trimPointer(payload.ContentWidth),
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

	if acceptsEventStream(r) {
		streamer, ok := h.service.(ProgressThemeService)
		if !ok {
			writeError(w, http.StatusNotImplemented, "theme_regeneration_stream_unavailable", "theme regeneration progress streaming is not configured")
			return
		}
		h.streamRegenerate(w, r, siteID, func(ctx context.Context, sink generation.ProgressSink) (ThemeState, error) {
			return streamer.RegenerateWithProgress(ctx, scope.WorkspaceID, siteID, sink)
		})
		return
	}

	state, err := h.service.Regenerate(r.Context(), scope.WorkspaceID, siteID)
	if err != nil {
		writeThemeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func acceptsEventStream(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/event-stream")
}

func (h *Handler) streamRegenerate(w http.ResponseWriter, r *http.Request, siteID string, run func(context.Context, generation.ProgressSink) (ThemeState, error)) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusNotImplemented, "streaming_unsupported", "streaming is not supported by this server")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(": theme-generation-progress\n\n"))
	flusher.Flush()

	progressEvents := make(chan generation.ProgressStep, 8)
	jobIDCh := make(chan string, 1)
	resultCh := make(chan ThemeState, 1)
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
		result, err := run(runCtx, themeProgressSink{
			onJobCreated: func(jobID string) {
				select {
				case jobIDCh <- jobID:
				case <-r.Context().Done():
				}
			},
			onProgress: func(step generation.ProgressStep) {
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

	jobID := ""
	for {
		select {
		case <-r.Context().Done():
			return
		case jobID = <-jobIDCh:
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
			code, message, status := themeErrorDetails(err)
			_ = writeSSEEvent(w, "failed", map[string]any{
				"reason":  code,
				"message": message,
				"status":  status,
			})
			flusher.Flush()
			return
		case <-resultCh:
			if !drainPendingEvents() {
				return
			}
			_ = writeSSEEvent(w, "complete", map[string]string{
				"jobId":   jobID,
				"siteId":  siteID,
				"draftId": siteID,
			})
			flusher.Flush()
			return
		}
	}
}

func trimPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func writeThemeError(w http.ResponseWriter, err error) {
	code, message, status := themeErrorDetails(err)
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
	writeError(w, status, code, message)
}

func themeErrorDetails(err error) (code string, message string, status int) {
	var validationErr siteconfig.ValidationError
	switch {
	case errors.Is(err, ErrNotFound):
		return "site_not_found", "site was not found", http.StatusNotFound
	case errors.Is(err, ErrNoThemeChanges):
		return "no_theme_changes", "at least one theme field must change", http.StatusBadRequest
	case errors.Is(err, ErrThemePaletteInvalid):
		return "invalid_theme_palette", "theme palette is not supported", http.StatusBadRequest
	case errors.Is(err, ErrThemeFontPresetInvalid):
		return "invalid_theme_font_preset", "theme font preset is not supported", http.StatusBadRequest
	case errors.Is(err, ErrThemeTypeScaleInvalid):
		return "invalid_theme_type_scale", "theme type scale is not supported", http.StatusBadRequest
	case errors.Is(err, ErrThemeSpacingInvalid):
		return "invalid_theme_section_spacing", "theme section spacing is not supported", http.StatusBadRequest
	case errors.Is(err, ErrThemeContentWidthInvalid):
		return "invalid_theme_content_width", "theme content width is not supported", http.StatusBadRequest
	case errors.Is(err, ErrThemeRadiusInvalid):
		return "invalid_theme_radius", "theme radius is not supported", http.StatusBadRequest
	case errors.Is(err, ErrThemeButtonStyleInvalid):
		return "invalid_theme_button_style", "theme button style is not supported", http.StatusBadRequest
	case errors.Is(err, ErrThemeImageStyleInvalid):
		return "invalid_theme_image_style", "theme image style is not supported", http.StatusBadRequest
	case errors.Is(err, ErrThemeRegenerationOff):
		return "theme_regeneration_unavailable", "theme regeneration is not configured", http.StatusServiceUnavailable
	case errors.Is(err, sites.ErrDraftConflict):
		return "draft_conflict", "this draft changed while your edit was in flight; reload the latest version and try again", http.StatusConflict
	case errors.As(err, &validationErr):
		return "invalid_theme", "theme changes failed validation", http.StatusBadRequest
	default:
		return "theme_write_failed", "could not save theme", http.StatusInternalServerError
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

type themeProgressSink struct {
	onJobCreated func(string)
	onProgress   func(generation.ProgressStep)
}

func (s themeProgressSink) OnJobCreated(jobID string) {
	if s.onJobCreated != nil {
		s.onJobCreated(jobID)
	}
}

func (s themeProgressSink) OnProgress(step generation.ProgressStep) {
	if s.onProgress != nil {
		s.onProgress(step)
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
