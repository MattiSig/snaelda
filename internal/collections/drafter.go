package collections

import (
	"context"
	"errors"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// CollectionDrafter turns a free-form user prompt into a proposed collection
// shape (labels, slug, schema). It is the contract the OpenAI planner
// satisfies so the collections page can offer a "prompt up a collection"
// affordance. Implementations are read-only — they never persist anything;
// the handler is responsible for storing the resulting draft.
type CollectionDrafter interface {
	DraftCollection(ctx context.Context, request CollectionDraftRequest) (CollectionDraftResponse, error)
}

// CollectionDraftRequest is the structured payload sent to the drafter. The
// site context is used so the model can pick fields that fit the brand and
// avoid duplicating existing collections.
type CollectionDraftRequest struct {
	Prompt              string   `json:"prompt"`
	SiteName            string   `json:"siteName,omitempty"`
	SiteGoal            string   `json:"siteGoal,omitempty"`
	ExistingCollections []string `json:"existingCollections,omitempty"`
}

// CollectionDraftResponse is what the drafter returns. The handler validates
// the schema and slug before persisting, so the model can be permissive here.
type CollectionDraftResponse struct {
	Slug          string                       `json:"slug,omitempty"`
	SingularLabel string                       `json:"singularLabel"`
	PluralLabel   string                       `json:"pluralLabel"`
	Schema        []siteconfig.FieldDefinition `json:"schema"`
}

// ErrCollectionDrafterUnavailable is returned when the drafter is not
// configured (e.g. no OpenAI key). The handler converts it to a 503.
var ErrCollectionDrafterUnavailable = errors.New("collection drafter is not configured")
