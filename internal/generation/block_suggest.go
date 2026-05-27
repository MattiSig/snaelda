package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/MattiSig/snaelda/internal/platform/audit"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// BlockSuggestAction enumerates the supported AI-suggest actions on a block.
// These map to the spec 20 dropdown entries (Tighten / Expand / Change tone /
// Rewrite from prompt).
const (
	BlockSuggestActionTighten = "tighten"
	BlockSuggestActionExpand  = "expand"
	BlockSuggestActionTone    = "tone"
	BlockSuggestActionRewrite = "rewrite"
)

// BlockSuggestTone enumerates the tones the "Change tone" action supports.
const (
	BlockSuggestToneFriendlier   = "friendlier"
	BlockSuggestToneProfessional = "professional"
	BlockSuggestTonePlayful      = "playful"
	BlockSuggestToneDirect       = "direct"
)

var (
	ErrBlockSuggestActionUnknown = errors.New("block suggest action is unknown")
	ErrBlockSuggestToneRequired  = errors.New("block suggest tone is required")
	ErrBlockSuggestPromptMissing = errors.New("block suggest rewrite prompt is required")
	ErrBlockSuggestUnavailable   = errors.New("block suggest is not configured")
	ErrBlockSuggestNotFound      = errors.New("block was not found in the draft")
)

// BlockSuggestInput captures the user-initiated AI-suggest action on a single
// block. Exactly one of Action ∈ {tighten, expand, tone, rewrite} is accepted.
// Tone requires a Tone value; Rewrite requires Instruction.
type BlockSuggestInput struct {
	Action      string
	Tone        string
	Instruction string
}

// BlockSuggester rewrites the props of a single block using the model. The
// implementation must constrain the result to the block's existing
// type/version and prop schema; only props change.
type BlockSuggester interface {
	SuggestBlockProps(ctx context.Context, request BlockSuggestRequest) (BlockSuggestResponse, error)
}

// BlockSuggestRequest is the structured input shipped to the BlockSuggester.
// It carries the block definition (which constrains the schema), the existing
// props (which the model rewrites), and the surrounding site context (page
// title, neighboring blocks) the model uses to keep the suggestion grounded.
type BlockSuggestRequest struct {
	Action       string
	Tone         string
	Instruction  string
	Block        siteconfig.BlockInstance
	Definition   siteconfig.BlockDefinition
	PageTitle    string
	PageSlug     string
	SiteName     string
	SiteGoal     string
	NeighborText []string
}

// BlockSuggestResponse is the result of a block-suggest call. Props must
// validate against the block's existing PropSchema. ChangeSummary is a short
// model-authored sentence used in the reprompt history.
type BlockSuggestResponse struct {
	Props         map[string]any
	ChangeSummary string
}

// SuggestBlock applies an AI-suggest action to a single block in the draft.
// On success it: (1) captures a pre/post draft revision pair, (2) writes a
// reprompt_history entry scoped to the block, (3) creates and completes a
// generation_jobs row, and (4) returns the updated draft.
func (s *Service) SuggestBlock(
	ctx context.Context,
	workspaceID string,
	userID string,
	siteID string,
	blockID string,
	input BlockSuggestInput,
) (GenerateResult, error) {
	if s.suggester == nil {
		return GenerateResult{}, ErrBlockSuggestUnavailable
	}
	action, tone, instruction, err := normalizeBlockSuggestInput(input)
	if err != nil {
		return GenerateResult{}, err
	}
	s.pruneGenerationJobs(ctx)

	currentDraft, err := s.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return GenerateResult{}, err
	}

	pageIndex, blockIndex, ok := findDraftBlockIndex(currentDraft, blockID)
	if !ok {
		return GenerateResult{}, ErrBlockSuggestNotFound
	}
	page := currentDraft.Pages[pageIndex]
	block := page.Blocks[blockIndex]

	definition, err := siteconfig.DefaultBlockRegistry().Lookup(block.Type, block.Version)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("lookup block definition: %w", err)
	}

	metadata, err := s.loadSiteMetadata(ctx, workspaceID, siteID)
	if err != nil {
		return GenerateResult{}, err
	}
	historyPrompt := describeBlockSuggest(action, tone, instruction)
	previousRevisionID, err := s.captureDraftRevision(ctx, workspaceID, siteID, draftRevisionRecord{
		Scope:                 "block",
		PageID:                page.ID,
		Prompt:                historyPrompt,
		Draft:                 currentDraft,
		GenerationPrompt:      metadata.Prompt,
		GenerationSummaryJSON: metadata.SummaryJSON,
		CreatedBy:             userID,
	})
	if err != nil {
		return GenerateResult{}, err
	}

	jobID, err := s.createGenerationJob(ctx, workspaceID, userID, JobKindBlockSuggest, generationInputContext{
		SiteID:   siteID,
		PageID:   page.ID,
		NameHint: currentDraft.Site.Name,
		Prompt:   historyPrompt,
		Scope:    "block",
	})
	if err != nil {
		return GenerateResult{}, err
	}
	tracker := newProgressTracker(s, jobID, JobKindBlockSuggest, ProgressStepsForKind(JobKindBlockSuggest, false), nil)
	if err := tracker.emit(ctx, "prompt.normalize"); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}
	if err := tracker.emit(ctx, "plan.blocks"); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}

	suggestResp, err := s.suggester.SuggestBlockProps(ctx, BlockSuggestRequest{
		Action:       action,
		Tone:         tone,
		Instruction:  instruction,
		Block:        block,
		Definition:   definition,
		PageTitle:    page.Title,
		PageSlug:     page.Slug,
		SiteName:     currentDraft.Site.Name,
		SiteGoal:     currentDraft.Site.SEO.Description,
		NeighborText: collectNeighborText(page.Blocks, blockIndex),
	})
	if err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}
	if len(suggestResp.Props) == 0 {
		err := fmt.Errorf("block suggest returned empty props")
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}

	if err := tracker.emit(ctx, "validate.repair"); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}

	nextDraft := cloneDraftShallow(currentDraft)
	nextDraft.Pages[pageIndex].Blocks[blockIndex].Props = suggestResp.Props

	if err := siteconfig.ValidateDraft(nextDraft); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}

	if err := tracker.emit(ctx, "persist"); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}
	if err := s.writer.SaveDraft(ctx, workspaceID, nextDraft); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}

	suggestSummary, err := json.Marshal(map[string]any{
		"action":  action,
		"tone":    tone,
		"blockId": blockID,
		"pageId":  page.ID,
	})
	if err != nil {
		return GenerateResult{}, fmt.Errorf("encode block suggest summary: %w", err)
	}

	savedDraft, err := s.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return GenerateResult{}, err
	}
	resultRevisionID, err := s.captureDraftRevision(ctx, workspaceID, siteID, draftRevisionRecord{
		Scope:                 "block",
		PageID:                page.ID,
		Prompt:                historyPrompt,
		Draft:                 savedDraft,
		GenerationPrompt:      metadata.Prompt,
		GenerationSummaryJSON: suggestSummary,
		CreatedBy:             userID,
	})
	if err != nil {
		return GenerateResult{}, err
	}

	summary := strings.TrimSpace(suggestResp.ChangeSummary)
	if summary == "" {
		summary = describeBlockSuggestSummary(action, tone, definition.DisplayName)
	}
	if err := s.recordRepromptHistory(ctx, workspaceID, siteID, repromptHistoryRecord{
		Scope:              "block",
		TargetID:           blockID,
		Prompt:             historyPrompt,
		ChangeSummary:      summary,
		PreviousRevisionID: previousRevisionID,
		ResultRevisionID:   resultRevisionID,
		JobID:              jobID,
		CreatedBy:          userID,
	}); err != nil {
		return GenerateResult{}, err
	}

	if err := s.completeGenerationJob(ctx, jobID, siteID, generationPlan{
		SiteName:    currentDraft.Site.Name,
		SiteGoal:    currentDraft.Site.SEO.Description,
		ThemePreset: metadata.themePreset(),
		Theme:       currentDraft.Theme,
		Pages: []generationPagePlan{{
			Title: page.Title,
			Slug:  page.Slug,
			Blocks: []generationBlockPlan{{
				Type:    block.Type,
				Purpose: summary,
				Props:   suggestResp.Props,
			}},
		}},
	}); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}
	if err := s.incrementTrialPromptUsage(ctx, workspaceID); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return GenerateResult{}, err
	}

	s.recordAudit(ctx, audit.Event{
		WorkspaceID: workspaceID,
		SiteID:      siteID,
		UserID:      userID,
		Action:      "block.suggest",
		Metadata: map[string]any{
			"jobId":   jobID,
			"blockId": blockID,
			"pageId":  page.ID,
			"action":  action,
			"tone":    tone,
		},
	})
	return GenerateResult{JobID: jobID, Draft: savedDraft}, nil
}

func findDraftBlockIndex(draft siteconfig.SiteDraft, blockID string) (int, int, bool) {
	for pageIndex, page := range draft.Pages {
		for blockIndex, block := range page.Blocks {
			if block.ID == blockID {
				return pageIndex, blockIndex, true
			}
		}
	}
	return -1, -1, false
}

func normalizeBlockSuggestInput(input BlockSuggestInput) (action string, tone string, instruction string, err error) {
	action = strings.ToLower(strings.TrimSpace(input.Action))
	tone = strings.ToLower(strings.TrimSpace(input.Tone))
	instruction = strings.TrimSpace(input.Instruction)
	switch action {
	case BlockSuggestActionTighten, BlockSuggestActionExpand:
		return action, "", "", nil
	case BlockSuggestActionTone:
		switch tone {
		case BlockSuggestToneFriendlier,
			BlockSuggestToneProfessional,
			BlockSuggestTonePlayful,
			BlockSuggestToneDirect:
			return action, tone, "", nil
		default:
			return "", "", "", ErrBlockSuggestToneRequired
		}
	case BlockSuggestActionRewrite:
		if instruction == "" {
			return "", "", "", ErrBlockSuggestPromptMissing
		}
		if len(instruction) > maxGenerationPromptCharacters {
			return "", "", "", fmt.Errorf("%w: %d", ErrPromptTooLong, maxGenerationPromptCharacters)
		}
		return action, "", instruction, nil
	default:
		return "", "", "", ErrBlockSuggestActionUnknown
	}
}

func collectNeighborText(blocks []siteconfig.BlockInstance, index int) []string {
	values := make([]string, 0, 4)
	for offset := -2; offset <= 2; offset++ {
		if offset == 0 {
			continue
		}
		neighborIndex := index + offset
		if neighborIndex < 0 || neighborIndex >= len(blocks) {
			continue
		}
		text := strings.TrimSpace(extractBlockHeadlineText(blocks[neighborIndex].Props))
		if text != "" {
			values = append(values, text)
		}
	}
	return values
}

func extractBlockHeadlineText(props map[string]any) string {
	if props == nil {
		return ""
	}
	for _, key := range []string{"headline", "heading", "title"} {
		if value, ok := props[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func describeBlockSuggest(action string, tone string, instruction string) string {
	switch action {
	case BlockSuggestActionTighten:
		return "Tighten the block copy"
	case BlockSuggestActionExpand:
		return "Expand the block copy"
	case BlockSuggestActionTone:
		return "Change tone to " + tone
	case BlockSuggestActionRewrite:
		return "Rewrite: " + instruction
	default:
		return "Improve with AI"
	}
}

func describeBlockSuggestSummary(action string, tone string, displayName string) string {
	if displayName == "" {
		displayName = "block"
	}
	switch action {
	case BlockSuggestActionTighten:
		return fmt.Sprintf("Tightened the %s copy.", strings.ToLower(displayName))
	case BlockSuggestActionExpand:
		return fmt.Sprintf("Expanded the %s copy.", strings.ToLower(displayName))
	case BlockSuggestActionTone:
		return fmt.Sprintf("Shifted the %s tone to %s.", strings.ToLower(displayName), tone)
	case BlockSuggestActionRewrite:
		return fmt.Sprintf("Rewrote the %s from your prompt.", strings.ToLower(displayName))
	default:
		return fmt.Sprintf("Updated the %s with AI.", strings.ToLower(displayName))
	}
}

