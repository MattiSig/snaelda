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

func hasIssue(t *testing.T, err error, code string) bool {
	t.Helper()
	validationErr, ok := err.(ValidationError)
	if !ok {
		return false
	}
	return validationErr.Has(code)
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
