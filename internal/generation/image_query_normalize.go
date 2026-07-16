package generation

import (
	"context"
	"fmt"
	"strings"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// StarterImageQueryPlanner is the contract the OpenAI planner satisfies so
// starter-imagery enrichment can normalize every empty image slot's search
// query in one batched call. The draft copy is often not in English (and even
// English headlines make poor stock queries); the model rewrites each slot's
// local context into a short English stock-photo search phrase.
type StarterImageQueryPlanner interface {
	NormalizeStarterImageQueries(ctx context.Context, request StarterImageQueryRequest) ([]string, error)
}

// StarterImageSlotContext is the per-slot context sent to the model: the
// slot's own copy plus where it sits, all in the site's language.
type StarterImageSlotContext struct {
	BlockType string `json:"blockType"`
	PageTitle string `json:"pageTitle,omitempty"`
	Heading   string `json:"heading,omitempty"`
	Body      string `json:"body,omitempty"`
}

// StarterImageQueryRequest is one batched normalization request covering all
// empty image slots of a freshly generated draft.
type StarterImageQueryRequest struct {
	SiteName string                    `json:"siteName,omitempty"`
	SiteGoal string                    `json:"siteGoal,omitempty"`
	Locale   string                    `json:"locale,omitempty"`
	Slots    []StarterImageSlotContext `json:"slots"`
}

// starterImageSlotKey addresses one image slot by its walk position. The
// collector and the fill walk must visit slots in the same order (pages, then
// blocks, then gallery items); itemIndex is -1 for single-image blocks.
func starterImageSlotKey(pageIndex int, blockIndex int, itemIndex int) string {
	return fmt.Sprintf("%d/%d/%d", pageIndex, blockIndex, itemIndex)
}

// collectStarterImageSlots gathers every image slot that still needs an asset,
// in the exact order applyStarterImagery fills them, together with the local
// copy the model needs to write a search query for the slot.
func collectStarterImageSlots(draft siteconfig.SiteDraft) ([]string, []StarterImageSlotContext) {
	keys := []string{}
	slots := []StarterImageSlotContext{}
	appendSlot := func(key string, slot StarterImageSlotContext) {
		keys = append(keys, key)
		slots = append(slots, slot)
	}

	for pageIndex := range draft.Pages {
		page := &draft.Pages[pageIndex]
		for blockIndex := range page.Blocks {
			block := &page.Blocks[blockIndex]
			if block.Props == nil {
				continue
			}
			switch block.Type {
			case "hero":
				if _, needs := imageNeedsAsset(block.Props, "image"); needs {
					appendSlot(starterImageSlotKey(pageIndex, blockIndex, -1), StarterImageSlotContext{
						BlockType: block.Type,
						PageTitle: page.Title,
						Heading:   readGeneratedText(block.Props, "headline", 120),
						Body:      readGeneratedText(block.Props, "subheadline", 200),
					})
				}
			case "image_text":
				if _, needs := imageNeedsAsset(block.Props, "image"); needs {
					appendSlot(starterImageSlotKey(pageIndex, blockIndex, -1), StarterImageSlotContext{
						BlockType: block.Type,
						PageTitle: page.Title,
						Heading:   readGeneratedText(block.Props, "heading", 120),
						Body:      readGeneratedText(block.Props, "body", 200),
					})
				}
			case "gallery":
				raw, ok := block.Props["images"].([]any)
				if !ok {
					continue
				}
				for itemIndex := range raw {
					item, ok := raw[itemIndex].(map[string]any)
					if !ok {
						continue
					}
					if _, needs := imageNeedsAsset(item, "image"); !needs {
						continue
					}
					appendSlot(starterImageSlotKey(pageIndex, blockIndex, itemIndex), StarterImageSlotContext{
						BlockType: block.Type,
						PageTitle: page.Title,
						Heading:   readGeneratedText(item, "title", 120),
						Body:      readGeneratedText(item, "caption", 200),
					})
				}
			}
		}
	}
	return keys, slots
}

// normalizeStarterImageQueries runs the batched query-normalization call and
// returns the English search query per slot key. Any failure (planner not
// configured, call error, count mismatch) degrades to nil so enrichment falls
// back to the deterministic query chain — imagery must never fail the spin.
func (s *Service) normalizeStarterImageQueries(ctx context.Context, draft siteconfig.SiteDraft, prompt string) map[string]string {
	if s.imageQueryPlanner == nil {
		return nil
	}
	keys, slots := collectStarterImageSlots(draft)
	if len(slots) == 0 {
		return nil
	}
	queries, err := s.imageQueryPlanner.NormalizeStarterImageQueries(ctx, StarterImageQueryRequest{
		SiteName: draft.Site.Name,
		SiteGoal: prompt,
		Locale:   draft.Site.DefaultLocale,
		Slots:    slots,
	})
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("normalize starter image queries", "siteId", draft.Site.ID, "slots", len(slots), "error", err.Error())
		}
		return nil
	}
	if len(queries) != len(keys) {
		if s.logger != nil {
			s.logger.Warn("normalize starter image queries", "siteId", draft.Site.ID, "error", fmt.Sprintf("expected %d queries, got %d", len(keys), len(queries)))
		}
		return nil
	}
	normalized := make(map[string]string, len(keys))
	for index, key := range keys {
		if query := strings.TrimSpace(queries[index]); query != "" {
			normalized[key] = query
		}
	}
	return normalized
}

// prependQuery puts the normalized English query first in the fallback chain
// so the provider tries it before the deterministic (often non-English)
// queries. No-op when the slot has no normalized query.
func prependQuery(queries []string, query string) []string {
	clean := strings.TrimSpace(query)
	if clean == "" {
		return queries
	}
	for _, existing := range queries {
		if strings.EqualFold(existing, clean) {
			return queries
		}
	}
	return append([]string{clean}, queries...)
}
