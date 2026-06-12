package generation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MattiSig/snaelda/internal/platform/ids"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"golang.org/x/sync/errgroup"
)

// maxParallelBlockRewrites caps the number of concurrent SuggestBlockProps
// calls during a page reprompt. Generous enough that a typical 4-6 block page
// runs in roughly one rewrite-duration, conservative enough not to spike the
// model provider.
const maxParallelBlockRewrites = 4

// applyPageChangeSet runs the diff-style page reprompt:
//  1. ask the change-set planner which blocks to keep, edit, remove, insert;
//  2. preserve "keep" blocks verbatim with their existing IDs;
//  3. rewrite "edit" and seed "insert" blocks via SuggestBlockProps in parallel.
//
// Returns the updated PageDraft and a summary plan suitable for the metadata /
// completeGenerationJob path. When the planner or block suggester is not
// configured, returns ErrPageChangeSetUnavailable so the caller can fall back
// to the legacy whole-page reprompt.
func (s *Service) applyPageChangeSet(
	ctx context.Context,
	draft siteconfig.SiteDraft,
	page siteconfig.PageDraft,
	prompt string,
) (siteconfig.PageDraft, generationPagePlan, error) {
	nextPage, plan, _, err := s.applyPageChangeSetWithSummary(ctx, draft, page, prompt)
	return nextPage, plan, err
}

func (s *Service) applyPageChangeSetWithSummary(
	ctx context.Context,
	draft siteconfig.SiteDraft,
	page siteconfig.PageDraft,
	prompt string,
) (siteconfig.PageDraft, generationPagePlan, string, error) {
	if s.pageChangeSetPlanner == nil || s.suggester == nil {
		return siteconfig.PageDraft{}, generationPagePlan{}, "", ErrPageChangeSetUnavailable
	}

	request := buildPageChangeSetRequest(draft, page, prompt)
	response, err := s.pageChangeSetPlanner.PlanPageChanges(ctx, request)
	if err != nil {
		return siteconfig.PageDraft{}, generationPagePlan{}, "", err
	}
	if len(response.Operations) == 0 {
		return siteconfig.PageDraft{}, generationPagePlan{}, "", ErrPageChangeSetEmpty
	}

	pending, err := resolveChangeSetOperations(page, response.Operations)
	if err != nil {
		return siteconfig.PageDraft{}, generationPagePlan{}, "", err
	}

	finalBlocks := make([]siteconfig.BlockInstance, len(pending))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxParallelBlockRewrites)
	for i, op := range pending {
		i, op := i, op
		switch op.Action {
		case PageChangeSetActionKeep:
			finalBlocks[i] = op.Existing
		case PageChangeSetActionEdit, PageChangeSetActionInsert:
			g.Go(func() error {
				rewritten, err := s.draftBlockForChangeSet(gctx, draft, page, op, pending, i)
				if err != nil {
					return err
				}
				finalBlocks[i] = rewritten
				return nil
			})
		}
	}
	if err := g.Wait(); err != nil {
		return siteconfig.PageDraft{}, generationPagePlan{}, "", err
	}

	nextPage := page
	nextPage.Blocks = finalBlocks

	plan := summarizePageForMetadata(page, finalBlocks)
	return nextPage, plan, strings.TrimSpace(response.ChangeSummary), nil
}

// pendingBlock captures a single output slot before its props are produced.
// For Keep it carries the existing instance verbatim. For Edit it carries the
// existing instance whose props will be rewritten in place. For Insert it
// carries the new block type and a fresh ID; props are produced from scratch.
type pendingBlock struct {
	Action   string
	Existing siteconfig.BlockInstance
	Type     string
	Purpose  string
}

func resolveChangeSetOperations(
	page siteconfig.PageDraft,
	operations []PageChangeSetOperation,
) ([]pendingBlock, error) {
	blocksByID := make(map[string]siteconfig.BlockInstance, len(page.Blocks))
	for _, block := range page.Blocks {
		blocksByID[block.ID] = block
	}

	pending := make([]pendingBlock, 0, len(operations))
	for _, op := range operations {
		switch op.Action {
		case PageChangeSetActionKeep:
			existing, ok := blocksByID[strings.TrimSpace(op.BlockID)]
			if !ok {
				continue
			}
			pending = append(pending, pendingBlock{
				Action:   PageChangeSetActionKeep,
				Existing: existing,
			})
		case PageChangeSetActionEdit:
			existing, ok := blocksByID[strings.TrimSpace(op.BlockID)]
			if !ok {
				continue
			}
			pending = append(pending, pendingBlock{
				Action:   PageChangeSetActionEdit,
				Existing: existing,
				Type:     existing.Type,
				Purpose:  strings.TrimSpace(op.Purpose),
			})
		case PageChangeSetActionRemove:
			continue
		case PageChangeSetActionInsert:
			blockType := strings.TrimSpace(op.Type)
			if blockType == "" {
				continue
			}
			pending = append(pending, pendingBlock{
				Action:  PageChangeSetActionInsert,
				Type:    blockType,
				Purpose: strings.TrimSpace(op.Purpose),
			})
		}
	}
	if len(pending) == 0 {
		return nil, ErrPageChangeSetEmpty
	}
	return pending, nil
}

func (s *Service) draftBlockForChangeSet(
	ctx context.Context,
	draft siteconfig.SiteDraft,
	page siteconfig.PageDraft,
	op pendingBlock,
	allPending []pendingBlock,
	index int,
) (siteconfig.BlockInstance, error) {
	registry := siteconfig.DefaultBlockRegistry()
	blockType := op.Type
	if blockType == "" {
		blockType = op.Existing.Type
	}
	definition, err := registry.Lookup(blockType, siteconfig.BlockVersionV1)
	if err != nil {
		return siteconfig.BlockInstance{}, fmt.Errorf("lookup %s for change-set: %w", blockType, err)
	}

	var block siteconfig.BlockInstance
	if op.Action == PageChangeSetActionEdit {
		block = op.Existing
	} else {
		newID, err := ids.New()
		if err != nil {
			return siteconfig.BlockInstance{}, fmt.Errorf("mint id for inserted block: %w", err)
		}
		block = siteconfig.BlockInstance{
			ID:      newID,
			Type:    blockType,
			Version: siteconfig.BlockVersionV1,
			Props:   map[string]any{},
		}
	}

	instruction := op.Purpose
	if instruction == "" {
		instruction = fmt.Sprintf("Update this %s to fit the latest direction for the page.", strings.ToLower(definition.DisplayName))
	}

	resp, err := s.suggester.SuggestBlockProps(ctx, BlockSuggestRequest{
		Action:       BlockSuggestActionRewrite,
		Instruction:  instruction,
		Block:        block,
		Definition:   definition,
		PageTitle:    page.Title,
		PageSlug:     page.Slug,
		SiteName:     draft.Site.Name,
		SiteGoal:     draft.Site.SEO.Description,
		NeighborText: pendingNeighborText(allPending, index),
	})
	if err != nil {
		return siteconfig.BlockInstance{}, err
	}
	if len(resp.Props) == 0 {
		return siteconfig.BlockInstance{}, errors.New("change-set rewrite returned empty props")
	}
	block.Props = resp.Props
	return block, nil
}

func pendingNeighborText(pending []pendingBlock, index int) []string {
	neighbors := make([]string, 0, 4)
	for offset := -2; offset <= 2; offset++ {
		if offset == 0 {
			continue
		}
		neighbor := index + offset
		if neighbor < 0 || neighbor >= len(pending) {
			continue
		}
		var text string
		if pending[neighbor].Action == PageChangeSetActionKeep || pending[neighbor].Action == PageChangeSetActionEdit {
			text = extractBlockHeadlineText(pending[neighbor].Existing.Props)
		}
		if text == "" {
			text = strings.TrimSpace(pending[neighbor].Purpose)
		}
		if text != "" {
			neighbors = append(neighbors, text)
		}
	}
	return neighbors
}

func buildPageChangeSetRequest(
	draft siteconfig.SiteDraft,
	page siteconfig.PageDraft,
	prompt string,
) PageChangeSetRequest {
	summaries := make([]ChangeSetBlockSummary, 0, len(page.Blocks))
	for _, block := range page.Blocks {
		summary := strings.TrimSpace(extractBlockHeadlineText(block.Props))
		summaries = append(summaries, ChangeSetBlockSummary{
			BlockID: block.ID,
			Type:    block.Type,
			Summary: summary,
		})
	}

	neighbors := make([]NeighborPage, 0, len(draft.Pages))
	for _, other := range draft.Pages {
		if other.ID == page.ID {
			continue
		}
		neighbors = append(neighbors, NeighborPage{
			Title: other.Title,
			Slug:  other.Slug,
		})
	}

	registry := siteconfig.DefaultBlockRegistry()
	definitions := registry.Definitions()
	insertable := make([]InsertableBlockType, 0, len(definitions))
	for _, definition := range definitions {
		insertable = append(insertable, InsertableBlockType{
			Type:        definition.Type,
			DisplayName: definition.DisplayName,
			Category:    string(definition.Category),
		})
	}

	return PageChangeSetRequest{
		SiteName:        draft.Site.Name,
		SiteGoal:        draft.Site.SEO.Description,
		Brand:           draft.Brand,
		Page:            PageChangeSetPage{Title: page.Title, Slug: page.Slug, Blocks: summaries},
		NeighborPages:   neighbors,
		InsertableTypes: insertable,
		Prompt:          prompt,
	}
}

func summarizePageForMetadata(page siteconfig.PageDraft, blocks []siteconfig.BlockInstance) generationPagePlan {
	planBlocks := make([]generationBlockPlan, 0, len(blocks))
	for _, block := range blocks {
		planBlocks = append(planBlocks, generationBlockPlan{
			Type:  block.Type,
			Props: block.Props,
		})
	}
	return generationPagePlan{
		Title:  page.Title,
		Slug:   page.Slug,
		Blocks: planBlocks,
		SEO:    page.SEO,
	}
}
