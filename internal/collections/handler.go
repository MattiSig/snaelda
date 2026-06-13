package collections

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
	"github.com/MattiSig/snaelda/internal/generation"
	"github.com/MattiSig/snaelda/internal/platform/audit"
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
	drafter    CollectionDrafter
	reader     Reader
	limiter    *generation.GenerationRateLimiter
	jobs       *generation.PromptActionManager
	history    *generation.PromptHistoryRecorder
	logger     *slog.Logger
	recorder   *audit.Recorder
	billingDB  billing.AccessStore
}

// HandlerConfig is the optional wiring for a Handler. Drafter is wired by the
// API server when an OpenAI key is configured.
type HandlerConfig struct {
	Drafter       CollectionDrafter
	Logger        *slog.Logger
	AuditRecorder *audit.Recorder
}

// NewHandler wires a Handler against the sites DB used by the sites module so
// collections share the same draft store.
func NewHandler(db sites.DB) *Handler {
	return NewHandlerWithConfig(db, HandlerConfig{})
}

// NewHandlerWithConfig wires a Handler with optional drafter support.
func NewHandlerWithConfig(db sites.DB, cfg HandlerConfig) *Handler {
	reader := sites.NewPostgresReader(db)
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		mutator:    NewMutator(reader, sites.NewPostgresWriter(db)),
		authorizer: authorization.New(db),
		drafter:    cfg.Drafter,
		reader:     reader,
		limiter:    generation.NewGenerationRateLimiter(db, logger),
		jobs:       generation.NewPromptActionManagerFromDB(db, logger),
		history:    generation.NewPromptHistoryRecorder(db, logger),
		logger:     logger,
		recorder:   cfg.AuditRecorder,
		billingDB:  db,
	}
}

// Mount registers the collection routes onto mux.
func (h *Handler) Mount(mux *http.ServeMux, requireUser func(http.Handler) http.Handler) {
	mux.Handle("GET /api/sites/{siteId}/collections", requireUser(http.HandlerFunc(h.list)))
	mux.Handle("POST /api/sites/{siteId}/collections", requireUser(http.HandlerFunc(h.create)))
	mux.Handle("POST /api/sites/{siteId}/collections/draft-from-prompt", requireUser(http.HandlerFunc(h.draftFromPrompt)))
	mux.Handle("GET /api/sites/{siteId}/collections/{collectionId}", requireUser(http.HandlerFunc(h.get)))
	mux.Handle("PATCH /api/sites/{siteId}/collections/{collectionId}", requireUser(http.HandlerFunc(h.update)))
	mux.Handle("DELETE /api/sites/{siteId}/collections/{collectionId}", requireUser(http.HandlerFunc(h.delete)))
	mux.Handle("POST /api/sites/{siteId}/collections/{collectionId}/schema/migrate", requireUser(http.HandlerFunc(h.migrateSchema)))

	mux.Handle("GET /api/sites/{siteId}/collections/{collectionId}/entries", requireUser(http.HandlerFunc(h.listEntries)))
	mux.Handle("POST /api/sites/{siteId}/collections/{collectionId}/entries", requireUser(http.HandlerFunc(h.createEntry)))
	mux.Handle("POST /api/sites/{siteId}/collections/{collectionId}/entries/draft-from-prompt", requireUser(http.HandlerFunc(h.draftEntriesFromPrompt)))
	mux.Handle("GET /api/sites/{siteId}/collections/{collectionId}/entries/{entryId}", requireUser(http.HandlerFunc(h.getEntry)))
	mux.Handle("PATCH /api/sites/{siteId}/collections/{collectionId}/entries/{entryId}", requireUser(http.HandlerFunc(h.updateEntry)))
	mux.Handle("DELETE /api/sites/{siteId}/collections/{collectionId}/entries/{entryId}", requireUser(http.HandlerFunc(h.deleteEntry)))
	mux.Handle("POST /api/sites/{siteId}/collections/{collectionId}/entries/{entryId}/duplicate", requireUser(http.HandlerFunc(h.duplicateEntry)))
	mux.Handle("POST /api/sites/{siteId}/collections/{collectionId}/entries/{entryId}/reprompt", requireUser(http.HandlerFunc(h.repromptEntry)))
	mux.Handle("POST /api/sites/{siteId}/collections/{collectionId}/entries/reorder", requireUser(http.HandlerFunc(h.reorderEntries)))
}

type createCollectionRequest struct {
	Slug          string                         `json:"slug,omitempty"`
	SingularLabel string                         `json:"singularLabel"`
	PluralLabel   string                         `json:"pluralLabel"`
	Schema        []siteconfig.FieldDefinition   `json:"schema,omitempty"`
	Settings      *siteconfig.CollectionSettings `json:"settings,omitempty"`
}

type updateCollectionRequest struct {
	Slug          *string                        `json:"slug,omitempty"`
	SingularLabel *string                        `json:"singularLabel,omitempty"`
	PluralLabel   *string                        `json:"pluralLabel,omitempty"`
	Schema        []siteconfig.FieldDefinition   `json:"schema,omitempty"`
	Settings      *siteconfig.CollectionSettings `json:"settings,omitempty"`
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
	if h.billingDB != nil {
		if err := billing.EnforceCollectionLimit(r.Context(), h.billingDB, scope.WorkspaceID, 1); err != nil {
			writeCollectionError(w, err)
			return
		}
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

type draftFromPromptRequest struct {
	Prompt string `json:"prompt"`
}

type draftEntriesFromPromptRequest struct {
	Prompt string `json:"prompt"`
}

type repromptEntryRequest struct {
	Prompt string `json:"prompt"`
}

type migrateSchemaRequest struct {
	Mode     string                       `json:"mode,omitempty"`
	Schema   []siteconfig.FieldDefinition `json:"schema"`
	Mappings []FieldMapping               `json:"mappings,omitempty"`
}

func (h *Handler) migrateSchema(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	collectionID := r.PathValue("collectionId")
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	var payload migrateSchemaRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}
	if payload.Schema == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "schema is required")
		return
	}
	mode := strings.ToLower(strings.TrimSpace(payload.Mode))
	if mode == "" {
		mode = "preview"
	}
	switch mode {
	case "preview":
		plan, err := h.mutator.PreviewSchemaMigration(r.Context(), siteID, collectionID, payload.Schema, payload.Mappings)
		if err != nil {
			writeCollectionError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"plan": plan})
	case "apply":
		collection, plan, err := h.mutator.MigrateSchema(r.Context(), scope.WorkspaceID, siteID, collectionID, payload.Schema, payload.Mappings)
		if err != nil {
			writeCollectionError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"collection": collection,
			"plan":       plan,
		})
	default:
		writeError(w, http.StatusBadRequest, "invalid_request", "mode must be preview or apply")
	}
}

func (h *Handler) draftFromPrompt(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	if h.drafter == nil {
		writeError(w, http.StatusServiceUnavailable, "drafter_unavailable", "the AI collection drafter is not configured")
		return
	}

	var payload draftFromPromptRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}
	prompt := strings.TrimSpace(payload.Prompt)
	if prompt == "" {
		writeError(w, http.StatusBadRequest, "invalid_prompt", "prompt is required")
		return
	}
	if len(prompt) > 2000 {
		writeError(w, http.StatusBadRequest, "invalid_prompt", "prompt is too long")
		return
	}
	session, _ := builderSessionFromContext(r.Context())
	userID := ""
	if session.User != nil {
		userID = session.User.ID
	}
	if !h.limiter.Allow(r.Context(), scope.WorkspaceID, userID, "collection_draft") {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many generation requests; please wait before trying again")
		return
	}

	draft, err := h.reader.LoadDraft(r.Context(), siteID)
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	existingNames := make([]string, 0, len(draft.Collections))
	for _, c := range draft.Collections {
		existingNames = append(existingNames, c.PluralLabel)
	}

	jobID, err := h.jobs.CreateJob(r.Context(), generation.PromptActionInput{
		WorkspaceID: scope.WorkspaceID,
		UserID:      userID,
		SiteID:      siteID,
		Kind:        generation.JobKindCollectionDraft,
		Prompt:      prompt,
		Payload: map[string]any{
			"scope":  "collection",
			"siteId": siteID,
			"prompt": prompt,
		},
	})
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	if err := h.jobs.UpdateProgress(r.Context(), jobID, generation.JobKindCollectionDraft, "prompt.normalize"); err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	if err := h.jobs.UpdateProgress(r.Context(), jobID, generation.JobKindCollectionDraft, "plan.blocks"); err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}

	drafted, err := h.drafter.DraftCollection(r.Context(), CollectionDraftRequest{
		Prompt:              prompt,
		SiteName:            draft.Site.Name,
		SiteGoal:            draft.Site.SEO.Description,
		ExistingCollections: existingNames,
	})
	if err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		if errors.Is(err, ErrCollectionDrafterUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "drafter_unavailable", "the AI collection drafter is not available")
			return
		}
		writeError(w, http.StatusBadGateway, "drafter_failed", err.Error())
		return
	}
	if err := h.jobs.UpdateProgress(r.Context(), jobID, generation.JobKindCollectionDraft, "validate.repair"); err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	if h.billingDB != nil {
		if err := billing.EnforceCollectionLimit(r.Context(), h.billingDB, scope.WorkspaceID, 1); err != nil {
			_ = h.jobs.FailJob(r.Context(), jobID, err)
			writeCollectionError(w, err)
			return
		}
	}

	previousDraft := draft
	collection, err := h.mutator.CreateCollection(r.Context(), scope.WorkspaceID, siteID, CreateCollectionInput{
		Slug:          strings.TrimSpace(drafted.Slug),
		SingularLabel: strings.TrimSpace(drafted.SingularLabel),
		PluralLabel:   strings.TrimSpace(drafted.PluralLabel),
		Schema:        drafted.Schema,
	})
	if err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	nextDraft, err := h.reader.LoadDraft(r.Context(), siteID)
	if err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	historyResult, err := h.recordHistory(r.Context(), generation.PromptHistoryInput{
		WorkspaceID:   scope.WorkspaceID,
		SiteID:        siteID,
		UserID:        userID,
		JobID:         jobID,
		Scope:         "collection",
		TargetID:      collection.ID,
		Prompt:        prompt,
		ChangeSummary: collectionDraftSummary(collection),
		PreviousDraft: previousDraft,
		NextDraft:     nextDraft,
		Summary: map[string]any{
			"collectionId":  collection.ID,
			"slug":          collection.Slug,
			"singularLabel": collection.SingularLabel,
			"pluralLabel":   collection.PluralLabel,
			"fieldCount":    len(collection.Schema),
		},
	})
	if err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	if err := h.jobs.CompleteJob(r.Context(), jobID, siteID, map[string]any{
		"collection":         collection,
		"historyId":          historyResult.HistoryID,
		"resultRevisionId":   historyResult.ResultRevisionID,
		"previousRevisionId": historyResult.PreviousRevisionID,
	}); err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	h.recordAudit(r.Context(), audit.Event{
		WorkspaceID: scope.WorkspaceID,
		SiteID:      siteID,
		UserID:      userID,
		Action:      "collection.draft",
		Metadata: map[string]any{
			"jobId":              jobID,
			"collectionId":       collection.ID,
			"slug":               collection.Slug,
			"historyId":          historyResult.HistoryID,
			"previousRevisionId": historyResult.PreviousRevisionID,
			"resultRevisionId":   historyResult.ResultRevisionID,
		},
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"collection":         collection,
		"jobId":              jobID,
		"historyId":          historyResult.HistoryID,
		"previousRevisionId": historyResult.PreviousRevisionID,
		"resultRevisionId":   historyResult.ResultRevisionID,
	})
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
	if h.billingDB != nil {
		if err := billing.EnforceCollectionEntryLimit(r.Context(), h.billingDB, scope.WorkspaceID, 1); err != nil {
			writeCollectionError(w, err)
			return
		}
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

func (h *Handler) draftEntriesFromPrompt(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	collectionID := r.PathValue("collectionId")
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	if h.drafter == nil {
		writeError(w, http.StatusServiceUnavailable, "drafter_unavailable", "the AI entry drafter is not configured")
		return
	}

	var payload draftEntriesFromPromptRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}
	prompt := strings.TrimSpace(payload.Prompt)
	if prompt == "" {
		writeError(w, http.StatusBadRequest, "invalid_prompt", "prompt is required")
		return
	}
	if len(prompt) > 2000 {
		writeError(w, http.StatusBadRequest, "invalid_prompt", "prompt is too long")
		return
	}
	session, _ := builderSessionFromContext(r.Context())
	userID := ""
	if session.User != nil {
		userID = session.User.ID
	}
	if !h.limiter.Allow(r.Context(), scope.WorkspaceID, userID, "entry_draft") {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many generation requests; please wait before trying again")
		return
	}

	draft, err := h.reader.LoadDraft(r.Context(), siteID)
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	index := findCollection(draft.Collections, collectionID)
	if index == -1 {
		writeCollectionError(w, ErrCollectionNotFound)
		return
	}
	collection := draft.Collections[index]
	if len(collection.Schema) == 0 {
		writeError(w, http.StatusBadRequest, "collection_schema_missing", "collection needs fields before entries can be drafted")
		return
	}
	if field := requiredUnsupportedEntryDraftField(collection.Schema); field != "" {
		writeError(w, http.StatusBadRequest, "collection_schema_unsupported", "required field "+field+" cannot be filled by the entry drafter yet")
		return
	}

	existingEntries := make([]EntryDraftExisting, 0, len(collection.Entries))
	for _, entry := range collection.Entries {
		existingEntries = append(existingEntries, EntryDraftExisting{
			Slug:  entry.Slug,
			Title: entryTitle(entry.Fields, collection.Schema),
		})
	}
	jobID, err := h.jobs.CreateJob(r.Context(), generation.PromptActionInput{
		WorkspaceID: scope.WorkspaceID,
		UserID:      userID,
		SiteID:      siteID,
		Kind:        generation.JobKindEntryDraft,
		Prompt:      prompt,
		Payload: map[string]any{
			"scope":        "collection_entries",
			"siteId":       siteID,
			"collectionId": collectionID,
			"prompt":       prompt,
		},
	})
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	if err := h.jobs.UpdateProgress(r.Context(), jobID, generation.JobKindEntryDraft, "prompt.normalize"); err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	if err := h.jobs.UpdateProgress(r.Context(), jobID, generation.JobKindEntryDraft, "plan.blocks"); err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	drafted, err := h.drafter.DraftEntries(r.Context(), EntryDraftRequest{
		Prompt:   prompt,
		SiteName: draft.Site.Name,
		SiteGoal: draft.Site.SEO.Description,
		Collection: EntryDraftCollection{
			SingularLabel: collection.SingularLabel,
			PluralLabel:   collection.PluralLabel,
			Slug:          collection.Slug,
			Schema:        collection.Schema,
		},
		ExistingEntries: existingEntries,
	})
	if err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		if errors.Is(err, ErrCollectionDrafterUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "drafter_unavailable", "the AI entry drafter is not available")
			return
		}
		writeError(w, http.StatusBadGateway, "drafter_failed", err.Error())
		return
	}
	if err := h.jobs.UpdateProgress(r.Context(), jobID, generation.JobKindEntryDraft, "copy.write"); err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	if err := h.jobs.UpdateProgress(r.Context(), jobID, generation.JobKindEntryDraft, "validate.repair"); err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	if h.billingDB != nil {
		if err := billing.EnforceCollectionEntryLimit(r.Context(), h.billingDB, scope.WorkspaceID, len(drafted.Entries)); err != nil {
			_ = h.jobs.FailJob(r.Context(), jobID, err)
			writeCollectionError(w, err)
			return
		}
	}

	inputs := make([]CreateEntryInput, 0, len(drafted.Entries))
	for _, draftEntry := range drafted.Entries {
		inputs = append(inputs, CreateEntryInput{
			Slug:   strings.TrimSpace(draftEntry.Slug),
			Fields: sanitizeEntryDraftFields(draftEntry.Fields, collection.Schema),
			SEO:    draftEntry.SEO,
			Status: siteconfig.EntryStatusDraft,
		})
	}
	previousDraft := draft
	created, err := h.mutator.CreateEntries(r.Context(), scope.WorkspaceID, siteID, collectionID, inputs)
	if err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	nextDraft, err := h.reader.LoadDraft(r.Context(), siteID)
	if err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	entryIDs := make([]string, 0, len(created))
	for _, entry := range created {
		entryIDs = append(entryIDs, entry.ID)
	}
	historyResult, err := h.recordHistory(r.Context(), generation.PromptHistoryInput{
		WorkspaceID:   scope.WorkspaceID,
		SiteID:        siteID,
		UserID:        userID,
		JobID:         jobID,
		Scope:         "entry",
		TargetID:      collectionID,
		Prompt:        prompt,
		ChangeSummary: entryDraftSummary(collection, len(created)),
		PreviousDraft: previousDraft,
		NextDraft:     nextDraft,
		Summary: map[string]any{
			"collectionId":   collectionID,
			"collectionSlug": collection.Slug,
			"entryCount":     len(created),
			"entryIds":       entryIDs,
		},
	})
	if err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	if err := h.jobs.CompleteJob(r.Context(), jobID, siteID, map[string]any{
		"entries":            created,
		"historyId":          historyResult.HistoryID,
		"resultRevisionId":   historyResult.ResultRevisionID,
		"previousRevisionId": historyResult.PreviousRevisionID,
	}); err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	h.recordAudit(r.Context(), audit.Event{
		WorkspaceID: scope.WorkspaceID,
		SiteID:      siteID,
		UserID:      userID,
		Action:      "collection.entries_draft",
		Metadata: map[string]any{
			"jobId":              jobID,
			"collectionId":       collectionID,
			"entryCount":         len(created),
			"entryIds":           entryIDs,
			"historyId":          historyResult.HistoryID,
			"previousRevisionId": historyResult.PreviousRevisionID,
			"resultRevisionId":   historyResult.ResultRevisionID,
		},
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"entries":            entriesOrEmpty(created),
		"jobId":              jobID,
		"historyId":          historyResult.HistoryID,
		"previousRevisionId": historyResult.PreviousRevisionID,
		"resultRevisionId":   historyResult.ResultRevisionID,
	})
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

func (h *Handler) duplicateEntry(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	collectionID := r.PathValue("collectionId")
	entryID := r.PathValue("entryId")
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	if h.billingDB != nil {
		if err := billing.EnforceCollectionEntryLimit(r.Context(), h.billingDB, scope.WorkspaceID, 1); err != nil {
			writeCollectionError(w, err)
			return
		}
	}
	entry, err := h.mutator.DuplicateEntry(r.Context(), scope.WorkspaceID, siteID, collectionID, entryID)
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"entry": entry})
}

func (h *Handler) repromptEntry(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteId")
	collectionID := r.PathValue("collectionId")
	entryID := r.PathValue("entryId")
	scope, err := h.authorizer.RequireSite(r.Context(), siteID, authorization.RoleOwner, authorization.RoleEditor)
	if err != nil {
		writeAuthorizationError(w, err)
		return
	}
	if h.drafter == nil {
		writeError(w, http.StatusServiceUnavailable, "drafter_unavailable", "the AI entry drafter is not configured")
		return
	}

	var payload repromptEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}
	prompt := strings.TrimSpace(payload.Prompt)
	if prompt == "" {
		writeError(w, http.StatusBadRequest, "invalid_prompt", "prompt is required")
		return
	}
	if len(prompt) > 2000 {
		writeError(w, http.StatusBadRequest, "invalid_prompt", "prompt is too long")
		return
	}

	session, _ := builderSessionFromContext(r.Context())
	userID := ""
	if session.User != nil {
		userID = session.User.ID
	}
	if !h.limiter.Allow(r.Context(), scope.WorkspaceID, userID, "entry_reprompt") {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many generation requests; please wait before trying again")
		return
	}

	draft, err := h.reader.LoadDraft(r.Context(), siteID)
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	collectionIndex := findCollection(draft.Collections, collectionID)
	if collectionIndex == -1 {
		writeCollectionError(w, ErrCollectionNotFound)
		return
	}
	collection := draft.Collections[collectionIndex]
	entryIndex := findEntry(collection.Entries, entryID)
	if entryIndex == -1 {
		writeCollectionError(w, ErrEntryNotFound)
		return
	}
	entry := collection.Entries[entryIndex]

	jobID, err := h.jobs.CreateJob(r.Context(), generation.PromptActionInput{
		WorkspaceID: scope.WorkspaceID,
		UserID:      userID,
		SiteID:      siteID,
		Kind:        generation.JobKindEntryDraft,
		Prompt:      prompt,
		Payload: map[string]any{
			"scope":        "entry_reprompt",
			"siteId":       siteID,
			"collectionId": collectionID,
			"entryId":      entryID,
			"prompt":       prompt,
		},
	})
	if err != nil {
		writeCollectionError(w, err)
		return
	}
	if err := h.jobs.UpdateProgress(r.Context(), jobID, generation.JobKindEntryDraft, "prompt.normalize"); err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	if err := h.jobs.UpdateProgress(r.Context(), jobID, generation.JobKindEntryDraft, "plan.blocks"); err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}

	rewritten, err := h.drafter.RewriteEntry(r.Context(), EntryRewriteRequest{
		Prompt:   prompt,
		SiteName: draft.Site.Name,
		SiteGoal: draft.Site.SEO.Description,
		Collection: EntryDraftCollection{
			SingularLabel: collection.SingularLabel,
			PluralLabel:   collection.PluralLabel,
			Slug:          collection.Slug,
			Schema:        collection.Schema,
		},
		Entry: EntryDraft{
			Slug:   entry.Slug,
			Fields: cloneAnyMap(entry.Fields),
			SEO:    entry.SEO,
		},
	})
	if err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		if errors.Is(err, ErrCollectionDrafterUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "drafter_unavailable", "the AI entry drafter is not available")
			return
		}
		writeError(w, http.StatusBadGateway, "drafter_failed", err.Error())
		return
	}
	if err := h.jobs.UpdateProgress(r.Context(), jobID, generation.JobKindEntryDraft, "copy.write"); err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	if err := h.jobs.UpdateProgress(r.Context(), jobID, generation.JobKindEntryDraft, "validate.repair"); err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}

	nextFields := cloneAnyMap(entry.Fields)
	for key, value := range sanitizeEntryDraftFields(rewritten.Entry.Fields, collection.Schema) {
		nextFields[key] = value
	}
	nextSEO := entry.SEO
	if title := strings.TrimSpace(rewritten.Entry.SEO.Title); title != "" {
		nextSEO.Title = title
	}
	if description := strings.TrimSpace(rewritten.Entry.SEO.Description); description != "" {
		nextSEO.Description = description
	}
	nextSlug := entry.Slug
	if slug := strings.TrimSpace(rewritten.Entry.Slug); slug != "" {
		nextSlug = slug
	}
	fieldsPatch := replaceEntryFields(entry.Fields, nextFields)
	previousDraft := draft
	updatedEntry, err := h.mutator.UpdateEntry(r.Context(), scope.WorkspaceID, siteID, collectionID, entryID, UpdateEntryInput{
		Slug:   &nextSlug,
		Fields: fieldsPatch,
		SEO:    &nextSEO,
	})
	if err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	nextDraft, err := h.reader.LoadDraft(r.Context(), siteID)
	if err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	changeSummary := strings.TrimSpace(rewritten.ChangeSummary)
	if changeSummary == "" {
		changeSummary = entryRewriteSummary(collection, updatedEntry)
	}
	historyResult, err := h.recordHistory(r.Context(), generation.PromptHistoryInput{
		WorkspaceID:   scope.WorkspaceID,
		SiteID:        siteID,
		UserID:        userID,
		JobID:         jobID,
		Scope:         "entry",
		TargetID:      entryID,
		Prompt:        prompt,
		ChangeSummary: changeSummary,
		PreviousDraft: previousDraft,
		NextDraft:     nextDraft,
		Summary: map[string]any{
			"collectionId": collectionID,
			"entryId":      entryID,
			"slug":         updatedEntry.Slug,
		},
	})
	if err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	if err := h.jobs.CompleteJob(r.Context(), jobID, siteID, map[string]any{
		"entry":              updatedEntry,
		"historyId":          historyResult.HistoryID,
		"resultRevisionId":   historyResult.ResultRevisionID,
		"previousRevisionId": historyResult.PreviousRevisionID,
	}); err != nil {
		_ = h.jobs.FailJob(r.Context(), jobID, err)
		writeCollectionError(w, err)
		return
	}
	h.recordAudit(r.Context(), audit.Event{
		WorkspaceID: scope.WorkspaceID,
		SiteID:      siteID,
		UserID:      userID,
		Action:      "collection.entry_reprompt",
		Metadata: map[string]any{
			"jobId":              jobID,
			"collectionId":       collectionID,
			"entryId":            entryID,
			"historyId":          historyResult.HistoryID,
			"previousRevisionId": historyResult.PreviousRevisionID,
			"resultRevisionId":   historyResult.ResultRevisionID,
		},
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"entry":              updatedEntry,
		"jobId":              jobID,
		"historyId":          historyResult.HistoryID,
		"previousRevisionId": historyResult.PreviousRevisionID,
		"resultRevisionId":   historyResult.ResultRevisionID,
	})
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
	var migrationRequired *SchemaMigrationRequiredError
	var migrationIncomplete *SchemaMigrationIncompleteError
	switch {
	case errors.As(err, &migrationRequired):
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": map[string]string{
				"code":    "schema_migration_required",
				"message": "this change would lose or invalidate stored entry data; call the schema/migrate endpoint with explicit mappings",
			},
			"diff":     migrationRequired.Diff,
			"unmapped": migrationRequired.Unmapped,
		})
	case errors.As(err, &migrationIncomplete):
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": map[string]string{
				"code":    "schema_migration_incomplete",
				"message": "the migration is missing acknowledgements for at least one destructive change",
			},
			"diff":     migrationIncomplete.Diff,
			"unmapped": migrationIncomplete.Unmapped,
		})
	case errors.Is(err, generation.ErrGenerationRateLimited):
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many generation requests; please wait before trying again")
	case isPromptQuotaError(err, "trial_exhausted"):
		writeError(w, http.StatusForbidden, "trial_exhausted", err.Error())
	case isPromptQuotaError(err, "plan_limit_exceeded"):
		writeError(w, http.StatusForbidden, "plan_limit_exceeded", err.Error())
	case errors.Is(err, billing.ErrPlanLimitExceeded):
		writeError(w, http.StatusForbidden, "plan_limit_exceeded", err.Error())
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
	case errors.Is(err, sites.ErrDraftConflict):
		writeError(w, http.StatusConflict, "draft_conflict", "this draft changed while your edit was in flight; reload the latest version and try again")
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

func (h *Handler) recordHistory(ctx context.Context, input generation.PromptHistoryInput) (generation.PromptHistoryResult, error) {
	if h == nil || h.history == nil {
		return generation.PromptHistoryResult{}, nil
	}
	result, err := h.history.Record(ctx, input)
	if err != nil {
		if h.logger != nil {
			h.logger.Error("record collection prompt history",
				"scope", input.Scope,
				"siteId", input.SiteID,
				"workspaceId", input.WorkspaceID,
				"error", err.Error(),
			)
		}
	}
	return result, err
}

func collectionDraftSummary(collection siteconfig.Collection) string {
	label := strings.TrimSpace(collection.PluralLabel)
	if label == "" {
		label = strings.TrimSpace(collection.SingularLabel)
	}
	if label == "" {
		return "Drafted a new collection."
	}
	return fmt.Sprintf("Drafted the %s collection.", label)
}

func entryDraftSummary(collection siteconfig.Collection, count int) string {
	label := strings.TrimSpace(collection.PluralLabel)
	if label == "" {
		label = strings.TrimSpace(collection.SingularLabel)
	}
	if label == "" {
		label = "entries"
	}
	if count == 1 {
		return fmt.Sprintf("Drafted 1 %s entry.", label)
	}
	return fmt.Sprintf("Drafted %d %s entries.", count, label)
}

func entryRewriteSummary(collection siteconfig.Collection, entry siteconfig.CollectionEntry) string {
	label := strings.TrimSpace(collection.SingularLabel)
	if label == "" {
		label = "entry"
	}
	title := strings.TrimSpace(entryTitle(entry.Fields, collection.Schema))
	if title == "" {
		return fmt.Sprintf("Rewrote the %s entry.", strings.ToLower(label))
	}
	return fmt.Sprintf("Rewrote %s.", title)
}

func replaceEntryFields(current map[string]any, next map[string]any) map[string]any {
	patch := map[string]any{}
	for key := range current {
		if _, ok := next[key]; !ok {
			patch[key] = nil
		}
	}
	for key, value := range next {
		patch[key] = value
	}
	return patch
}

func (h *Handler) recordAudit(ctx context.Context, event audit.Event) {
	if h == nil || h.recorder == nil {
		return
	}
	if err := h.recorder.Record(ctx, event); err != nil && h.logger != nil {
		h.logger.Warn("record audit event",
			"action", event.Action,
			"siteId", event.SiteID,
			"workspaceId", event.WorkspaceID,
			"error", err.Error(),
		)
	}
}

func isPromptQuotaError(err error, code string) bool {
	var quotaErr *generation.PromptQuotaExceededError
	return errors.As(err, &quotaErr) && quotaErr.Code == code
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

func requiredUnsupportedEntryDraftField(schema []siteconfig.FieldDefinition) string {
	for _, field := range schema {
		if !field.Required {
			continue
		}
		switch field.Type {
		case siteconfig.FieldTypeAsset, siteconfig.FieldTypeAssetList, siteconfig.FieldTypeReference:
			return field.Label
		}
	}
	return ""
}

func sanitizeEntryDraftFields(fields map[string]any, schema []siteconfig.FieldDefinition) map[string]any {
	allowed := make(map[string]siteconfig.FieldDefinition, len(schema))
	for _, field := range schema {
		allowed[field.Key] = field
	}
	out := map[string]any{}
	for key, value := range fields {
		field, ok := allowed[key]
		if !ok || value == nil {
			continue
		}
		switch field.Type {
		case siteconfig.FieldTypeAsset, siteconfig.FieldTypeAssetList, siteconfig.FieldTypeReference:
			continue
		}
		out[key] = value
	}
	return out
}
