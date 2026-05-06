package siteconfig

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"

	"github.com/MattiSig/snaelda/internal/platform/slugs"
)

const (
	SiteConfigVersionV1 = "site-config.v1"
	ThemeVersionV1      = "theme.v1"
	MaxPagesPerSite     = 10
)

var (
	stableIDPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
	localePattern       = regexp.MustCompile(`^[a-z]{2}(?:-[A-Z]{2})?$`)
	hexColorPattern     = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	anchorPattern       = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,79}$`)
	unsafeControlChars  = regexp.MustCompile(`[\x00-\x1f\x7f]`)
	supportedColorKeys  = set("background", "text", "foreground", "surface", "surfaceMuted", "primary", "secondary", "accent", "muted", "border", "ring")
	supportedTypeKeys   = set("headingFont", "bodyFont", "heading", "body", "scale")
	supportedLayoutKeys = set("maxWidth", "contentWidth", "sectionSpacing")
	supportedShapeKeys  = set("radius", "shadow")
)

type Validator struct {
	Blocks *BlockRegistry
}

func NewValidator(blocks *BlockRegistry) Validator {
	if blocks == nil {
		blocks = DefaultBlockRegistry()
	}
	return Validator{Blocks: blocks}
}

func ValidateDraft(draft SiteDraft) error {
	return NewValidator(nil).ValidateDraft(draft)
}

func ValidatePublishedSnapshot(snapshot PublishedSnapshot) error {
	return NewValidator(nil).ValidatePublishedSnapshot(snapshot)
}

func (v Validator) ValidateDraft(draft SiteDraft) error {
	var c collector
	v.validateDraftSite("site", draft.Site, &c)
	v.validateTheme("theme", draft.Theme, &c)
	pageIDs := v.validatePages("pages", draft.Pages, false, &c)
	v.validateNavigation("navigation", draft.Navigation, pageIDs, &c)
	return c.err()
}

func (v Validator) ValidatePublishedSnapshot(snapshot PublishedSnapshot) error {
	var c collector
	if snapshot.SchemaVersion != SiteConfigVersionV1 {
		c.add("schemaVersion", "invalid_value", "schemaVersion must be "+SiteConfigVersionV1)
	}
	validateStableID("site.id", snapshot.Site.ID, &c)
	validateRequiredText("site.name", snapshot.Site.Name, 1, 120, &c)
	v.validateLocale("site.defaultLocale", snapshot.Site.DefaultLocale, true, &c)
	validateSEO("site.seo", snapshot.Site.SEO, true, &c)
	v.validateTheme("theme", snapshot.Theme, &c)
	pageIDs := v.validatePages("pages", snapshot.Pages, true, &c)
	v.validateNavigation("navigation", snapshot.Navigation, pageIDs, &c)
	return c.err()
}

func (v Validator) validateDraftSite(path string, site DraftSite, c *collector) {
	validateStableID(child(path, "id"), site.ID, c)
	validateRequiredText(child(path, "name"), site.Name, 1, 120, c)
	if !slugs.IsValid(site.Slug) {
		c.add(child(path, "slug"), "invalid_slug", "site slug must use lowercase words separated by hyphens")
	}
	if site.Status != "" && site.Status != "draft" {
		c.add(child(path, "status"), "invalid_value", "draft site status must be draft")
	}
	v.validateLocale(child(path, "defaultLocale"), site.DefaultLocale, false, c)
	validateSEO(child(path, "seo"), site.SEO, false, c)
}

func (v Validator) validatePages(path string, pages []PageDraft, requireSEO bool, c *collector) map[string]bool {
	pageIDs := map[string]bool{}
	slugsSeen := map[string]bool{}
	hasHomepage := false

	if len(pages) == 0 {
		c.add(path, "required", "site must include at least one page")
	}
	if len(pages) > MaxPagesPerSite {
		c.add(path, "too_many_pages", fmt.Sprintf("site cannot include more than %d pages", MaxPagesPerSite))
	}

	for index, page := range pages {
		pagePath := fmt.Sprintf("%s[%d]", path, index)
		validateStableID(child(pagePath, "id"), page.ID, c)
		if page.ID != "" {
			if pageIDs[page.ID] {
				c.add(child(pagePath, "id"), "duplicate_id", "page id must be unique")
			}
			pageIDs[page.ID] = true
		}
		validateRequiredText(child(pagePath, "title"), page.Title, 1, 120, c)
		if !validPageSlug(page.Slug) {
			c.add(child(pagePath, "slug"), "invalid_slug", "page slug must be / or a slash-prefixed slug")
		}
		if page.Slug == "/" {
			hasHomepage = true
		}
		if page.Slug != "" {
			if slugsSeen[page.Slug] {
				c.add(child(pagePath, "slug"), "duplicate_slug", "page slug must be unique")
			}
			slugsSeen[page.Slug] = true
		}
		validateSEO(child(pagePath, "seo"), page.SEO, requireSEO, c)
		v.validateBlocks(child(pagePath, "blocks"), page.Blocks, c)
	}

	if len(pages) > 0 && !hasHomepage {
		c.add(path, "missing_homepage", "site must include a homepage with slug /")
	}

	return pageIDs
}

func (v Validator) validateBlocks(path string, blocks []BlockInstance, c *collector) {
	blockIDs := map[string]bool{}
	if v.Blocks == nil {
		v.Blocks = DefaultBlockRegistry()
	}
	for index, block := range blocks {
		blockPath := fmt.Sprintf("%s[%d]", path, index)
		validateStableID(child(blockPath, "id"), block.ID, c)
		if block.ID != "" {
			if blockIDs[block.ID] {
				c.add(child(blockPath, "id"), "duplicate_id", "block id must be unique within the page")
			}
			blockIDs[block.ID] = true
		}
		if block.Type == "" {
			c.add(child(blockPath, "type"), "required", "block type is required")
		}
		if block.Version == "" {
			c.add(child(blockPath, "version"), "required", "block version is required")
		}
		definition, err := v.Blocks.Lookup(block.Type, block.Version)
		if errors.Is(err, ErrBlockTypeUnknown) {
			c.add(child(blockPath, "type"), "unknown_block", "block type is not registered")
		} else if errors.Is(err, ErrBlockVersionUnknown) {
			c.add(child(blockPath, "version"), "unknown_block_version", "block version is not registered")
		} else if err != nil {
			c.add(blockPath, "invalid_block", err.Error())
		} else {
			props := block.Props
			if props == nil {
				props = map[string]any{}
			}
			definition.ValidateProps(child(blockPath, "props"), props, c)
		}
		if block.Settings.AnchorID != "" && !anchorPattern.MatchString(block.Settings.AnchorID) {
			c.add(child(blockPath, "settings.anchorId"), "invalid_anchor", "anchorId must start with a letter and contain only letters, numbers, underscores, or hyphens")
		}
	}
}

func (v Validator) validateTheme(path string, theme ThemeConfig, c *collector) {
	if theme.Version != ThemeVersionV1 {
		c.add(child(path, "version"), "invalid_value", "theme version must be "+ThemeVersionV1)
	}
	if len(theme.Tokens.Colors) == 0 {
		c.add(child(path, "tokens.colors"), "required", "theme colors are required")
	}
	if theme.Tokens.Colors["background"] == "" {
		c.add(child(path, "tokens.colors.background"), "required", "background color is required")
	}
	if theme.Tokens.Colors["text"] == "" && theme.Tokens.Colors["foreground"] == "" {
		c.add(child(path, "tokens.colors.text"), "required", "text or foreground color is required")
	}
	if theme.Tokens.Colors["primary"] == "" {
		c.add(child(path, "tokens.colors.primary"), "required", "primary color is required")
	}
	for key, value := range theme.Tokens.Colors {
		fieldPath := child(child(path, "tokens.colors"), key)
		if !supportedColorKeys[key] {
			c.add(fieldPath, "unknown_token", "color token is not supported")
			continue
		}
		if !hexColorPattern.MatchString(value) {
			c.add(fieldPath, "invalid_color", "color token must be a hex color")
		}
	}
	validateTokenMap(child(path, "tokens.typography"), theme.Tokens.Typography, supportedTypeKeys, c)
	validateTokenMap(child(path, "tokens.layout"), theme.Tokens.Layout, supportedLayoutKeys, c)
	validateTokenMap(child(path, "tokens.shape"), theme.Tokens.Shape, supportedShapeKeys, c)
}

func (v Validator) validateNavigation(path string, navigation NavigationConfig, pageIDs map[string]bool, c *collector) {
	for index, item := range navigation.Primary {
		itemPath := fmt.Sprintf("%s.primary[%d]", path, index)
		validateRequiredText(child(itemPath, "label"), item.Label, 1, 60, c)
		if item.PageID == "" && item.Href == "" {
			c.add(itemPath, "required", "navigation item must include pageId or href")
		}
		if item.PageID != "" {
			if item.Href != "" {
				c.add(itemPath, "invalid_value", "navigation item cannot include both pageId and href")
			}
			if !pageIDs[item.PageID] {
				c.add(child(itemPath, "pageId"), "unresolved_reference", "navigation pageId must reference a page in the site")
			}
		}
		if item.Href != "" {
			if err := ValidateURL(item.Href); err != nil {
				c.add(child(itemPath, "href"), "unsafe_url", err.Error())
			}
		}
	}
}

func (v Validator) validateLocale(path string, value string, required bool, c *collector) {
	if value == "" {
		if required {
			c.add(path, "required", "locale is required")
		}
		return
	}
	if !localePattern.MatchString(value) {
		c.add(path, "invalid_locale", "locale must look like en or en-US")
	}
}

func validateSEO(path string, seo SEOConfig, required bool, c *collector) {
	if required {
		validateRequiredText(child(path, "title"), seo.Title, 1, 70, c)
		validateRequiredText(child(path, "description"), seo.Description, 1, 180, c)
		return
	}
	if seo.Title != "" {
		validateRequiredText(child(path, "title"), seo.Title, 1, 70, c)
	}
	if seo.Description != "" {
		validateRequiredText(child(path, "description"), seo.Description, 1, 180, c)
	}
}

func ValidateURL(raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return errors.New("URL is required")
	}
	if len(value) > 2048 {
		return errors.New("URL is too long")
	}
	if unsafeControlChars.MatchString(value) {
		return errors.New("URL contains control characters")
	}
	if strings.HasPrefix(value, "#") {
		anchor := strings.TrimPrefix(value, "#")
		if anchor == "" || !anchorPattern.MatchString(anchor) {
			return errors.New("anchor URL is invalid")
		}
		return nil
	}
	if strings.HasPrefix(value, "/") {
		if strings.HasPrefix(value, "//") {
			return errors.New("protocol-relative URLs are not supported")
		}
		if _, err := url.ParseRequestURI(value); err != nil {
			return errors.New("internal URL path is invalid")
		}
		return nil
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return errors.New("URL is invalid")
	}
	switch parsed.Scheme {
	case "http", "https":
		if parsed.Host == "" {
			return errors.New("absolute URL must include a host")
		}
		return nil
	case "mailto":
		if parsed.Opaque == "" {
			return errors.New("mailto URL must include an address")
		}
		if _, err := mail.ParseAddress(parsed.Opaque); err != nil {
			return errors.New("mailto URL is invalid")
		}
		return nil
	case "tel":
		if parsed.Opaque == "" {
			return errors.New("tel URL must include a number")
		}
		for _, r := range parsed.Opaque {
			if !(r >= '0' && r <= '9' || strings.ContainsRune("+()- .", r)) {
				return errors.New("tel URL contains unsupported characters")
			}
		}
		return nil
	default:
		return errors.New("URL scheme is not supported")
	}
}

func validateStableID(path string, value string, c *collector) {
	if value == "" {
		c.add(path, "required", "id is required")
		return
	}
	if !stableIDPattern.MatchString(value) {
		c.add(path, "invalid_id", "id must be stable and contain only letters, numbers, underscores, or hyphens")
	}
}

func validateRequiredText(path string, value string, minLength int, maxLength int, c *collector) {
	text := strings.TrimSpace(value)
	if len(text) < minLength {
		c.add(path, "required", "value is required")
		return
	}
	if len(text) > maxLength {
		c.add(path, "invalid_length", "value is too long")
	}
}

func validateTokenMap(path string, values map[string]any, allowed map[string]bool, c *collector) {
	for key, value := range values {
		fieldPath := child(path, key)
		if !allowed[key] {
			c.add(fieldPath, "unknown_token", "token is not supported")
			continue
		}
		switch value.(type) {
		case string, int, int64, float64:
		default:
			c.add(fieldPath, "invalid_type", "token value must be a string or number")
		}
	}
}

func validPageSlug(value string) bool {
	if value == "/" {
		return true
	}
	if !strings.HasPrefix(value, "/") || strings.Contains(value, "//") {
		return false
	}
	return slugs.IsValid(strings.TrimPrefix(value, "/"))
}

func set(values ...string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}
