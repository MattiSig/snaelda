package generation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"path/filepath"
	"strings"

	"github.com/MattiSig/snaelda/internal/assets"
	"github.com/MattiSig/snaelda/internal/imagery"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// AssetImporter is the subset of the assets service the generation flow
// needs to ingest backend-fetched starter imagery.
type AssetImporter interface {
	ImportExternal(ctx context.Context, input assets.ImportExternalInput) (assets.Asset, error)
}

// StarterImagery wraps a provider with the per-run dedupe logic. It is
// nil-safe so generation can run without configured imagery.
type StarterImagery struct {
	provider *imagery.DedupedProvider
}

// NewStarterImagery returns a StarterImagery that delegates to the
// supplied provider. Passing a nil provider produces a no-op that the
// generation flow treats as "imagery not configured".
func NewStarterImagery(provider imagery.Provider) *StarterImagery {
	if provider == nil {
		return nil
	}
	return &StarterImagery{provider: imagery.NewDedupedProvider(provider)}
}

func (s *StarterImagery) available() bool {
	return s != nil && s.provider != nil
}

// applyStarterImagery walks the supplied draft and fills image slots that
// the generator left empty with assets sourced from the configured imagery
// provider. Failures are logged and the slots are simply left empty so
// generation never blocks on third-party availability.
func (s *Service) applyStarterImagery(ctx context.Context, workspaceID string, userID string, draft siteconfig.SiteDraft, prompt string) siteconfig.SiteDraft {
	if !s.imagery.available() || s.assetImporter == nil {
		return draft
	}
	if strings.TrimSpace(draft.Site.ID) == "" {
		return draft
	}

	profile := profilePrompt(prompt)
	for pageIndex := range draft.Pages {
		page := &draft.Pages[pageIndex]
		for blockIndex := range page.Blocks {
			block := &page.Blocks[blockIndex]
			s.fillBlockStarterImagery(ctx, workspaceID, userID, draft.Site, page, block, profile, prompt)
		}
	}

	return draft
}

func (s *Service) fillBlockStarterImagery(
	ctx context.Context,
	workspaceID string,
	userID string,
	site siteconfig.DraftSite,
	page *siteconfig.PageDraft,
	block *siteconfig.BlockInstance,
	profile promptProfile,
	prompt string,
) {
	if block == nil || block.Props == nil {
		return
	}

	switch block.Type {
	case "hero":
		if image, ok := imageNeedsAsset(block.Props, "image"); ok {
			queries := heroImageQueries(site, page, block, profile, prompt)
			alt := firstNonEmpty(readGeneratedText(block.Props, "headline", 180), readGeneratedText(block.Props, "subheadline", 180), site.Name)
			if filled := s.fetchAndStoreImage(ctx, workspaceID, userID, site.ID, queries, alt, imagery.OrientationLandscape, image); filled != nil {
				block.Props["image"] = filled
			}
		}
	case "image_text":
		if image, ok := imageNeedsAsset(block.Props, "image"); ok {
			queries := imageTextQueries(site, page, block, profile)
			alt := firstNonEmpty(readGeneratedText(block.Props, "heading", 180), readGeneratedText(block.Props, "body", 180), page.Title)
			if filled := s.fetchAndStoreImage(ctx, workspaceID, userID, site.ID, queries, alt, imagery.OrientationLandscape, image); filled != nil {
				block.Props["image"] = filled
			}
		}
	case "gallery":
		raw, ok := block.Props["images"].([]any)
		if !ok {
			return
		}
		for index := range raw {
			item, ok := raw[index].(map[string]any)
			if !ok {
				continue
			}
			image, needs := imageNeedsAsset(item, "image")
			if !needs {
				continue
			}
			queries := galleryImageQueries(site, page, item, profile)
			alt := firstNonEmpty(readGeneratedText(item, "caption", 180), readGeneratedText(item, "title", 180), page.Title)
			if filled := s.fetchAndStoreImage(ctx, workspaceID, userID, site.ID, queries, alt, imagery.OrientationLandscape, image); filled != nil {
				item["image"] = filled
				raw[index] = item
			}
		}
		block.Props["images"] = raw
	}
}

// imageNeedsAsset returns the current image map and whether the slot still
// needs an asset (no assetId set). If no image map exists yet a blank one
// is returned so the caller can fill it.
func imageNeedsAsset(props map[string]any, key string) (map[string]any, bool) {
	if props == nil {
		return map[string]any{}, true
	}
	switch typed := props[key].(type) {
	case nil:
		return map[string]any{}, true
	case map[string]any:
		if id, _ := typed["assetId"].(string); strings.TrimSpace(id) != "" {
			return typed, false
		}
		return typed, true
	default:
		return map[string]any{}, true
	}
}

func (s *Service) fetchAndStoreImage(
	ctx context.Context,
	workspaceID string,
	userID string,
	siteID string,
	queries []string,
	alt string,
	orientation imagery.Orientation,
	existing map[string]any,
) map[string]any {
	if len(queries) == 0 {
		return nil
	}

	photo, err := s.imagery.provider.PickFresh(ctx, queries, orientation)
	if err != nil {
		s.logImageryFailure("pick starter image", queries, err)
		return nil
	}

	data, err := s.imagery.provider.Download(ctx, photo)
	if err != nil {
		s.logImageryFailure("download starter image", queries, err)
		return nil
	}

	contentType := strings.ToLower(strings.TrimSpace(data.ContentType))
	if contentType == "" {
		contentType = "image/jpeg"
	}
	fileName := buildStarterFileName(photo, contentType)
	queryLabel := strings.Join(queries, " | ")

	asset, err := s.assetImporter.ImportExternal(ctx, assets.ImportExternalInput{
		WorkspaceID: workspaceID,
		SiteID:      siteID,
		UserID:      userID,
		FileName:    fileName,
		ContentType: contentType,
		Body:        data.Body,
		AltText:     clampSentence(alt, 180),
		Width:       photo.Width,
		Height:      photo.Height,
		Provenance: assets.AssetProvenance{
			Provider:   photo.Provider,
			ProviderID: photo.ProviderID,
			Author:     photo.Author,
			AuthorURL:  photo.AuthorURL,
			License:    photo.License,
			Query:      queryLabel,
			SourceURL:  photo.SourceURL,
		},
	})
	if err != nil {
		s.logImageryFailure("import starter image", queries, err)
		return nil
	}

	next := map[string]any{
		"assetId": asset.ID,
	}
	if altClean := clampSentence(alt, 180); altClean != "" {
		next["alt"] = altClean
	} else if existing != nil {
		if altRaw, ok := existing["alt"].(string); ok && altRaw != "" {
			next["alt"] = clampSentence(altRaw, 180)
		}
	}
	return next
}

func (s *Service) logImageryFailure(operation string, queries []string, err error) {
	if s == nil {
		return
	}
	if errors.Is(err, imagery.ErrNoResults) || errors.Is(err, imagery.ErrUnavailable) {
		if s.logger != nil {
			s.logger.Debug("starter imagery skipped", "operation", operation, "queries", queries, "reason", err.Error())
		}
		return
	}
	if s.logger != nil {
		s.logger.Warn("starter imagery failed", "operation", operation, "queries", queries, "error", err.Error())
	}
}

var starterImageExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
	"image/avif": ".avif",
}

func buildStarterFileName(photo imagery.Photo, contentType string) string {
	extension := starterImageExtensions[strings.ToLower(strings.TrimSpace(contentType))]
	if extension == "" {
		if extensions, err := mime.ExtensionsByType(contentType); err == nil && len(extensions) > 0 {
			extension = extensions[0]
		}
	}
	if extension == "" {
		extension = filepath.Ext(photo.DownloadURL)
	}
	if extension == "" {
		extension = ".jpg"
	}
	if !strings.HasPrefix(extension, ".") {
		extension = "." + extension
	}
	providerID := strings.TrimSpace(photo.ProviderID)
	if providerID == "" {
		providerID = "starter"
	}
	return fmt.Sprintf("%s-%s%s", photo.Provider, providerID, extension)
}

func heroImageQueries(site siteconfig.DraftSite, page *siteconfig.PageDraft, block *siteconfig.BlockInstance, profile promptProfile, prompt string) []string {
	queries := []string{}
	headline := readGeneratedText(block.Props, "headline", 80)
	eyebrow := readGeneratedText(block.Props, "eyebrow", 60)
	if subject := strings.TrimSpace(joinNonEmpty(" ", eyebrow, headline)); subject != "" {
		queries = appendQuery(queries, subject)
	}
	queries = appendQuery(queries, categoryHeroQuery(profile.Category))
	queries = appendQuery(queries, joinNonEmpty(" ", profile.CategoryLabel, "lifestyle"))
	queries = appendQuery(queries, prompt)
	queries = appendQuery(queries, site.Name)
	return dedupeQueries(queries)
}

func imageTextQueries(site siteconfig.DraftSite, page *siteconfig.PageDraft, block *siteconfig.BlockInstance, profile promptProfile) []string {
	queries := []string{}
	heading := readGeneratedText(block.Props, "heading", 80)
	body := readGeneratedText(block.Props, "body", 120)
	if subject := strings.TrimSpace(joinNonEmpty(" ", heading, page.Title)); subject != "" {
		queries = appendQuery(queries, subject)
	}
	queries = appendQuery(queries, joinNonEmpty(" ", profile.CategoryLabel, "behind the scenes"))
	queries = appendQuery(queries, body)
	queries = appendQuery(queries, categoryHeroQuery(profile.Category))
	queries = appendQuery(queries, site.Name)
	return dedupeQueries(queries)
}

func galleryImageQueries(site siteconfig.DraftSite, page *siteconfig.PageDraft, item map[string]any, profile promptProfile) []string {
	queries := []string{}
	title := readGeneratedText(item, "title", 80)
	caption := readGeneratedText(item, "caption", 120)
	if subject := strings.TrimSpace(joinNonEmpty(" ", title, caption)); subject != "" {
		queries = appendQuery(queries, subject)
	}
	queries = appendQuery(queries, joinNonEmpty(" ", page.Title, profile.CategoryLabel))
	queries = appendQuery(queries, categoryHeroQuery(profile.Category))
	queries = appendQuery(queries, site.Name+" gallery")
	return dedupeQueries(queries)
}

func categoryHeroQuery(category string) string {
	switch category {
	case "photography":
		return "natural photography studio"
	case "florist":
		return "florist studio flowers"
	case "wellness":
		return "calm wellness practice"
	case "creative":
		return "creative studio workspace"
	case "craft":
		return "handmade craft workshop"
	case "food":
		return "cozy cafe atmosphere"
	default:
		return "small business storefront"
	}
}

func appendQuery(queries []string, value string) []string {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return queries
	}
	if len(clean) > 80 {
		clean = strings.TrimSpace(clean[:80])
	}
	return append(queries, clean)
}

func dedupeQueries(queries []string) []string {
	seen := map[string]bool{}
	output := make([]string, 0, len(queries))
	for _, query := range queries {
		key := strings.ToLower(query)
		if seen[key] {
			continue
		}
		seen[key] = true
		output = append(output, query)
		if len(output) == 2 {
			break
		}
	}
	return output
}

func joinNonEmpty(separator string, parts ...string) string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		clean := strings.TrimSpace(part)
		if clean != "" {
			values = append(values, clean)
		}
	}
	return strings.Join(values, separator)
}

// Provider-agnostic logging guard so tests can run without configuring a
// global logger sink.
var _ = slog.Default
