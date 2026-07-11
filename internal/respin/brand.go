package respin

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"

	// Register the decoders used for dimension checks and logo dominant-colour
	// derivation. webp/avif have no std-lib decoder, so those ingest fine but
	// contribute no decoded dimensions or dominant colour (a soft degradation).
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/MattiSig/snaelda/internal/assets"
	"github.com/MattiSig/snaelda/internal/siteconfig"
)

// Brand-pull resource caps (Spec 21 §Resource Caps).
const (
	// maxImportImageBytes bounds the total bytes ingested across a single import
	// (~40 MB) so a page full of large images cannot exhaust storage.
	maxImportImageBytes = 40 << 20
	// minHeroDimension is the smallest decoded edge an image may have to be
	// treated as a hero photo rather than an icon or spacer.
	minHeroDimension = 200
	// defaultMaxHeroes bounds how many hero photos a single import pulls.
	defaultMaxHeroes = 3
	// maxLogoAttempts bounds how many logo candidates are fetched before giving
	// up, so a page with many header images cannot fan out into many fetches.
	maxLogoAttempts = 6
	// maxHeroCandidates bounds how many hero candidates are considered.
	maxHeroCandidates = 12
)

// imageContentTypeExt maps the ingestable image content types to a canonical
// file extension. It mirrors assets.allowedImageContentTypes; an image whose
// type is not here is skipped before any ingest is attempted.
var imageContentTypeExt = map[string]string{
	"image/avif": ".avif",
	"image/gif":  ".gif",
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// AssetIngestor stores an externally-fetched image and returns a normal asset.
// It is the seam over assets.Service.ImportExternal (which satisfies it) so the
// brand stage can be exercised without object storage.
type AssetIngestor interface {
	ImportExternal(ctx context.Context, input assets.ImportExternalInput) (assets.Asset, error)
}

// BrandPuller performs Spec 21 pipeline step 9: it selects a logo, a primary
// colour, and hero photos from a fetched site's brand hints, ingests each image
// through the SSRF-guarded fetcher into the asset store, and returns a
// siteconfig.BrandConfig ready to drop into the canonical generation input.
type BrandPuller struct {
	fetcher   *Fetcher
	ingestor  AssetIngestor
	logger    *slog.Logger
	maxHeroes int
}

// BrandPullerOption customizes a BrandPuller.
type BrandPullerOption func(*BrandPuller)

// WithMaxHeroes overrides how many hero photos are pulled.
func WithMaxHeroes(n int) BrandPullerOption {
	return func(p *BrandPuller) {
		if n >= 0 {
			p.maxHeroes = n
		}
	}
}

// WithBrandLogger attaches a logger for best-effort ingest diagnostics.
func WithBrandLogger(logger *slog.Logger) BrandPullerOption {
	return func(p *BrandPuller) {
		if logger != nil {
			p.logger = logger
		}
	}
}

// NewBrandPuller builds a brand-asset puller over the SSRF-guarded fetcher and
// an asset ingestor. Both are required; a nil fetcher or ingestor makes
// PullBrand a no-op that returns an empty brand so the pipeline still degrades
// cleanly to Spec 07's derivation rules.
func NewBrandPuller(fetcher *Fetcher, ingestor AssetIngestor, opts ...BrandPullerOption) *BrandPuller {
	p := &BrandPuller{
		fetcher:   fetcher,
		ingestor:  ingestor,
		logger:    slog.Default(),
		maxHeroes: defaultMaxHeroes,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// PullOptions carries the ownership and labelling context an ingest needs. The
// demo creates a real trial workspace/site up front (Spec 21), so these IDs are
// always available by the time the brand stage runs.
type PullOptions struct {
	WorkspaceID  string
	SiteID       string
	UserID       string
	ImportID     string
	BusinessName string // seeds logo alt text when the source omits one
	SourceURL    string
	MaxHeroes    int    // overrides the puller default for this call; 0 uses the default
	LanguageAlt  string // "is" | "en": localizes the fallback logo alt text
}

// BrandResult is the outcome of a brand pull: the populated brand config, the
// hero photo asset ids the composer can seed image slots with, and the full set
// of ingested asset ids for the import's provenance record (pulled_asset_ids)
// and unclaimed-asset garbage collection.
type BrandResult struct {
	Brand          siteconfig.BrandConfig `json:"brand"`
	HeroAssetIDs   []string               `json:"heroAssetIds,omitempty"`
	PulledAssetIDs []string               `json:"pulledAssetIds,omitempty"`
	LogoSource     string                 `json:"logoSource,omitempty"`  // header-img | content-img | apple-touch-icon | icon | og:image
	ColorSource    string                 `json:"colorSource,omitempty"` // css-var | theme-color | logo-dominant
}

// aggregateHints merges the per-page brand hints into one deduplicated set. The
// root page dominates, but a logo declared only on an interior page is still a
// valid candidate, so images and colours are collected across all pages.
type aggregateHints struct {
	images      []ImageRef
	icons       []IconRef
	ogImages    []string
	themeColors []string
	cssColors   []ColorHint
}

// PullBrand runs the brand-asset stage over the fetched pages. It never returns
// an error for missing or unfetchable assets — a partial pull simply yields a
// thinner BrandConfig (Spec 21 graceful degradation). It returns an error only
// when the ownership context is unusable (no workspace/site to ingest into).
func (p *BrandPuller) PullBrand(ctx context.Context, pages []Page, opts PullOptions) (BrandResult, error) {
	var result BrandResult
	result.Brand.BusinessName = strings.TrimSpace(opts.BusinessName)

	if p == nil || p.fetcher == nil || p.ingestor == nil {
		return result, nil
	}
	if strings.TrimSpace(opts.WorkspaceID) == "" || strings.TrimSpace(opts.SiteID) == "" {
		return result, fmt.Errorf("respin: brand pull needs a workspace and site")
	}

	hints := aggregate(pages)
	var budget int64 = maxImportImageBytes

	// --- Logo ---
	logoAlt := logoAltText(opts)
	excludeURLs := map[string]bool{}
	if id, logoURL, img, source, ok := p.pullLogo(ctx, hints, opts, logoAlt, &budget); ok {
		result.Brand.Logo = &siteconfig.BrandLogo{AssetID: id, Alt: logoAlt}
		result.LogoSource = source
		result.PulledAssetIDs = append(result.PulledAssetIDs, id)
		excludeURLs[logoURL] = true

		// --- Primary colour, with the decoded logo as the last-resort source ---
		if color, csource := resolvePrimaryColor(hints, img); color != "" {
			result.Brand.PrimaryColor = color
			result.ColorSource = csource
		}
	} else if color, csource := resolvePrimaryColor(hints, nil); color != "" {
		result.Brand.PrimaryColor = color
		result.ColorSource = csource
	}

	// --- Hero photos ---
	maxHeroes := p.maxHeroes
	if opts.MaxHeroes > 0 {
		maxHeroes = opts.MaxHeroes
	}
	heroIDs := p.pullHeroes(ctx, hints, opts, maxHeroes, &budget, excludeURLs)
	result.HeroAssetIDs = heroIDs
	result.PulledAssetIDs = append(result.PulledAssetIDs, heroIDs...)

	return result, nil
}

// aggregate merges hints from every page, deduplicating images by URL (keeping
// the strongest structural region) and colours/icons by value.
func aggregate(pages []Page) aggregateHints {
	var agg aggregateHints
	imgByURL := map[string]int{} // url -> index into agg.images
	iconSeen := map[string]bool{}
	ogSeen := map[string]bool{}
	themeSeen := map[string]bool{}
	colorSeen := map[string]int{} // hex -> index into agg.cssColors

	for _, page := range pages {
		for _, img := range page.Meta.Images {
			if idx, ok := imgByURL[img.URL]; ok {
				if regionRank(img.Region) > regionRank(agg.images[idx].Region) {
					// Prefer the header/nav placement — a stronger logo signal.
					merged := agg.images[idx]
					merged.Region = img.Region
					if merged.Hint == "" {
						merged.Hint = img.Hint
					}
					agg.images[idx] = merged
				}
				continue
			}
			imgByURL[img.URL] = len(agg.images)
			agg.images = append(agg.images, img)
		}
		for _, icon := range page.Meta.Icons {
			if icon.Href == "" || iconSeen[icon.Href] {
				continue
			}
			iconSeen[icon.Href] = true
			agg.icons = append(agg.icons, icon)
		}
		if og := strings.TrimSpace(page.Meta.OGImage); og != "" && !ogSeen[og] {
			ogSeen[og] = true
			agg.ogImages = append(agg.ogImages, og)
		}
		if tc := strings.TrimSpace(page.Meta.ThemeColor); tc != "" && !themeSeen[tc] {
			themeSeen[tc] = true
			agg.themeColors = append(agg.themeColors, tc)
		}
		for _, c := range page.Meta.CSSColors {
			if idx, ok := colorSeen[c.Value]; ok {
				if c.Score > agg.cssColors[idx].Score {
					agg.cssColors[idx].Score = c.Score
				}
				continue
			}
			colorSeen[c.Value] = len(agg.cssColors)
			agg.cssColors = append(agg.cssColors, c)
		}
	}
	return agg
}

func regionRank(region string) int {
	switch region {
	case "header":
		return 3
	case "nav":
		return 2
	case "content":
		return 1
	default:
		return 0
	}
}

// logoCandidate is a scored logo source URL awaiting ingest.
type logoCandidate struct {
	url    string
	alt    string
	score  int
	source string
}

// logoKeywords are the class/id/alt/src tokens that mark an image as a logo.
var logoKeywords = []string{"logo", "brand", "wordmark", "masthead"}

// pullLogo scores logo candidates, fetches them best-first, and returns the
// first that ingests successfully along with its decoded image (for dominant
// colour) and the source category.
func (p *BrandPuller) pullLogo(ctx context.Context, hints aggregateHints, opts PullOptions, alt string, budget *int64) (string, string, image.Image, string, bool) {
	candidates := logoCandidates(hints)
	attempts := 0
	for _, c := range candidates {
		if attempts >= maxLogoAttempts {
			break
		}
		attempts++
		data, err := p.fetchImage(ctx, c.url, budget)
		if err != nil {
			p.debug("respin logo fetch skipped", "url", c.url, "error", err)
			continue
		}
		id, err := p.ingest(ctx, data, c.url, alt, opts)
		if err != nil {
			p.debug("respin logo ingest skipped", "url", c.url, "error", err)
			continue
		}
		return id, c.url, data.img, c.source, true
	}
	return "", "", nil, "", false
}

// logoCandidates builds the ranked logo source list from the aggregated hints:
// header/nav images (strongest when they name themselves a logo), content
// images that name themselves a logo, apple-touch-icons, other rel-icons
// (larger sizes preferred), and finally og:image as a last resort.
func logoCandidates(hints aggregateHints) []logoCandidate {
	var out []logoCandidate
	seen := map[string]bool{}
	add := func(c logoCandidate) {
		if c.url == "" || seen[c.url] {
			return
		}
		seen[c.url] = true
		out = append(out, c)
	}

	for _, img := range hints.images {
		named := containsAny(img.Hint, logoKeywords) || containsAny(strings.ToLower(img.Alt), logoKeywords) || urlSuggestsLogo(img.URL)
		switch img.Region {
		case "header", "nav":
			score := 60
			if named {
				score = 100
			}
			add(logoCandidate{url: img.URL, alt: img.Alt, score: score, source: "header-img"})
		case "content":
			if named {
				add(logoCandidate{url: img.URL, alt: img.Alt, score: 50, source: "content-img"})
			}
		}
	}
	for _, icon := range hints.icons {
		score := 30
		source := "icon"
		if strings.Contains(icon.Rel, "apple-touch") {
			score = 45
			source = "apple-touch-icon"
		}
		score += sizesBonus(icon.Sizes)
		add(logoCandidate{url: icon.Href, score: score, source: source})
	}
	for _, og := range hints.ogImages {
		add(logoCandidate{url: og, score: 20, source: "og:image"})
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	return out
}

// sizesBonus rewards a rel-icon that declares a larger raster size (a 180x180
// apple-touch-icon makes a far better logo than a 16x16 favicon).
func sizesBonus(sizes string) int {
	sizes = strings.ToLower(strings.TrimSpace(sizes))
	if sizes == "" || sizes == "any" {
		return 0
	}
	best := 0
	for _, tok := range strings.Fields(sizes) {
		if x := strings.IndexByte(tok, 'x'); x > 0 {
			if n := parseDimAttr(tok[:x]); n > best {
				best = n
			}
		}
	}
	switch {
	case best >= 180:
		return 20
	case best >= 96:
		return 12
	case best >= 48:
		return 5
	default:
		return 0
	}
}

// heroCandidate is a scored hero-photo source URL awaiting ingest.
type heroCandidate struct {
	url   string
	alt   string
	score int
}

var heroKeywords = []string{"hero", "banner", "cover", "masthead", "slider", "carousel", "gallery", "feature", "photo"}

// pullHeroes selects large content photos, fetches them best-first, verifies a
// minimum decoded dimension (so icons and spacers are rejected), and ingests up
// to maxHeroes of them, skipping any URL already pulled as the logo.
func (p *BrandPuller) pullHeroes(ctx context.Context, hints aggregateHints, opts PullOptions, maxHeroes int, budget *int64, excludeURLs map[string]bool) []string {
	if maxHeroes <= 0 {
		return nil
	}
	candidates := heroCandidates(hints)
	var ids []string
	considered := 0
	for _, c := range candidates {
		if len(ids) >= maxHeroes || considered >= maxHeroCandidates {
			break
		}
		if excludeURLs[c.url] {
			continue // already pulled as the logo
		}
		considered++
		data, err := p.fetchImage(ctx, c.url, budget)
		if err != nil {
			p.debug("respin hero fetch skipped", "url", c.url, "error", err)
			continue
		}
		// Reject anything too small to be a real photo when we can measure it.
		if data.img != nil && max(data.width, data.height) < minHeroDimension {
			continue
		}
		id, err := p.ingest(ctx, data, c.url, altOrBusiness(c.alt, opts), opts)
		if err != nil {
			p.debug("respin hero ingest skipped", "url", c.url, "error", err)
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// heroCandidates ranks content/og images as hero photos, dropping tiny declared
// sizes and logo-like images (those belong to the logo slot, not a photo grid).
func heroCandidates(hints aggregateHints) []heroCandidate {
	var out []heroCandidate
	seen := map[string]bool{}
	add := func(c heroCandidate) {
		if c.url == "" || seen[c.url] {
			return
		}
		seen[c.url] = true
		out = append(out, c)
	}

	for _, img := range hints.images {
		if img.Region == "header" || img.Region == "footer" || img.Region == "nav" {
			continue
		}
		if containsAny(img.Hint, logoKeywords) || urlSuggestsLogo(img.URL) {
			continue
		}
		// Skip images that explicitly declare a small size.
		if w, h := img.Width, img.Height; (w > 0 && w < minHeroDimension) || (h > 0 && h < minHeroDimension) {
			continue
		}
		score := 30
		if containsAny(img.Hint, heroKeywords) || containsAny(strings.ToLower(img.Alt), heroKeywords) {
			score += 40
		}
		if img.Width >= 600 || img.Height >= 400 {
			score += 15
		}
		add(heroCandidate{url: img.URL, alt: img.Alt, score: score})
	}
	// og:image is a strong hero candidate (it is the site's chosen social image).
	for _, og := range hints.ogImages {
		add(heroCandidate{url: og, score: 55})
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	return out
}

// resolvePrimaryColor picks the brand primary colour in priority order: a
// declared CSS custom property, then a theme-color meta, then the dominant
// colour of the decoded logo. It returns the colour and its source, or "" when
// none is available (Spec 07 then derives one).
func resolvePrimaryColor(hints aggregateHints, logo image.Image) (string, string) {
	if len(hints.cssColors) > 0 {
		return hints.cssColors[0].Value, "css-var"
	}
	for _, tc := range hints.themeColors {
		if hex, ok := parseCSSColor(tc); ok {
			return hex, "theme-color"
		}
	}
	if logo != nil {
		if hex, ok := dominantColor(logo); ok {
			return hex, "logo-dominant"
		}
	}
	return "", ""
}

// imageData is a fetched, size-checked image body plus its best-effort decode.
type imageData struct {
	body          []byte
	contentType   string
	ext           string
	img           image.Image // nil when the format has no std-lib decoder
	width, height int
}

// fetchImage fetches an image through the SSRF-guarded client, enforces the
// per-import byte budget and the image content-type allow-list, and decodes it
// best-effort for dimension and dominant-colour use. It never ingests.
func (p *BrandPuller) fetchImage(ctx context.Context, imgURL string, budget *int64) (imageData, error) {
	if budget != nil && *budget <= 0 {
		return imageData{}, fmt.Errorf("respin: import image budget exhausted")
	}
	res, err := p.fetcher.Fetch(ctx, imgURL, FetchOptions{
		MaxBytes: defaultMaxImgBytes,
		Accept:   "image/avif,image/webp,image/png,image/jpeg,image/gif,*/*;q=0.8",
	})
	if err != nil {
		return imageData{}, err
	}
	if res.StatusCode >= 400 {
		return imageData{}, &FetchStatusError{StatusCode: res.StatusCode, URL: res.FinalURL}
	}
	if len(res.Body) == 0 {
		return imageData{}, fmt.Errorf("respin: empty image body")
	}

	contentType := imageContentType(res.ContentType, res.Body)
	ext, ok := imageContentTypeExt[contentType]
	if !ok {
		return imageData{}, &ContentTypeError{ContentType: res.ContentType, URL: res.FinalURL}
	}
	if budget != nil {
		*budget -= int64(len(res.Body))
	}

	data := imageData{body: res.Body, contentType: contentType, ext: ext}
	if cfg, _, derr := image.DecodeConfig(bytes.NewReader(res.Body)); derr == nil {
		data.width, data.height = cfg.Width, cfg.Height
	}
	if img, _, derr := image.Decode(bytes.NewReader(res.Body)); derr == nil {
		data.img = img
		b := img.Bounds()
		data.width, data.height = b.Dx(), b.Dy()
	}
	return data, nil
}

// ingest stores a fetched image as an asset with re-spin provenance and returns
// the new asset id.
func (p *BrandPuller) ingest(ctx context.Context, data imageData, imgURL, alt string, opts PullOptions) (string, error) {
	asset, err := p.ingestor.ImportExternal(ctx, assets.ImportExternalInput{
		WorkspaceID: opts.WorkspaceID,
		SiteID:      opts.SiteID,
		UserID:      opts.UserID,
		FileName:    fileNameForImage(imgURL, data.ext),
		ContentType: data.contentType,
		Body:        data.body,
		AltText:     alt,
		Width:       data.width,
		Height:      data.height,
		Provenance: assets.AssetProvenance{
			Provider:   "respin",
			ProviderID: strings.TrimSpace(opts.ImportID),
			SourceURL:  imgURL,
			Query:      strings.TrimSpace(opts.SourceURL),
		},
	})
	if err != nil {
		return "", err
	}
	return asset.ID, nil
}

func (p *BrandPuller) debug(msg string, args ...any) {
	if p.logger != nil {
		p.logger.Debug(msg, args...)
	}
}

// imageContentType resolves the effective image content type, falling back to
// content sniffing when the server sends a generic or missing type.
func imageContentType(header string, body []byte) string {
	ct := normalizeContentType(header)
	if _, ok := imageContentTypeExt[ct]; ok {
		return ct
	}
	sniffed := normalizeContentType(http.DetectContentType(body))
	if _, ok := imageContentTypeExt[sniffed]; ok {
		return sniffed
	}
	return ct
}

func normalizeContentType(ct string) string {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return ct
}

// fileNameForImage derives a storage file name from the image URL, forcing the
// extension to match the resolved content type.
func fileNameForImage(imgURL, ext string) string {
	base := "image"
	if u, err := url.Parse(imgURL); err == nil {
		if name := strings.TrimSpace(path.Base(u.Path)); name != "" && name != "/" && name != "." {
			if dot := strings.LastIndexByte(name, '.'); dot > 0 {
				name = name[:dot]
			}
			if name != "" {
				base = name
			}
		}
	}
	return base + ext
}

func logoAltText(opts PullOptions) string {
	if name := strings.TrimSpace(opts.BusinessName); name != "" {
		return name
	}
	if normalizeStageLocale(opts.LanguageAlt) == "is" {
		return "Merki"
	}
	return "Logo"
}

func altOrBusiness(alt string, opts PullOptions) string {
	if a := strings.TrimSpace(alt); a != "" {
		return a
	}
	return strings.TrimSpace(opts.BusinessName)
}

func containsAny(haystack string, needles []string) bool {
	if haystack == "" {
		return false
	}
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

// urlSuggestsLogo reports whether an image URL's path basename looks like a logo
// asset (e.g. /assets/logo.svg, /img/brand-mark.png).
func urlSuggestsLogo(imgURL string) bool {
	u, err := url.Parse(imgURL)
	if err != nil {
		return false
	}
	return containsAny(strings.ToLower(u.Path), logoKeywords)
}
