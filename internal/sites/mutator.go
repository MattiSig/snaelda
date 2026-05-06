package sites

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MattiSig/snaelda/internal/platform/ids"
	"github.com/MattiSig/snaelda/internal/platform/slugs"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrSiteNameRequired = errors.New("site name is required")
	ErrSiteSlugInvalid  = errors.New("site slug is invalid")
	ErrSiteSlugConflict = errors.New("site slug is already in use")
	ErrNoSiteChanges    = errors.New("site update requires at least one change")
)

type mutationDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Mutator interface {
	CreateSite(ctx context.Context, workspaceID string, input CreateSiteInput) (siteconfig.SiteDraft, error)
	UpdateSite(ctx context.Context, workspaceID string, siteID string, input UpdateSiteInput) (siteconfig.SiteDraft, error)
	DeleteSite(ctx context.Context, workspaceID string, siteID string) error
}

type CreateSiteInput struct {
	Name   string
	Slug   string
	Prompt string
}

type UpdateSiteInput struct {
	Name *string
	Slug *string
}

type PostgresMutator struct {
	db     mutationDB
	reader Reader
	writer Writer
}

func NewPostgresMutator(db DB) *PostgresMutator {
	return &PostgresMutator{
		db:     db,
		reader: NewPostgresReader(db),
		writer: NewPostgresWriter(db),
	}
}

func (m *PostgresMutator) CreateSite(ctx context.Context, workspaceID string, input CreateSiteInput) (siteconfig.SiteDraft, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return siteconfig.SiteDraft{}, ErrSiteNameRequired
	}

	slugValue, err := m.createSlug(ctx, workspaceID, input.Slug, name)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}

	draft, err := starterDraft(name, slugValue, input.Prompt)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}
	if err := m.savePrompt(ctx, workspaceID, draft.Site.ID, input.Prompt); err != nil {
		return siteconfig.SiteDraft{}, err
	}

	savedDraft, err := m.reader.LoadDraft(ctx, draft.Site.ID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	return savedDraft, nil
}

func (m *PostgresMutator) UpdateSite(ctx context.Context, workspaceID string, siteID string, input UpdateSiteInput) (siteconfig.SiteDraft, error) {
	if input.Name == nil && input.Slug == nil {
		return siteconfig.SiteDraft{}, ErrNoSiteChanges
	}

	draft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return siteconfig.SiteDraft{}, ErrSiteNameRequired
		}
		draft.Site.Name = name
	}

	if input.Slug != nil {
		slugValue := strings.TrimSpace(*input.Slug)
		if !slugs.IsValid(slugValue) {
			return siteconfig.SiteDraft{}, ErrSiteSlugInvalid
		}
		taken, err := m.siteSlugExists(ctx, workspaceID, slugValue, siteID)
		if err != nil {
			return siteconfig.SiteDraft{}, err
		}
		if taken {
			return siteconfig.SiteDraft{}, ErrSiteSlugConflict
		}
		draft.Site.Slug = slugValue
	}

	if err := m.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return siteconfig.SiteDraft{}, err
	}

	savedDraft, err := m.reader.LoadDraft(ctx, siteID)
	if err != nil {
		return siteconfig.SiteDraft{}, err
	}
	return savedDraft, nil
}

func (m *PostgresMutator) DeleteSite(ctx context.Context, workspaceID string, siteID string) error {
	tag, err := m.db.Exec(ctx, `
		delete from sites
		where id = $1
		  and workspace_id = $2
	`, siteID, workspaceID)
	if err != nil {
		return fmt.Errorf("delete site: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *PostgresMutator) createSlug(ctx context.Context, workspaceID string, requested string, name string) (string, error) {
	if value := strings.TrimSpace(requested); value != "" {
		if !slugs.IsValid(value) {
			return "", ErrSiteSlugInvalid
		}
		taken, err := m.siteSlugExists(ctx, workspaceID, value, "")
		if err != nil {
			return "", err
		}
		if taken {
			return "", ErrSiteSlugConflict
		}
		return value, nil
	}

	value, err := slugs.EnsureUnique(name, func(candidate string) (bool, error) {
		return m.siteSlugExists(ctx, workspaceID, candidate, "")
	})
	if err != nil {
		return "", fmt.Errorf("generate site slug: %w", err)
	}
	return value, nil
}

func (m *PostgresMutator) siteSlugExists(ctx context.Context, workspaceID string, slugValue string, excludeSiteID string) (bool, error) {
	var exists bool
	err := m.db.QueryRow(ctx, `
		select exists(
			select 1
			from sites
			where workspace_id = $1
			  and slug = $2
			  and ($3 = '' or id::text <> $3)
		)
	`, workspaceID, slugValue, excludeSiteID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check site slug: %w", err)
	}
	return exists, nil
}

func (m *PostgresMutator) savePrompt(ctx context.Context, workspaceID string, siteID string, prompt string) error {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil
	}

	tag, err := m.db.Exec(ctx, `
		update sites
		set generation_prompt = $1,
		    updated_at = now()
		where id = $2
		  and workspace_id = $3
	`, prompt, siteID, workspaceID)
	if err != nil {
		return fmt.Errorf("save site prompt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func starterDraft(name string, slugValue string, prompt string) (siteconfig.SiteDraft, error) {
	siteID, err := ids.New()
	if err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("generate site id: %w", err)
	}
	pageID, err := ids.New()
	if err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("generate page id: %w", err)
	}
	heroID, err := ids.New()
	if err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("generate hero block id: %w", err)
	}
	textID, err := ids.New()
	if err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("generate text block id: %w", err)
	}
	featuresID, err := ids.New()
	if err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("generate features block id: %w", err)
	}
	ctaID, err := ids.New()
	if err != nil {
		return siteconfig.SiteDraft{}, fmt.Errorf("generate cta block id: %w", err)
	}

	subheadline := strings.TrimSpace(prompt)
	if subheadline == "" {
		subheadline = "Start from a structured draft with real pages, editable sections, and a preview route that stays on the same site contract."
	}
	if len(subheadline) > 280 {
		subheadline = subheadline[:277] + "..."
	}

	return siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:            siteID,
			Name:          name,
			Slug:          slugValue,
			Status:        "draft",
			DefaultLocale: "en",
			SEO: siteconfig.SEOConfig{
				Title:       name,
				Description: subheadline,
			},
		},
		Theme: siteconfig.ThemeConfig{
			Version: siteconfig.ThemeVersionV1,
			Tokens: siteconfig.ThemeTokens{
				Colors: map[string]string{
					"background":   "#151215",
					"foreground":   "#f6f2ec",
					"surface":      "#231c24",
					"surfaceMuted": "#312736",
					"primary":      "#8fc6ff",
					"secondary":    "#8ee2d1",
					"accent":       "#ff8cad",
					"muted":        "#b78656",
					"border":       "#58415b",
					"ring":         "#f3b547",
				},
				Typography: map[string]any{
					"heading":     "Iowan Old Style",
					"body":        "Avenir Next",
					"headingFont": "Iowan Old Style",
					"bodyFont":    "Avenir Next",
				},
				Layout: map[string]any{
					"maxWidth":       "1120px",
					"contentWidth":   "720px",
					"sectionSpacing": "96px",
				},
				Shape: map[string]any{
					"radius": "28px",
					"shadow": "soft",
				},
			},
		},
		Navigation: siteconfig.NavigationConfig{
			Primary: []siteconfig.NavigationItem{
				{Label: "Home", PageID: pageID},
			},
		},
		Pages: []siteconfig.PageDraft{
			{
				ID:    pageID,
				Title: "Home",
				Slug:  "/",
				SEO: siteconfig.SEOConfig{
					Title:       name,
					Description: subheadline,
				},
				Blocks: []siteconfig.BlockInstance{
					{
						ID:      heroID,
						Type:    "hero",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"eyebrow":     name,
							"headline":    "A welcoming first draft for " + name,
							"subheadline": subheadline,
							"primaryCta": map[string]any{
								"label": "Review the draft",
								"href":  "#next-step",
							},
							"layout": "split-left",
						},
					},
					{
						ID:      textID,
						Type:    "text_section",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"heading":   "What this starter gives you",
							"body":      "A single-page site scaffold with validated blocks, a saved draft, and room to tune the copy before generation and publishing are wired in.",
							"alignment": "left",
							"width":     "default",
						},
					},
					{
						ID:      featuresID,
						Type:    "features_grid",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"heading": "Ready for the next loop",
							"intro":   "The prototype keeps each section inside the maintained registry so preview and publish can stay consistent later.",
							"columns": 3,
							"items": []any{
								map[string]any{
									"title": "Structured draft",
									"body":  "Every page and block is validated application data.",
								},
								map[string]any{
									"title": "Builder-friendly",
									"body":  "Site metadata can be edited without breaking the stored draft.",
								},
								map[string]any{
									"title": "Preview-ready",
									"body":  "The React preview reads the same draft shape the API serves.",
								},
							},
						},
					},
					{
						ID:      ctaID,
						Type:    "cta_band",
						Version: siteconfig.BlockVersionV1,
						Props: map[string]any{
							"heading": "Next step",
							"body":    "Refine the name or slug now, then move into richer page and block editing.",
							"cta": map[string]any{
								"label": "Stay in the builder",
								"href":  "/app/sites/" + siteID,
							},
							"variant": "accent",
						},
						Settings: siteconfig.BlockSettings{
							AnchorID: "next-step",
						},
					},
				},
			},
		},
	}, nil
}
