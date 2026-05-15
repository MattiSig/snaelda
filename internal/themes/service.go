package themes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound                = errors.New("theme site not found")
	ErrNoThemeChanges          = errors.New("theme update requires at least one change")
	ErrThemePaletteInvalid     = errors.New("theme palette is invalid")
	ErrThemeFontPresetInvalid  = errors.New("theme font preset is invalid")
	ErrThemeSpacingInvalid     = errors.New("theme section spacing is invalid")
	ErrThemeRadiusInvalid      = errors.New("theme radius is invalid")
	ErrThemeButtonStyleInvalid = errors.New("theme button style is invalid")
	ErrThemeImageStyleInvalid  = errors.New("theme image style is invalid")
	ErrThemeRegenerationOff    = errors.New("theme regeneration is not configured")
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type draftReader interface {
	LoadDraft(ctx context.Context, siteID string) (siteconfig.SiteDraft, error)
}

type draftWriter interface {
	SaveDraft(ctx context.Context, workspaceID string, draft siteconfig.SiteDraft) error
}

type generationMetadataReader interface {
	LoadGenerationMetadata(ctx context.Context, siteID string) (sites.GenerationMetadata, error)
}

type themeRegenerator interface {
	RegenerateThemeSelection(ctx context.Context, prompt string, draft siteconfig.SiteDraft) (siteconfig.ThemeSelection, error)
}

type Service struct {
	reader     draftReader
	writer     draftWriter
	metadata   generationMetadataReader
	regenerate themeRegenerator
}

type ThemeState struct {
	Theme     siteconfig.ThemeConfig        `json:"theme"`
	Selection siteconfig.ThemeSelection     `json:"selection"`
	Options   siteconfig.ThemeEditorCatalog `json:"options"`
}

type UpdateInput struct {
	Palette        *string
	FontPreset     *string
	SectionSpacing *string
	Radius         *string
	ButtonStyle    *string
	ImageStyle     *string
}

type ServiceConfig struct {
	Regenerator themeRegenerator
}

func NewService(db DB, cfg ServiceConfig) *Service {
	return &Service{
		reader:     sites.NewPostgresReader(db),
		writer:     sites.NewPostgresWriter(db),
		metadata:   sites.NewPostgresReader(db),
		regenerate: cfg.Regenerator,
	}
}

func (s *Service) Load(ctx context.Context, siteID string) (ThemeState, error) {
	draft, err := s.reader.LoadDraft(ctx, siteID)
	if errors.Is(err, sites.ErrNotFound) {
		return ThemeState{}, ErrNotFound
	}
	if err != nil {
		return ThemeState{}, fmt.Errorf("load draft theme: %w", err)
	}
	return themeStateFromDraft(draft), nil
}

func (s *Service) Update(ctx context.Context, workspaceID string, siteID string, input UpdateInput) (ThemeState, error) {
	if input.Palette == nil &&
		input.FontPreset == nil &&
		input.SectionSpacing == nil &&
		input.Radius == nil &&
		input.ButtonStyle == nil &&
		input.ImageStyle == nil {
		return ThemeState{}, ErrNoThemeChanges
	}

	draft, err := s.reader.LoadDraft(ctx, siteID)
	if errors.Is(err, sites.ErrNotFound) {
		return ThemeState{}, ErrNotFound
	}
	if err != nil {
		return ThemeState{}, fmt.Errorf("load draft theme: %w", err)
	}

	selection := siteconfig.DetectThemeSelection(draft.Theme)
	if input.Palette != nil {
		selection.Palette = strings.TrimSpace(*input.Palette)
	}
	if input.FontPreset != nil {
		selection.FontPreset = strings.TrimSpace(*input.FontPreset)
	}
	if input.SectionSpacing != nil {
		selection.SectionSpacing = strings.TrimSpace(*input.SectionSpacing)
	}
	if input.Radius != nil {
		selection.Radius = strings.TrimSpace(*input.Radius)
	}
	if input.ButtonStyle != nil {
		selection.ButtonStyle = strings.TrimSpace(*input.ButtonStyle)
	}
	if input.ImageStyle != nil {
		selection.ImageStyle = strings.TrimSpace(*input.ImageStyle)
	}
	if err := validateSelection(selection); err != nil {
		return ThemeState{}, err
	}

	draft.Theme = siteconfig.BuildTheme(selection)
	if err := s.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return ThemeState{}, err
	}

	return themeStateFromDraft(draft), nil
}

func (s *Service) Regenerate(ctx context.Context, workspaceID string, siteID string) (ThemeState, error) {
	if s.regenerate == nil {
		return ThemeState{}, ErrThemeRegenerationOff
	}

	draft, err := s.reader.LoadDraft(ctx, siteID)
	if errors.Is(err, sites.ErrNotFound) {
		return ThemeState{}, ErrNotFound
	}
	if err != nil {
		return ThemeState{}, fmt.Errorf("load draft theme: %w", err)
	}

	metadata, err := s.metadata.LoadGenerationMetadata(ctx, siteID)
	if err != nil && !errors.Is(err, sites.ErrNotFound) {
		return ThemeState{}, fmt.Errorf("load generation metadata: %w", err)
	}

	prompt := strings.TrimSpace(metadata.Prompt)
	if prompt == "" {
		prompt = fmt.Sprintf("Create a distinct, production-safe theme for %s.", draft.Site.Name)
	}

	selection, err := s.regenerate.RegenerateThemeSelection(ctx, prompt, draft)
	if err != nil {
		return ThemeState{}, err
	}
	if err := validateSelection(selection); err != nil {
		return ThemeState{}, err
	}

	draft.Theme = siteconfig.BuildTheme(selection)
	if err := s.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return ThemeState{}, err
	}

	return themeStateFromDraft(draft), nil
}

func themeStateFromDraft(draft siteconfig.SiteDraft) ThemeState {
	return ThemeState{
		Theme:     draft.Theme,
		Selection: siteconfig.DetectThemeSelection(draft.Theme),
		Options:   siteconfig.DefaultThemeEditorCatalog(),
	}
}

func validateSelection(selection siteconfig.ThemeSelection) error {
	catalog := siteconfig.DefaultThemeEditorCatalog()
	if !hasThemeOption(catalog.Palettes, selection.Palette) {
		return ErrThemePaletteInvalid
	}
	if !hasThemeOption(catalog.FontPresets, selection.FontPreset) {
		return ErrThemeFontPresetInvalid
	}
	if !hasThemeOption(catalog.SectionSpacings, selection.SectionSpacing) {
		return ErrThemeSpacingInvalid
	}
	if !hasThemeOption(catalog.Radii, selection.Radius) {
		return ErrThemeRadiusInvalid
	}
	if !hasThemeOption(catalog.ButtonStyles, selection.ButtonStyle) {
		return ErrThemeButtonStyleInvalid
	}
	if !hasThemeOption(catalog.ImageStyles, selection.ImageStyle) {
		return ErrThemeImageStyleInvalid
	}
	return nil
}

func hasThemeOption(options []siteconfig.ThemeOption, id string) bool {
	for _, option := range options {
		if option.ID == id {
			return true
		}
	}
	return false
}
