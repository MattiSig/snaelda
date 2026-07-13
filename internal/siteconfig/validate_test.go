package siteconfig

import "testing"

func TestValidateDraftAcceptsCanonicalPrototypeDraft(t *testing.T) {
	if err := ValidateDraft(validDraft()); err != nil {
		t.Fatalf("validate draft: %v", err)
	}
}

func TestValidateDraftRejectsUnknownBlock(t *testing.T) {
	draft := validDraft()
	draft.Pages[0].Blocks[0].Type = "script_embed"

	err := ValidateDraft(draft)
	if !hasIssue(t, err, "unknown_block") {
		t.Fatalf("expected unknown block issue, got %v", err)
	}
}

func TestValidateDraftRejectsUnknownBlockVersion(t *testing.T) {
	draft := validDraft()
	draft.Pages[0].Blocks[0].Version = "9.9.9"

	err := ValidateDraft(draft)
	if !hasIssue(t, err, "unknown_block_version") {
		t.Fatalf("expected unknown block version issue, got %v", err)
	}
}

func TestValidateDraftRejectsUnsafeBlockURL(t *testing.T) {
	draft := validDraft()
	draft.Pages[0].Blocks[0].Props["primaryCta"] = map[string]any{
		"label": "Run it",
		"href":  "javascript:alert(1)",
	}

	err := ValidateDraft(draft)
	if !hasIssue(t, err, "unsafe_url") {
		t.Fatalf("expected unsafe URL issue, got %v", err)
	}
}

func TestValidateDraftRejectsUnsupportedBlockPropertyAndInvalidAnchor(t *testing.T) {
	draft := validDraft()
	draft.Pages[0].Blocks[0].Props["script"] = "alert(1)"
	draft.Pages[0].Blocks[0].Settings.AnchorID = "123-starts-wrong"

	err := ValidateDraft(draft)
	if !hasIssue(t, err, "unknown_property") {
		t.Fatalf("expected unknown property issue, got %v", err)
	}
	if !hasIssue(t, err, "invalid_anchor") {
		t.Fatalf("expected invalid anchor issue, got %v", err)
	}
}

func TestValidateDraftRejectsHTMLInPlainTextFields(t *testing.T) {
	draft := validDraft()
	draft.Site.Name = "<strong>Nordic Studio</strong>"
	draft.Pages[0].Blocks[0].Props["headline"] = "<em>Clear websites for focused teams</em>"

	err := ValidateDraft(draft)
	if !hasIssue(t, err, "html_not_allowed") {
		t.Fatalf("expected html_not_allowed issue, got %v", err)
	}
}

func TestValidateDraftRejectsInvalidPageSet(t *testing.T) {
	draft := validDraft()
	draft.Pages = []PageDraft{}
	for i := 0; i < MaxPagesPerSite+1; i++ {
		page := validDraft().Pages[0]
		page.ID = "page_" + itoa(i)
		page.Slug = "/page-" + itoa(i)
		draft.Pages = append(draft.Pages, page)
	}
	draft.Navigation.Primary = []NavigationItem{{Label: "Missing", PageID: "missing"}}

	err := ValidateDraft(draft)
	if !hasIssue(t, err, "too_many_pages") {
		t.Fatalf("expected page limit issue, got %v", err)
	}
	if !hasIssue(t, err, "missing_homepage") {
		t.Fatalf("expected missing homepage issue, got %v", err)
	}
	if !hasIssue(t, err, "unresolved_reference") {
		t.Fatalf("expected unresolved navigation issue, got %v", err)
	}
}

func TestValidateDraftRejectsInvalidThemeToken(t *testing.T) {
	draft := validDraft()
	draft.Theme.Tokens.Colors["background"] = "warm"
	draft.Theme.Tokens.Colors["danger"] = "#ff0000"

	err := ValidateDraft(draft)
	if !hasIssue(t, err, "invalid_color") {
		t.Fatalf("expected invalid color issue, got %v", err)
	}
	if !hasIssue(t, err, "unknown_token") {
		t.Fatalf("expected unknown token issue, got %v", err)
	}
}

func TestValidatePublishedSnapshotRequiresSchemaVersionAndSEO(t *testing.T) {
	snapshot := PublishedSnapshot{
		SchemaVersion: "site-config.v0",
		Site: PublishedSite{
			ID:            "site_demo",
			Name:          "Nordic Studio",
			DefaultLocale: "en",
		},
		Theme:      validDraft().Theme,
		Navigation: validDraft().Navigation,
		Pages:      validDraft().Pages,
	}

	err := ValidatePublishedSnapshot(snapshot)
	if !hasIssue(t, err, "invalid_value") {
		t.Fatalf("expected invalid schema version issue, got %v", err)
	}
	if !hasIssue(t, err, "required") {
		t.Fatalf("expected required SEO issue, got %v", err)
	}
}

func TestValidatePublishedSnapshotRejectsBrokenPageAndNavigationContracts(t *testing.T) {
	draft := validDraft()
	snapshot := PublishedSnapshot{
		SchemaVersion: SiteConfigVersionV1,
		Site: PublishedSite{
			ID:            draft.Site.ID,
			Name:          draft.Site.Name,
			DefaultLocale: draft.Site.DefaultLocale,
			SEO: SEOConfig{
				Title:       draft.Site.Name,
				Description: "Published fallback description.",
			},
		},
		Theme: draft.Theme,
		Navigation: NavigationConfig{
			Primary: []NavigationItem{{Label: "Missing", PageID: "page_missing"}},
		},
		Pages: []PageDraft{
			{
				ID:    "page_about",
				Title: "About",
				Slug:  "/about",
				SEO: SEOConfig{
					Title:       "About | Nordic Studio",
					Description: "About page.",
				},
				Blocks: draft.Pages[0].Blocks,
			},
			{
				ID:    "page_duplicate",
				Title: "Duplicate",
				Slug:  "/about",
				SEO: SEOConfig{
					Title:       "Duplicate | Nordic Studio",
					Description: "Duplicate slug page.",
				},
				Blocks: draft.Pages[0].Blocks,
			},
		},
	}

	err := ValidatePublishedSnapshot(snapshot)
	if !hasIssue(t, err, "duplicate_slug") {
		t.Fatalf("expected duplicate slug issue, got %v", err)
	}
	if !hasIssue(t, err, "missing_homepage") {
		t.Fatalf("expected missing homepage issue, got %v", err)
	}
	if !hasIssue(t, err, "unresolved_reference") {
		t.Fatalf("expected unresolved navigation issue, got %v", err)
	}
}

func TestValidatePublishedSnapshotRejectsHTMLInSEO(t *testing.T) {
	draft := validDraft()
	snapshot := PublishedSnapshot{
		SchemaVersion: SiteConfigVersionV1,
		Site: PublishedSite{
			ID:            draft.Site.ID,
			Name:          draft.Site.Name,
			DefaultLocale: draft.Site.DefaultLocale,
			SEO: SEOConfig{
				Title:       "<title>Nordic Studio</title>",
				Description: "Published fallback description.",
			},
		},
		Theme:      draft.Theme,
		Navigation: draft.Navigation,
		Pages: []PageDraft{
			{
				ID:    "page_home",
				Title: "Home",
				Slug:  "/",
				SEO: SEOConfig{
					Title:       "Nordic Studio",
					Description: "<p>Published fallback description.</p>",
				},
				Blocks: draft.Pages[0].Blocks,
			},
		},
	}

	err := ValidatePublishedSnapshot(snapshot)
	if !hasIssue(t, err, "html_not_allowed") {
		t.Fatalf("expected html_not_allowed issue, got %v", err)
	}
}

func TestValidatePublishedSnapshotRejectsCollectionDetailWithoutPublishedEntries(t *testing.T) {
	draft := validDraft()
	snapshot := PublishedSnapshot{
		SchemaVersion: SiteConfigVersionV1,
		Site: PublishedSite{
			ID:            draft.Site.ID,
			Name:          draft.Site.Name,
			DefaultLocale: draft.Site.DefaultLocale,
			SEO: SEOConfig{
				Title:       draft.Site.Name,
				Description: "Calm design systems for focused teams.",
			},
		},
		Theme:      draft.Theme,
		Navigation: NavigationConfig{Primary: []NavigationItem{{Label: "Home", PageID: "page_home"}}},
		Pages: []PageDraft{
			{
				ID:     "page_home",
				Title:  "Home",
				Slug:   "/",
				SEO:    SEOConfig{Title: "Home", Description: "Welcome to the studio."},
				Blocks: draft.Pages[0].Blocks,
			},
			{
				ID:           "page_service_detail",
				Title:        "Service detail",
				Slug:         "/services-template",
				Type:         PageTypeCollectionDetail,
				CollectionID: "col_services",
				SEO:          SEOConfig{Title: "Detail", Description: "Per-entry detail page template."},
				Blocks:       []BlockInstance{},
			},
		},
		Collections: []Collection{{
			ID:            "col_services",
			Slug:          "services",
			SingularLabel: "Service",
			PluralLabel:   "Services",
			Schema: []FieldDefinition{{
				Key:   "title",
				Label: "Title",
				Type:  FieldTypeText,
			}},
			// No entries -> publish must fail.
		}},
	}

	err := ValidatePublishedSnapshot(snapshot)
	if !hasIssue(t, err, "no_published_entries") {
		t.Fatalf("expected no_published_entries issue, got %v", err)
	}
}

func TestValidatePublishedSnapshotAcceptsCollectionDetailWithPublishedEntry(t *testing.T) {
	draft := validDraft()
	snapshot := PublishedSnapshot{
		SchemaVersion: SiteConfigVersionV1,
		Site: PublishedSite{
			ID:            draft.Site.ID,
			Name:          draft.Site.Name,
			DefaultLocale: draft.Site.DefaultLocale,
			SEO: SEOConfig{
				Title:       draft.Site.Name,
				Description: "Calm design systems for focused teams.",
			},
		},
		Brand: BrandConfig{
			BusinessName: draft.Site.Name,
			PrimaryColor: "#3c78ad",
		},
		Theme:      draft.Theme,
		Navigation: NavigationConfig{Primary: []NavigationItem{{Label: "Home", PageID: "page_home"}}},
		Pages: []PageDraft{
			{
				ID:     "page_home",
				Title:  "Home",
				Slug:   "/",
				SEO:    SEOConfig{Title: "Home", Description: "Welcome to the studio."},
				Blocks: draft.Pages[0].Blocks,
			},
			{
				ID:           "page_service_detail",
				Title:        "Service detail",
				Slug:         "/services-template",
				Type:         PageTypeCollectionDetail,
				CollectionID: "col_services",
				SEO:          SEOConfig{Title: "Detail", Description: "Per-entry detail page template."},
				Blocks:       []BlockInstance{},
			},
		},
		Collections: []Collection{{
			ID:            "col_services",
			Slug:          "services",
			SingularLabel: "Service",
			PluralLabel:   "Services",
			Schema: []FieldDefinition{{
				Key:   "title",
				Label: "Title",
				Type:  FieldTypeText,
			}},
			Entries: []CollectionEntry{
				{
					ID:     "entry_a",
					Slug:   "scaffolding",
					Status: EntryStatusPublished,
					Fields: map[string]any{"title": "Scaffolding"},
				},
			},
		}},
	}

	if err := ValidatePublishedSnapshot(snapshot); err != nil {
		t.Fatalf("expected snapshot with published entry to validate, got %v", err)
	}
}

func TestValidateFormDefinitionRejectsUnsupportedFields(t *testing.T) {
	err := ValidateFormDefinition(FormDefinition{
		Fields: []FormField{
			{Name: "email", Label: "Email", Type: "email", Required: true},
			{Name: "html", Label: "HTML", Type: "rich_html"},
			{Name: "topic", Label: "Topic", Type: "select"},
		},
		NotificationEmail: "not an email",
	})
	if !hasIssue(t, err, "unsupported_field") {
		t.Fatalf("expected unsupported field issue, got %v", err)
	}
	if !hasIssue(t, err, "required") {
		t.Fatalf("expected required select options issue, got %v", err)
	}
	if !hasIssue(t, err, "invalid_email") {
		t.Fatalf("expected invalid email issue, got %v", err)
	}
}

func TestValidateDraftAcceptsBrand(t *testing.T) {
	draft := validDraft()
	draft.Brand = BrandConfig{
		BusinessName: "Nordic Studio",
		PrimaryColor: "#315c4f",
		Logo: &BrandLogo{
			AssetID: "asset_logo_primary",
			Alt:     "Nordic Studio logo",
		},
	}
	if err := ValidateDraft(draft); err != nil {
		t.Fatalf("validate draft with brand: %v", err)
	}
}

func TestValidateDraftRejectsInvalidBrandPrimaryColor(t *testing.T) {
	draft := validDraft()
	draft.Brand = BrandConfig{
		BusinessName: "Nordic Studio",
		PrimaryColor: "rgba(0,0,0,1)",
	}
	err := ValidateDraft(draft)
	if !hasIssue(t, err, "invalid_color") {
		t.Fatalf("expected invalid_color issue, got %v", err)
	}
}

func TestValidateDraftRejectsBrandLogoWithoutAsset(t *testing.T) {
	draft := validDraft()
	draft.Brand = BrandConfig{
		Logo: &BrandLogo{Alt: "missing"},
	}
	err := ValidateDraft(draft)
	if !hasIssue(t, err, "required") {
		t.Fatalf("expected required issue for missing logo assetId, got %v", err)
	}
}

func TestValidateDraftBrandLogoSize(t *testing.T) {
	draft := validDraft()
	draft.Brand = BrandConfig{
		BusinessName: "Nordic Studio",
		PrimaryColor: "#e52095",
		Logo: &BrandLogo{
			AssetID:  "asset_logo_primary",
			Alt:      "Nordic Studio lockup",
			Size:     "large",
			HideName: true,
		},
	}
	if err := ValidateDraft(draft); err != nil {
		t.Fatalf("expected large/hideName logo to validate, got %v", err)
	}

	draft.Brand.Logo.Size = "huge"
	err := ValidateDraft(draft)
	if !hasIssue(t, err, "invalid_value") {
		t.Fatalf("expected invalid_value issue for unknown logo size, got %v", err)
	}
}

func TestValidatePublishedSnapshotRequiresBrand(t *testing.T) {
	draft := validDraft()
	snapshot := PublishedSnapshot{
		SchemaVersion: SiteConfigVersionV1,
		Site: PublishedSite{
			ID:            draft.Site.ID,
			Name:          draft.Site.Name,
			DefaultLocale: draft.Site.DefaultLocale,
			SEO: SEOConfig{
				Title:       draft.Site.Name,
				Description: "Calm design systems for focused teams.",
			},
		},
		Theme:      draft.Theme,
		Navigation: draft.Navigation,
		Pages: []PageDraft{
			{
				ID:    "page_home",
				Title: "Home",
				Slug:  "/",
				SEO: SEOConfig{
					Title:       "Home | Nordic Studio",
					Description: "Welcome to the studio.",
				},
				Blocks: draft.Pages[0].Blocks,
			},
		},
	}
	err := ValidatePublishedSnapshot(snapshot)
	if !hasIssue(t, err, "required") {
		t.Fatalf("expected required brand issue, got %v", err)
	}
}

func TestBuildThemeWithBrandOverridesPrimary(t *testing.T) {
	theme := BuildThemeWithBrand(DefaultThemeSelection(), BrandConfig{PrimaryColor: "#abcdef"})
	if theme.Tokens.Colors["primary"] != "#abcdef" {
		t.Fatalf("expected primary to be overridden, got %q", theme.Tokens.Colors["primary"])
	}
	if theme.Tokens.Colors["secondary"] == "" || theme.Tokens.Colors["accent"] == "" || theme.Tokens.Colors["surface"] == "" {
		t.Fatalf("expected derived palette tokens to be populated, got %#v", theme.Tokens.Colors)
	}
}

func TestBuildThemeWithBrandIgnoresInvalidBrandColor(t *testing.T) {
	preset := BuildTheme(DefaultThemeSelection())
	theme := BuildThemeWithBrand(DefaultThemeSelection(), BrandConfig{PrimaryColor: "not-a-color"})
	if theme.Tokens.Colors["primary"] != preset.Tokens.Colors["primary"] {
		t.Fatalf("expected invalid brand color to be ignored, got %q", theme.Tokens.Colors["primary"])
	}
}

func TestDetectThemeSelectionKeepsPaletteWithBrandDerivedColors(t *testing.T) {
	selection := DefaultThemeSelection()
	selection.Palette = ThemePaletteEditorialStudio
	selection.TypeScale = ThemeTypeScaleExpressive
	selection.ContentWidth = ThemeContentWidthFocused
	theme := BuildThemeWithBrand(selection, BrandConfig{PrimaryColor: "#abcdef"})

	detected := DetectThemeSelection(theme)
	if detected.Palette != ThemePaletteEditorialStudio {
		t.Fatalf("expected palette detection to survive brand color derivation, got %#v", detected)
	}
	if detected.TypeScale != ThemeTypeScaleExpressive || detected.ContentWidth != ThemeContentWidthFocused {
		t.Fatalf("expected extended theme selection to be detected, got %#v", detected)
	}
}

func TestThemeEditorCatalogWithBrandAddsPreviewColors(t *testing.T) {
	catalog := ThemeEditorCatalogWithBrand(BrandConfig{PrimaryColor: "#abcdef"})
	if len(catalog.Palettes) != 6 {
		t.Fatalf("expected six palette options, got %d", len(catalog.Palettes))
	}
	if len(catalog.FontPresets) != 6 {
		t.Fatalf("expected six font presets, got %d", len(catalog.FontPresets))
	}
	if len(catalog.Radii) != 5 {
		t.Fatalf("expected five radius options, got %d", len(catalog.Radii))
	}
	for _, option := range catalog.Palettes {
		if option.PreviewColors["primary"] != "#abcdef" {
			t.Fatalf("expected branded preview color for %s, got %#v", option.ID, option.PreviewColors)
		}
	}
}

func TestBuildThemeSupportsSharpCornersAndDistinctFontPresets(t *testing.T) {
	selection := DefaultThemeSelection()
	selection.FontPreset = ThemeFontHumanist
	selection.Radius = ThemeRadiusSharp

	theme := BuildTheme(selection)
	if theme.Tokens.Typography["headingFont"] != "Trebuchet MS" || theme.Tokens.Typography["bodyFont"] != "Trebuchet MS" {
		t.Fatalf("expected humanist font tokens, got %#v", theme.Tokens.Typography)
	}
	if theme.Tokens.Shape["radius"] != "0px" {
		t.Fatalf("expected true square corners, got %#v", theme.Tokens.Shape)
	}
	if detected := DetectThemeSelection(theme); detected.FontPreset != ThemeFontHumanist || detected.Radius != ThemeRadiusSharp {
		t.Fatalf("expected expanded selection to round trip, got %#v", detected)
	}
}

func TestValidateDraftRejectsImageWithoutAlt(t *testing.T) {
	draft := validDraft()
	draft.Pages[0].Blocks[0].Props["image"] = map[string]any{
		"assetId": "asset_hero",
	}

	err := ValidateDraft(draft)
	if !hasIssue(t, err, "required") {
		t.Fatalf("expected required alt issue, got %v", err)
	}
}

func TestValidateDraftRejectsInvalidPageStatus(t *testing.T) {
	draft := validDraft()
	draft.Pages[0].Status = "archived"

	err := ValidateDraft(draft)
	if !hasIssue(t, err, "invalid_value") {
		t.Fatalf("expected invalid page status issue, got %v", err)
	}
}

func hasIssue(t *testing.T, err error, code string) bool {
	t.Helper()
	validationErr, ok := err.(ValidationError)
	if !ok {
		return false
	}
	return validationErr.Has(code)
}

func TestValidateDraftRejectsInvalidCollectionDefaultSort(t *testing.T) {
	draft := validDraft()
	draft.Collections = []Collection{{
		ID:            "col_services",
		Slug:          "services",
		SingularLabel: "Service",
		PluralLabel:   "Services",
		Schema: []FieldDefinition{
			{Key: "title", Label: "Title", Type: FieldTypeText, Required: true},
		},
		Settings: CollectionSettings{DefaultSort: "alphabetic"},
	}}
	err := ValidateDraft(draft)
	if !hasIssue(t, err, "invalid_value") {
		t.Fatalf("expected invalid_value on defaultSort, got %v", err)
	}
}

func TestValidateDraftAcceptsRegisteredCollectionDefaultSort(t *testing.T) {
	for _, sort := range SupportedCollectionSorts() {
		t.Run(sort, func(t *testing.T) {
			draft := validDraft()
			draft.Collections = []Collection{{
				ID:            "col_services",
				Slug:          "services",
				SingularLabel: "Service",
				PluralLabel:   "Services",
				Schema: []FieldDefinition{
					{Key: "title", Label: "Title", Type: FieldTypeText, Required: true},
				},
				Settings: CollectionSettings{DefaultSort: sort},
			}}
			if err := ValidateDraft(draft); err != nil {
				t.Fatalf("expected sort %q to validate, got %v", sort, err)
			}
		})
	}
}

func TestValidateDraftRejectsUnknownSEOTemplatePlaceholder(t *testing.T) {
	draft := validDraft()
	draft.Collections = []Collection{{
		ID:            "col_services",
		Slug:          "services",
		SingularLabel: "Service",
		PluralLabel:   "Services",
		Schema: []FieldDefinition{
			{Key: "title", Label: "Title", Type: FieldTypeText, Required: true},
		},
		Settings: CollectionSettings{
			SEOTitleTemplate: "{{entry.unknown_field}} | {{site.name}}",
		},
	}}
	err := ValidateDraft(draft)
	if !hasIssue(t, err, "unresolved_reference") {
		t.Fatalf("expected unresolved_reference on SEO template, got %v", err)
	}
}

func TestValidateDraftAcceptsValidSEOTemplates(t *testing.T) {
	draft := validDraft()
	draft.Collections = []Collection{{
		ID:            "col_services",
		Slug:          "services",
		SingularLabel: "Service",
		PluralLabel:   "Services",
		Schema: []FieldDefinition{
			{Key: "title", Label: "Title", Type: FieldTypeText, Required: true},
			{Key: "summary", Label: "Summary", Type: FieldTypeLongText},
		},
		Settings: CollectionSettings{
			SEOTitleTemplate:       "{{entry.title}} | {{site.name}}",
			SEODescriptionTemplate: "{{entry.summary}}",
		},
	}}
	if err := ValidateDraft(draft); err != nil {
		t.Fatalf("expected valid SEO templates to pass, got %v", err)
	}
}

func TestValidateDraftRejectsPageSlugCollidingWithCollectionPrefix(t *testing.T) {
	draft := validDraft()
	draft.Pages = append(draft.Pages, PageDraft{
		ID:    "page_services_static",
		Title: "Services",
		Slug:  "/services",
		Blocks: []BlockInstance{
			{
				ID:      "block_text",
				Type:    "text_section",
				Version: BlockVersionV1,
				Props:   map[string]any{"heading": "Services", "body": "List", "alignment": "left", "width": "default"},
			},
		},
	})
	draft.Collections = []Collection{{
		ID:            "col_services",
		Slug:          "services",
		SingularLabel: "Service",
		PluralLabel:   "Services",
		Schema: []FieldDefinition{
			{Key: "title", Label: "Title", Type: FieldTypeText, Required: true},
		},
		Settings: CollectionSettings{ExposeDetailURLs: true},
	}}
	err := ValidateDraft(draft)
	if !hasIssue(t, err, "collection_prefix_conflict") {
		t.Fatalf("expected collection_prefix_conflict, got %v", err)
	}
}

func TestValidateDraftAcceptsCollectionIndexBoundToSameSlug(t *testing.T) {
	draft := validDraft()
	draft.Pages = append(draft.Pages, PageDraft{
		ID:           "page_services_index",
		Title:        "Services",
		Slug:         "/services",
		Type:         PageTypeCollectionIndex,
		CollectionID: "col_services",
		Blocks: []BlockInstance{
			{
				ID:      "block_text",
				Type:    "text_section",
				Version: BlockVersionV1,
				Props:   map[string]any{"heading": "Services", "body": "List", "alignment": "left", "width": "default"},
			},
		},
	})
	draft.Collections = []Collection{{
		ID:            "col_services",
		Slug:          "services",
		SingularLabel: "Service",
		PluralLabel:   "Services",
		Schema: []FieldDefinition{
			{Key: "title", Label: "Title", Type: FieldTypeText, Required: true},
		},
		Settings: CollectionSettings{ExposeDetailURLs: true},
	}}
	if err := ValidateDraft(draft); err != nil {
		t.Fatalf("expected collection_index bound to same slug to validate, got %v", err)
	}
}

func TestValidatePublishedSnapshotRejectsEntryURLCollidingWithPage(t *testing.T) {
	draft := validDraft()
	snapshot := PublishedSnapshot{
		SchemaVersion: SiteConfigVersionV1,
		Site: PublishedSite{
			ID:            draft.Site.ID,
			Name:          draft.Site.Name,
			DefaultLocale: draft.Site.DefaultLocale,
			SEO: SEOConfig{
				Title:       draft.Site.Name,
				Description: "Calm design systems for focused teams.",
			},
		},
		Brand: BrandConfig{
			BusinessName: draft.Site.Name,
			PrimaryColor: "#3c78ad",
		},
		Theme:      draft.Theme,
		Navigation: NavigationConfig{Primary: []NavigationItem{{Label: "Home", PageID: "page_home"}}},
		Pages: []PageDraft{
			{
				ID:     "page_home",
				Title:  "Home",
				Slug:   "/",
				SEO:    SEOConfig{Title: "Home", Description: "Welcome."},
				Blocks: draft.Pages[0].Blocks,
			},
			{
				ID:     "page_static_collision",
				Title:  "Scaffolding rentals",
				Slug:   "/services/scaffolding",
				Status: PageStatusPublished,
				SEO:    SEOConfig{Title: "Scaffolding", Description: "Static page."},
				Blocks: draft.Pages[0].Blocks,
			},
			{
				ID:           "page_service_detail",
				Title:        "Service detail",
				Slug:         "/services-template",
				Type:         PageTypeCollectionDetail,
				CollectionID: "col_services",
				SEO:          SEOConfig{Title: "Detail", Description: "Template."},
				Blocks:       []BlockInstance{},
			},
		},
		Collections: []Collection{{
			ID:            "col_services",
			Slug:          "services",
			SingularLabel: "Service",
			PluralLabel:   "Services",
			Schema: []FieldDefinition{
				{Key: "title", Label: "Title", Type: FieldTypeText, Required: true},
			},
			Entries: []CollectionEntry{
				{
					ID:     "entry_a",
					Slug:   "scaffolding",
					Status: EntryStatusPublished,
					Fields: map[string]any{"title": "Scaffolding"},
				},
			},
		}},
	}
	err := ValidatePublishedSnapshot(snapshot)
	if !hasIssue(t, err, "entry_url_conflict") {
		t.Fatalf("expected entry_url_conflict, got %v", err)
	}
}

func validDraft() SiteDraft {
	return SiteDraft{
		Site: DraftSite{
			ID:            "site_demo",
			Name:          "Nordic Studio",
			Slug:          "nordic-studio",
			Status:        "draft",
			DefaultLocale: "en",
		},
		Theme: ThemeConfig{
			Version: ThemeVersionV1,
			Tokens: ThemeTokens{
				Colors: map[string]string{
					"background": "#f8f7f4",
					"text":       "#1d2520",
					"primary":    "#315c4f",
					"accent":     "#c2774b",
				},
				Typography: map[string]any{
					"headingFont": "Inter",
					"bodyFont":    "Inter",
				},
				Layout: map[string]any{
					"maxWidth": "1120px",
				},
				Shape: map[string]any{
					"radius": "8px",
				},
			},
		},
		Navigation: NavigationConfig{
			Primary: []NavigationItem{
				{Label: "Home", PageID: "page_home"},
				{Label: "Contact", Href: "/contact"},
			},
		},
		Pages: []PageDraft{
			{
				ID:    "page_home",
				Title: "Home",
				Slug:  "/",
				SEO: SEOConfig{
					Title:       "Nordic Studio",
					Description: "Calm design systems for focused teams.",
				},
				Blocks: []BlockInstance{
					{
						ID:      "block_hero",
						Type:    "hero",
						Version: BlockVersionV1,
						Props: map[string]any{
							"eyebrow":     "Nordic Studio",
							"headline":    "Clear websites for focused teams",
							"subheadline": "Structured sites from maintained blocks.",
							"primaryCta": map[string]any{
								"label": "Start a project",
								"href":  "/contact",
							},
							"layout": "centered",
						},
					},
					{
						ID:      "block_text",
						Type:    "text_section",
						Version: BlockVersionV1,
						Props: map[string]any{
							"heading":   "A structured seed draft",
							"body":      "This content is stored as validated application data, not generated code.",
							"alignment": "left",
							"width":     "default",
						},
					},
				},
			},
		},
	}
}
