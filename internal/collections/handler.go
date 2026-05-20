package collections

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/MattiSig/snaelda/internal/authorization"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
)

// Authorizer is the authorization surface the collections handler needs.
type Authorizer interface {
	RequireSite(ctx context.Context, siteID string, allowedRoles ...string) (authorization.Scope, error)
}

// Handler hosts the collections HTTP routes.
type Handler struct {
	mutator    *Mutator
	authorizer Authorizer
}

// NewHandler wires a Handler against the sites DB used by the sites module so
// collections share the same draft store.
func NewHandler(db sites.DB) *Handler {
	return &Handler{
		mutator:    NewMutator(sites.NewPostgresReader(db), sites.NewPostgresWriter(db)),
		authorizer: authorization.New(db),
	}
}

// Mount registers the collection routes onto mux.
func (h *Handler) Mount(mux *http.ServeMux, requireUser func(http.Handler) http.Handler) {
	mux.Handle("GET /api/sites/{siteId}/collections", requireUser(http.HandlerFunc(h.list)))
	mux.Handle("POST /api/sites/{siteId}/collections", requireUser(http.HandlerFunc(h.create)))
	mux.Handle("GET /api/sites/{siteId}/collections/{collectionId}", requireUser(http.HandlerFunc(h.get)))
	mux.Handle("PATCH /api/sites/{siteId}/collections/{collectionId}", requireUser(http.HandlerFunc(h.update)))
	mux.Handle("DELETE /api/sites/{siteId}/collections/{collectionId}", requireUser(http.HandlerFunc(h.delete)))

	mux.Handle("GET /api/sites/{siteId}/collections/{collectionId}/entries", requireUser(http.HandlerFunc(h.listEntries)))
	mux.Handle("POST /api/sites/{siteId}/collections/{collectionId}/entries", requireUser(http.HandlerFunc(h.createEntry)))
	mux.Handle("GET /api/sites/{siteId}/collections/{collectionId}/entries/{entryId}", requireUser(http.HandlerFunc(h.getEntry)))
	mux.Handle("PATCH /api/sites/{siteId}/collections/{collectionId}/entries/{entryId}", requireUser(http.HandlerFunc(h.updateEntry)))
	mux.Handle("DELETE /api/sites/{siteId}/collections/{collectionId}/entries/{entryId}", requireUser(http.HandlerFunc(h.deleteEntry)))
	mux.Handle("POST /api/sites/{siteId}/collections/{collectionId}/entries/reorder", requireUser(http.HandlerFunc(h.reorderEntries)))
}

type createCollectionRequest struct {
	Slug          string                            `json:"slug,omitempty"`
	SingularLabel string                            `json:"singularLabel"`
	PluralLabel   string                            `json:"pluralLabel"`
	Schema        []siteconfig.FieldDefinition      `json:"schema,omitempty"`
	Settings      *siteconfig.CollectionSettings    `json:"settings,omitempty"`
}

type updateCollectionRequest struct {
	Slug          *string                           `json:"slug,omitempty"`
	SingularLabel *string                           `json:"singularLabel,omitempty"`
	PluralLabel   *string                           `json:"pluralLabel,omitempty"`
	Schema        []siteconfig.FieldDefinition      `json:"schema,omitempty"`
	Settings      *siteconfig.CollectionSettings    `json:"settings,omitempty"`
}

type createEntryRequest struct {
	Slug   string               `json:"slug,omitempty"`
	Fields map[string]any       `json:"fields,omitempty"`
	SEO    siteconfig.SEOConfig `json:"seo,omitempty"`
	Status string               `json:"status,omitempty"`
}

type updateEntryRequest struct {
	Slug   *string               `json:"slug,omitempty"`
	Fields map[string]any        `json:"fields,omitempty"`
	SEO    *siteconfig.SEOConfig `json:"seo,omitempty"`
	Status *string               `json:"status,omitempty"`
}

type reorderEntriesRequest struct {
	EntryIDs []string `json:"entryIds"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	if _, err := h.authorizer.RequireSite(r.Context(), siteID); err != nil {
		writeAuthorizationError(w, err)
		return
	}
	collections, err := h.mutator.ListCollections(r.Context(), siteID)
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"collections": collectionsOrEmpty(collections)})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	collectionID := r.PathValue("collectionId")
	if _, err := h.authorizer.RequireSite(r.Context(), siteID); err != nil {
		writeAuthorizationError(w, err)
		return
	}
	collection, err := h.mutator.GetCollection(r.Context(), siteID, collectionID)
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"collection": collection})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	var payload createCollectionRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}
	input := CreateCollectionInput{
		Slug:          strings.TrimSpace(payload.Slug),
		SingularLabel: payload.SingularLabel,
		PluralLabel:   payload.PluralLabel,
		Schema:        payload.Schema,
	}
	if payload.Settings != nil {
		input.Settings = *payload.Settings
	}
	collection, err := h.mutator.CreateCollection(r.Context(), scope.WorkspaceID, siteID, input)
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"collection": collection})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	collectionID := r.PathValue("collectionId")
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	var payload updateCollectionRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}
	input := UpdateCollectionInput{
		Slug:          payload.Slug,
		SingularLabel: payload.SingularLabel,
		PluralLabel:   payload.PluralLabel,
		Schema:        payload.Schema,
		Settings:      payload.Settings,
	}
	collection, err := h.mutator.UpdateCollection(r.Context(), scope.WorkspaceID, siteID, collectionID, input)
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"collection": collection})
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	collectionID := r.PathValue("collectionId")
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	if err := h.mutator.DeleteCollection(r.Context(), scope.WorkspaceID, siteID, collectionID); err != nil {
		writeCollectionError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listEntries(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	collectionID := r.PathValue("collectionId")
	if _, err := h.authorizer.RequireSite(r.Context(), siteID); err != nil {
		writeAuthorizationError(w, err)
		return
	}
	entries, err := h.mutator.ListEntries(r.Context(), siteID, collectionID)
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entriesOrEmpty(entries)})
}

func (h *Handler) getEntry(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	collectionID := r.PathValue("collectionId")
	entryID := r.PathValue("entryId")
	if _, err := h.authorizer.RequireSite(r.Context(), siteID); err != nil {
		writeAuthorizationError(w, err)
		return
	}
	entry, err := h.mutator.GetEntry(r.Context(), siteID, collectionID, entryID)
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entry": entry})
}

func (h *Handler) createEntry(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	collectionID := r.PathValue("collectionId")
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	var payload createEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}
	entry, err := h.mutator.CreateEntry(r.Context(), scope.WorkspaceID, siteID, collectionID, CreateEntryInput{
		Slug:   strings.TrimSpace(payload.Slug),
		Fields: payload.Fields,
		SEO:    payload.SEO,
		Status: payload.Status,
	})
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"entry": entry})
}

func (h *Handler) updateEntry(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	collectionID := r.PathValue("collectionId")
	entryID := r.PathValue("entryId")
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	var payload updateEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}
	entry, err := h.mutator.UpdateEntry(r.Context(), scope.WorkspaceID, siteID, collectionID, entryID, UpdateEntryInput{
		Slug:   payload.Slug,
		Fields: payload.Fields,
		SEO:    payload.SEO,
		Status: payload.Status,
	})
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entry": entry})
}

func (h *Handler) deleteEntry(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	collectionID := r.PathValue("collectionId")
	entryID := r.PathValue("entryId")
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	if err := h.mutator.DeleteEntry(r.Context(), scope.WorkspaceID, siteID, collectionID, entryID); err != nil {
		writeCollectionError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) reorderEntries(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	collectionID := r.PathValue("collectionId")
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	var payload reorderEntriesRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}
	entries, err := h.mutator.ReorderEntries(r.Context(), scope.WorkspaceID, siteID, collectionID, payload.EntryIDs)
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entriesOrEmpty(entries)})
}

func writeCollectionError(w http.ResponseWriter, err error) {
	var validationErr siteconfig.ValidationError
	switch {
	case errors.Is(err, ErrCollectionNotFound):
		writeError(w, http.StatusNotFound, "collection_not_found", "collection was not found")
	case errors.Is(err, ErrEntryNotFound):
		writeError(w, http.StatusNotFound, "entry_not_found", "collection entry was not found")
	case errors.Is(err, ErrCollectionInUse):
		writeError(w, http.StatusBadRequest, "collection_in_use", err.Error())
	case errors.Is(err, ErrCollectionSlugConflict):
		writeError(w, http.StatusConflict, "collection_slug_conflict", "collection slug is already in use")
	case errors.Is(err, ErrEntrySlugConflict):
		writeError(w, http.StatusConflict, "entry_slug_conflict", "entry slug is already in use")
	case errors.Is(err, ErrCollectionLabelRequired):
		writeError(w, http.StatusBadRequest, "invalid_collection_label", "collection labels are required")
	case errors.Is(err, ErrCollectionSlugInvalid):
		writeError(w, http.StatusBadRequest, "invalid_collection_slug", "collection slug must be lowercase words separated by hyphens")
	case errors.Is(err, ErrNoCollectionChanges):
		writeError(w, http.StatusBadRequest, "no_collection_changes", "at least one field must change")
	case errors.Is(err, ErrNoEntryChanges):
		writeError(w, http.StatusBadRequest, "no_entry_changes", "at least one field must change")
	case errors.Is(err, ErrEntryOrderInvalid):
		writeError(w, http.StatusBadRequest, "invalid_entry_order", "entry reorder must include every entry exactly once")
	case errors.Is(err, sites.ErrNotFound):
		writeError(w, http.StatusNotFound, "site_not_found", "site was not found")
	case errors.As(err, &validationErr):
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": map[string]string{
				"code":    "invalid_site_draft",
				"message": "changes failed validation",
			},
			"issues": validationErr.Issues,
		})
	default:
		writeError(w, http.StatusInternalServerError, "collection_write_failed", "could not save collection")
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

func collectionsOrEmpty(collections []siteconfig.Collection) []siteconfig.Collection {
	if collections == nil {
		return []siteconfig.Collection{}
	}
	return collections
}

func entriesOrEmpty(entries []siteconfig.CollectionEntry) []siteconfig.CollectionEntry {
	if entries == nil {
		return []siteconfig.CollectionEntry{}
	}
	return entries
}
