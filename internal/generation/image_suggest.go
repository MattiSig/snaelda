package generation

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/MattiSig/snaelda/internal/assets"
	"github.com/MattiSig/snaelda/internal/imagery"
	"github.com/MattiSig/snaelda/internal/platform/audit"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// ImageQueryRewriter is the contract the OpenAI planner satisfies so the
// "Find a better image" affordance can ask the model to rewrite a search
// query from the surrounding page/block context.
type ImageQueryRewriter interface {
	RewriteImageQuery(ctx context.Context, request ImageQueryRequest) (string, error)
}

// ImageQueryRequest is the structured payload we send the model so it can
// suggest a sharper Pexels search query than what the user could type from
// memory. It is read-only — the model never returns block props from this
// call, only the search query string.
type ImageQueryRequest struct {
	SiteName        string   `json:"siteName,omitempty"`
	SiteGoal        string   `json:"siteGoal,omitempty"`
	PageTitle       string   `json:"pageTitle,omitempty"`
	PageSlug        string   `json:"pageSlug,omitempty"`
	BlockType       string   `json:"blockType"`
	BlockHeadline   string   `json:"blockHeadline,omitempty"`
	BlockBody       string   `json:"blockBody,omitempty"`
	CurrentAlt      string   `json:"currentAlt,omitempty"`
	NeighborText    []string `json:"neighborText,omitempty"`
	UserInstruction string   `json:"userInstruction,omitempty"`
}

// ImageSuggestInput captures what the frontend asks for. Path identifies
// which image slot inside the block — for hero/image_text it is ["image"];
// for gallery it is ["images", "<index>", "image"].
type ImageSuggestInput struct {
	Path        []string
	Instruction string
}

// ImageSuggestCandidate is a single Pexels result returned to the picker.
// The frontend round-trips this verbatim to the apply endpoint when the user
// picks one — we re-validate it server-side before downloading.
type ImageSuggestCandidate struct {
	Provider    string `json:"provider"`
	ProviderID  string `json:"providerId"`
	DownloadURL string `json:"downloadUrl"`
	SourceURL   string `json:"sourceUrl,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Author      string `json:"author,omitempty"`
	AuthorURL   string `json:"authorUrl,omitempty"`
	License     string `json:"license,omitempty"`
	Description string `json:"description,omitempty"`
}

// ImageSuggestResult is what the read-only suggest endpoint returns.
type ImageSuggestResult struct {
	Query      string                  `json:"query"`
	Candidates []ImageSuggestCandidate `json:"candidates"`
}

// ImageApplyInput is the body of the apply endpoint — the chosen candidate,
// where to put it on the block, and the alt text the user confirms.
type ImageApplyInput struct {
	Path        []string
	Photo       ImageSuggestCandidate
	Alt         string
	Query       string
	Instruction string
}

// ImageApplyResult mirrors GenerateResult but also returns the new asset so
// the builder can refresh its library without an extra round-trip.
type ImageApplyResult struct {
	JobID string               `json:"jobId"`
	Draft siteconfig.SiteDraft `json:"draft"`
	Asset *assets.Asset        `json:"asset,omitempty"`
	Image map[string]any       `json:"image,omitempty"`
}

var (
	ErrImageSuggestUnavailable  = errors.New("image suggest is not configured")
	ErrImageSuggestInvalidPath  = errors.New("image path is invalid")
	ErrImageSuggestNoCandidates = errors.New("no image candidates returned for query")
	ErrImageSuggestMissingPhoto = errors.New("image candidate is incomplete")
)

// SuggestImage returns a model-derived query plus a fresh set of Pexels
// candidates for the requested block image slot. It is read-only — it does
// not change the draft, capture a revision, or spend a prompt budget unit.
func (s *Service) SuggestImage(
	ctx context.Context,
	workspaceID string,
	siteID string,
	blockID string,
	input ImageSuggestInput,
) (ImageSuggestResult, error) {
	if s.imagery == nil || !s.imagery.available() {
		return ImageSuggestResult{}, ErrImageSuggestUnavailable
	}
	path := normalizeImagePath(input.Path)
	if len(path) == 0 {
		return ImageSuggestResult{}, ErrImageSuggestInvalidPath
	}

	draft, err := s.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return ImageSuggestResult{}, err
	}

	pageIndex, blockIndex, ok := findDraftBlockIndex(draft, blockID)
	if !ok {
		return ImageSuggestResult{}, ErrBlockSuggestNotFound
	}
	page := draft.Pages[pageIndex]
	block := page.Blocks[blockIndex]
	if _, _, err := resolveImageSlot(block.Props, path); err != nil {
		return ImageSuggestResult{}, fmt.Errorf("%w: %v", ErrImageSuggestInvalidPath, err)
	}

	request := ImageQueryRequest{
		SiteName:        draft.Site.Name,
		SiteGoal:        draft.Site.SEO.Description,
		PageTitle:       page.Title,
		PageSlug:        page.Slug,
		BlockType:       block.Type,
		BlockHeadline:   extractBlockHeadlineText(block.Props),
		BlockBody:       extractBlockBodyText(block.Props),
		CurrentAlt:      currentImageAlt(block.Props, path),
		NeighborText:    collectNeighborText(page.Blocks, blockIndex),
		UserInstruction: strings.TrimSpace(input.Instruction),
	}

	query := ""
	if s.imageRewriter != nil {
		modelQuery, modelErr := s.imageRewriter.RewriteImageQuery(ctx, request)
		if modelErr == nil {
			query = strings.TrimSpace(modelQuery)
		} else if s.logger != nil {
			s.logger.Warn("image query rewrite failed",
				"siteId", siteID,
				"blockId", blockID,
				"error", modelErr.Error(),
			)
		}
	}
	if query == "" {
		query = fallbackImageQuery(request)
	}
	if query == "" {
		return ImageSuggestResult{}, ErrImageSuggestNoCandidates
	}

	provider := s.imagery.Inner()
	if provider == nil {
		return ImageSuggestResult{}, ErrImageSuggestUnavailable
	}

	photos, err := provider.Search(ctx, imagery.SearchInput{
		Query:       query,
		Orientation: orientationForBlock(block.Type),
		Count:       9,
	})
	if err != nil {
		if errors.Is(err, imagery.ErrNoResults) {
			return ImageSuggestResult{}, ErrImageSuggestNoCandidates
		}
		return ImageSuggestResult{}, fmt.Errorf("image provider search: %w", err)
	}
	if len(photos) == 0 {
		return ImageSuggestResult{}, ErrImageSuggestNoCandidates
	}

	candidates := make([]ImageSuggestCandidate, 0, len(photos))
	for _, photo := range photos {
		candidates = append(candidates, candidateFromPhoto(photo))
	}
	return ImageSuggestResult{Query: query, Candidates: candidates}, nil
}

// ApplyImageSuggestion downloads the chosen candidate, imports it as a site
// asset, swaps the block's image slot, captures before/after draft revisions,
// records a block-scoped reprompt history row, and finalizes a generation
// job. This is the only path that mutates the draft for image-suggest, and
// it does spend a prompt budget unit.
func (s *Service) ApplyImageSuggestion(
	ctx context.Context,
	workspaceID string,
	userID string,
	siteID string,
	blockID string,
	input ImageApplyInput,
) (ImageApplyResult, error) {
	if s.imagery == nil || !s.imagery.available() {
		return ImageApplyResult{}, ErrImageSuggestUnavailable
	}
	if s.assetImporter == nil {
		return ImageApplyResult{}, ErrImageSuggestUnavailable
	}
	path := normalizeImagePath(input.Path)
	if len(path) == 0 {
		return ImageApplyResult{}, ErrImageSuggestInvalidPath
	}
	if strings.TrimSpace(input.Photo.DownloadURL) == "" || strings.TrimSpace(input.Photo.Provider) == "" {
		return ImageApplyResult{}, ErrImageSuggestMissingPhoto
	}

	s.pruneGenerationJobs(ctx)

	currentDraft, err := s.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return ImageApplyResult{}, err
	}

	pageIndex, blockIndex, ok := findDraftBlockIndex(currentDraft, blockID)
	if !ok {
		return ImageApplyResult{}, ErrBlockSuggestNotFound
	}
	page := currentDraft.Pages[pageIndex]
	block := page.Blocks[blockIndex]
	if _, _, err := resolveImageSlot(block.Props, path); err != nil {
		return ImageApplyResult{}, fmt.Errorf("%w: %v", ErrImageSuggestInvalidPath, err)
	}

	metadata, err := s.loadSiteMetadata(ctx, workspaceID, siteID)
	if err != nil {
		return ImageApplyResult{}, err
	}
	historyPrompt := describeImageApply(input)
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
		return ImageApplyResult{}, err
	}

	jobID, err := s.createGenerationJob(ctx, workspaceID, userID, JobKindBlockSuggest, generationInputContext{
		SiteID:   siteID,
		PageID:   page.ID,
		NameHint: currentDraft.Site.Name,
		Prompt:   historyPrompt,
		Scope:    "block",
	})
	if err != nil {
		return ImageApplyResult{}, err
	}
	tracker := newProgressTracker(s, jobID, JobKindBlockSuggest, ProgressStepsForKind(JobKindBlockSuggest, true), nil)
	if err := tracker.emit(ctx, "prompt.normalize"); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return ImageApplyResult{}, err
	}
	if err := tracker.emit(ctx, "plan.blocks"); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return ImageApplyResult{}, err
	}
	if err := tracker.emit(ctx, "assets.fetch"); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return ImageApplyResult{}, err
	}

	provider := s.imagery.Inner()
	if provider == nil {
		err := ErrImageSuggestUnavailable
		_ = s.failGenerationJob(ctx, jobID, err)
		return ImageApplyResult{}, err
	}
	photoData, err := provider.Download(ctx, imagery.Photo{
		Provider:    input.Photo.Provider,
		ProviderID:  input.Photo.ProviderID,
		Width:       input.Photo.Width,
		Height:      input.Photo.Height,
		DownloadURL: input.Photo.DownloadURL,
		ContentType: input.Photo.ContentType,
		SourceURL:   input.Photo.SourceURL,
		Author:      input.Photo.Author,
		AuthorURL:   input.Photo.AuthorURL,
		License:     input.Photo.License,
		Description: input.Photo.Description,
	})
	if err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return ImageApplyResult{}, fmt.Errorf("download image candidate: %w", err)
	}

	contentType := strings.ToLower(strings.TrimSpace(photoData.ContentType))
	if contentType == "" {
		contentType = "image/jpeg"
	}
	fileName := buildStarterFileName(photoData.Photo, contentType)
	altText := clampSentence(strings.TrimSpace(input.Alt), 180)
	if altText == "" {
		altText = clampSentence(strings.TrimSpace(input.Photo.Description), 180)
	}
	if altText == "" {
		altText = clampSentence(currentImageAlt(block.Props, path), 180)
	}
	if altText == "" {
		altText = clampSentence(extractBlockHeadlineText(block.Props), 180)
	}
	if altText == "" {
		altText = clampSentence(currentDraft.Site.Name, 180)
	}
	queryLabel := strings.TrimSpace(input.Query)
	if queryLabel == "" {
		queryLabel = fallbackImageQuery(ImageQueryRequest{
			BlockType:     block.Type,
			BlockHeadline: extractBlockHeadlineText(block.Props),
			PageTitle:     page.Title,
		})
	}

	asset, err := s.assetImporter.ImportExternal(ctx, assets.ImportExternalInput{
		WorkspaceID: workspaceID,
		SiteID:      siteID,
		UserID:      userID,
		FileName:    fileName,
		ContentType: contentType,
		Body:        photoData.Body,
		AltText:     altText,
		Width:       photoData.Width,
		Height:      photoData.Height,
		Provenance: assets.AssetProvenance{
			Provider:   photoData.Provider,
			ProviderID: photoData.ProviderID,
			Author:     photoData.Author,
			AuthorURL:  photoData.AuthorURL,
			License:    photoData.License,
			Query:      queryLabel,
			SourceURL:  photoData.SourceURL,
		},
	})
	if err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return ImageApplyResult{}, fmt.Errorf("import image candidate: %w", err)
	}

	if err := tracker.emit(ctx, "validate.repair"); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return ImageApplyResult{}, err
	}

	nextDraft := cloneDraftShallow(currentDraft)
	newImage := map[string]any{
		"assetId": asset.ID,
		"alt":     altText,
	}
	if err := setImageAtPath(nextDraft.Pages[pageIndex].Blocks[blockIndex].Props, path, newImage); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return ImageApplyResult{}, err
	}
	if err := siteconfig.ValidateDraft(nextDraft); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return ImageApplyResult{}, err
	}

	if err := tracker.emit(ctx, "persist"); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return ImageApplyResult{}, err
	}
	if err := s.writer.SaveDraft(ctx, workspaceID, nextDraft); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return ImageApplyResult{}, err
	}

	savedDraft, err := s.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return ImageApplyResult{}, err
	}
	resultRevisionID, err := s.captureDraftRevision(ctx, workspaceID, siteID, draftRevisionRecord{
		Scope:                 "block",
		PageID:                page.ID,
		Prompt:                historyPrompt,
		Draft:                 savedDraft,
		GenerationPrompt:      metadata.Prompt,
		GenerationSummaryJSON: metadata.SummaryJSON,
		CreatedBy:             userID,
	})
	if err != nil {
		return ImageApplyResult{}, err
	}

	summary := describeImageApplySummary(input)
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
		return ImageApplyResult{}, err
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
				Props:   nextDraft.Pages[pageIndex].Blocks[blockIndex].Props,
			}},
		}},
		AssetsNeeded: []string{"supporting-image"},
	}); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return ImageApplyResult{}, err
	}
	if err := s.incrementTrialPromptUsage(ctx, workspaceID); err != nil {
		_ = s.failGenerationJob(ctx, jobID, err)
		return ImageApplyResult{}, err
	}

	s.recordAudit(ctx, audit.Event{
		WorkspaceID: workspaceID,
		SiteID:      siteID,
		UserID:      userID,
		Action:      "block.image_suggest",
		Metadata: map[string]any{
			"jobId":   jobID,
			"blockId": blockID,
			"pageId":  page.ID,
			"assetId": asset.ID,
			"query":   queryLabel,
		},
	})

	return ImageApplyResult{
		JobID: jobID,
		Draft: savedDraft,
		Asset: &asset,
		Image: newImage,
	}, nil
}

// normalizeImagePath trims each segment and drops empties. The resulting
// path is the canonical form used by the resolver — every segment is a
// non-empty string. Array indices are still encoded as their decimal string.
func normalizeImagePath(path []string) []string {
	output := make([]string, 0, len(path))
	for _, segment := range path {
		clean := strings.TrimSpace(segment)
		if clean == "" {
			continue
		}
		output = append(output, clean)
	}
	return output
}

// resolveImageSlot walks props using path and returns the map that holds the
// leaf together with the leaf key. Because Go maps are references, callers
// can mutate the returned parent to update the underlying block props.
func resolveImageSlot(props map[string]any, path []string) (map[string]any, string, error) {
	if len(path) == 0 {
		return nil, "", errors.New("image path is empty")
	}
	var current any = props
	for i := 0; i < len(path)-1; i++ {
		switch typed := current.(type) {
		case map[string]any:
			current = typed[path[i]]
		case []any:
			idx, err := strconv.Atoi(path[i])
			if err != nil {
				return nil, "", fmt.Errorf("invalid array index %q", path[i])
			}
			if idx < 0 || idx >= len(typed) {
				return nil, "", fmt.Errorf("array index out of range: %d", idx)
			}
			current = typed[idx]
		case nil:
			return nil, "", fmt.Errorf("path segment %q points to a nil value", path[i])
		default:
			return nil, "", fmt.Errorf("cannot descend into %T at %q", current, path[i])
		}
	}
	parent, ok := current.(map[string]any)
	if !ok {
		return nil, "", fmt.Errorf("image slot parent must be a map, got %T", current)
	}
	return parent, path[len(path)-1], nil
}

func setImageAtPath(props map[string]any, path []string, image map[string]any) error {
	parent, leaf, err := resolveImageSlot(props, path)
	if err != nil {
		return err
	}
	parent[leaf] = image
	return nil
}

func currentImageAlt(props map[string]any, path []string) string {
	parent, leaf, err := resolveImageSlot(props, path)
	if err != nil {
		return ""
	}
	current, ok := parent[leaf].(map[string]any)
	if !ok {
		return ""
	}
	if alt, ok := current["alt"].(string); ok {
		return alt
	}
	return ""
}

func candidateFromPhoto(photo imagery.Photo) ImageSuggestCandidate {
	return ImageSuggestCandidate{
		Provider:    photo.Provider,
		ProviderID:  photo.ProviderID,
		DownloadURL: photo.DownloadURL,
		SourceURL:   photo.SourceURL,
		Width:       photo.Width,
		Height:      photo.Height,
		ContentType: photo.ContentType,
		Author:      photo.Author,
		AuthorURL:   photo.AuthorURL,
		License:     photo.License,
		Description: photo.Description,
	}
}

func orientationForBlock(blockType string) imagery.Orientation {
	switch blockType {
	case "hero", "image_text":
		return imagery.OrientationLandscape
	case "gallery":
		return imagery.OrientationAny
	default:
		return imagery.OrientationLandscape
	}
}

func extractBlockBodyText(props map[string]any) string {
	if props == nil {
		return ""
	}
	for _, key := range []string{"subheadline", "body", "description", "summary"} {
		if value, ok := props[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func describeImageApply(input ImageApplyInput) string {
	if instruction := strings.TrimSpace(input.Instruction); instruction != "" {
		return "Find a better image: " + instruction
	}
	if query := strings.TrimSpace(input.Query); query != "" {
		return "Replace image with " + query + " photo"
	}
	return "Replace image with AI-suggested photo"
}

func describeImageApplySummary(input ImageApplyInput) string {
	author := strings.TrimSpace(input.Photo.Author)
	switch {
	case author != "":
		return fmt.Sprintf("Replaced the image with a photo by %s.", author)
	case strings.TrimSpace(input.Query) != "":
		return fmt.Sprintf("Replaced the image with a fresh %s photo.", strings.TrimSpace(input.Query))
	default:
		return "Replaced the image with an AI-suggested photo."
	}
}

func fallbackImageQuery(request ImageQueryRequest) string {
	candidates := []string{
		request.UserInstruction,
		request.BlockHeadline,
		request.PageTitle,
		request.SiteGoal,
		request.SiteName,
	}
	for _, candidate := range candidates {
		clean := strings.TrimSpace(candidate)
		if clean == "" {
			continue
		}
		if len(clean) > 80 {
			clean = strings.TrimSpace(clean[:80])
		}
		return clean
	}
	return ""
}
