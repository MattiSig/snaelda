package generation

import (
	"context"
	"errors"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// PageChangeSetAction enumerates the verbs the change-set planner can pick
// for each block in a page when interpreting a reprompt.
const (
	PageChangeSetActionKeep   = "keep"
	PageChangeSetActionEdit   = "edit"
	PageChangeSetActionRemove = "remove"
	PageChangeSetActionInsert = "insert"
)

// ErrPageChangeSetUnavailable signals that the diff-style page reprompt path
// is not configured (planner or block suggester missing). Callers should fall
// back to the legacy whole-page reprompt.
var ErrPageChangeSetUnavailable = errors.New("page change-set planner is not configured")

// ErrPageChangeSetEmpty signals that the change-set planner returned no
// operations. Callers should fall back to the legacy whole-page reprompt.
var ErrPageChangeSetEmpty = errors.New("page change-set returned no operations")

// PageChangeSetPlanner decides which blocks on a page to keep, edit, remove,
// or insert in response to a user reprompt. It is intentionally narrow: the
// model returns a small list of operations, not block copy. Block copy is
// produced separately by per-block calls so the change-set call stays cheap
// and the rewrites can run in parallel.
type PageChangeSetPlanner interface {
	PlanPageChanges(ctx context.Context, request PageChangeSetRequest) (PageChangeSetResponse, error)
}

// PageChangeSetRequest carries the minimal context needed for the model to
// decide which blocks to keep, edit, remove, or insert: the user's reprompt
// directive, the current page, neighbor pages, and the brand. No theme
// catalog and no full block registry — only the block types already on the
// page plus a short list of allowed insertable types.
type PageChangeSetRequest struct {
	SiteName          string                 `json:"siteName"`
	SiteGoal          string                 `json:"siteGoal,omitempty"`
	PreferredLanguage string                 `json:"preferredLanguage,omitempty"`
	Brand             siteconfig.BrandConfig `json:"brand,omitempty"`
	Page              PageChangeSetPage      `json:"page"`
	NeighborPages     []NeighborPage         `json:"neighborPages,omitempty"`
	InsertableTypes   []InsertableBlockType  `json:"insertableTypes,omitempty"`
	Prompt            string                 `json:"prompt"`
}

// PageChangeSetPage is the in-flight description of the page being reprompted.
type PageChangeSetPage struct {
	Title  string                  `json:"title"`
	Slug   string                  `json:"slug"`
	Blocks []ChangeSetBlockSummary `json:"blocks"`
}

// ChangeSetBlockSummary describes one existing block to the model. We do not
// send the full props — only the type and a short textual summary so the
// model can reason about which blocks to touch without paying for the props.
type ChangeSetBlockSummary struct {
	BlockID string `json:"blockId"`
	Type    string `json:"type"`
	Summary string `json:"summary,omitempty"`
}

// NeighborPage is a sibling page surfaced to the planner so it can avoid
// duplicating content that already lives elsewhere on the site.
type NeighborPage struct {
	Title string `json:"title"`
	Slug  string `json:"slug"`
}

// InsertableBlockType names a block type the planner may insert and gives the
// model a short description so it can pick sensible additions.
type InsertableBlockType struct {
	Type        string `json:"type"`
	DisplayName string `json:"displayName,omitempty"`
	Category    string `json:"category,omitempty"`
}

// PageChangeSetResponse is the structured reply from the change-set planner.
// Operations apply in order against the current block list.
type PageChangeSetResponse struct {
	Operations    []PageChangeSetOperation `json:"operations"`
	ChangeSummary string                   `json:"changeSummary,omitempty"`
}

// PageChangeSetOperation is a single op in the change set. Semantics by Action:
//   - keep:   BlockID identifies an existing block to copy through unchanged.
//   - edit:   BlockID identifies an existing block; Purpose is the rewrite directive.
//   - remove: BlockID identifies an existing block to drop.
//   - insert: Type names the new block; Purpose describes what it should say.
type PageChangeSetOperation struct {
	Action  string `json:"action"`
	BlockID string `json:"blockId,omitempty"`
	Type    string `json:"type,omitempty"`
	Purpose string `json:"purpose,omitempty"`
	Reason  string `json:"reason,omitempty"`
}
