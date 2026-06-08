package themes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/MattiSig/snaelda/internal/generation"
	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound                 = errors.New("theme site not found")
	ErrNoThemeChanges           = errors.New("theme update requires at least one change")
	ErrThemePaletteInvalid      = errors.New("theme palette is invalid")
	ErrThemeFontPresetInvalid   = errors.New("theme font preset is invalid")
	ErrThemeTypeScaleInvalid    = errors.New("theme type scale is invalid")
	ErrThemeSpacingInvalid      = errors.New("theme section spacing is invalid")
	ErrThemeContentWidthInvalid = errors.New("theme content width is invalid")
	ErrThemeRadiusInvalid       = errors.New("theme radius is invalid")
	ErrThemeButtonStyleInvalid  = errors.New("theme button style is invalid")
	ErrThemeImageStyleInvalid   = errors.New("theme image style is invalid")
	ErrThemeRegenerationOff     = errors.New("theme regeneration is not configured")
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
	db         DB
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
	TypeScale      *string
	SectionSpacing *string
	ContentWidth   *string
	Radius         *string
	ButtonStyle    *string
	ImageStyle     *string
}

type ServiceConfig struct {
	Regenerator themeRegenerator
}

func NewService(db DB, cfg ServiceConfig) *Service {
	return &Service{
		db:         db,
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
		input.TypeScale == nil &&
		input.SectionSpacing == nil &&
		input.ContentWidth == nil &&
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
	if input.TypeScale != nil {
		selection.TypeScale = strings.TrimSpace(*input.TypeScale)
	}
	if input.SectionSpacing != nil {
		selection.SectionSpacing = strings.TrimSpace(*input.SectionSpacing)
	}
	if input.ContentWidth != nil {
		selection.ContentWidth = strings.TrimSpace(*input.ContentWidth)
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

	draft.Theme = siteconfig.BuildThemeWithBrand(selection, draft.Brand)
	if err := s.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		return ThemeState{}, err
	}

	return themeStateFromDraft(draft), nil
}

func (s *Service) Regenerate(ctx context.Context, workspaceID string, siteID string) (ThemeState, error) {
	return s.RegenerateWithProgress(ctx, workspaceID, siteID, nil)
}

func (s *Service) RegenerateWithProgress(ctx context.Context, workspaceID string, siteID string, sink generation.ProgressSink) (ThemeState, error) {
	if s.regenerate == nil {
		return ThemeState{}, ErrThemeRegenerationOff
	}
	s.pruneGenerationJobs(ctx)

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

	jobID := ""
	if sink != nil {
		jobID, err = s.createGenerationJob(ctx, workspaceID, siteID, prompt)
		if err != nil {
			return ThemeState{}, err
		}
		sink.OnJobCreated(jobID)
		if err := s.emitGenerationProgress(ctx, jobID, "prompt.normalize", sink); err != nil {
			_ = s.failGenerationJob(ctx, jobID, err)
			return ThemeState{}, err
		}
		if err := s.emitGenerationProgress(ctx, jobID, "plan.theme", sink); err != nil {
			_ = s.failGenerationJob(ctx, jobID, err)
			return ThemeState{}, err
		}
	}

	selection, err := s.regenerate.RegenerateThemeSelection(ctx, prompt, draft)
	if err != nil {
		if jobID != "" {
			_ = s.failGenerationJob(ctx, jobID, err)
		}
		return ThemeState{}, err
	}
	if err := validateSelection(selection); err != nil {
		if jobID != "" {
			_ = s.failGenerationJob(ctx, jobID, err)
		}
		return ThemeState{}, err
	}
	if jobID != "" {
		if err := s.emitGenerationProgress(ctx, jobID, "validate.repair", sink); err != nil {
			_ = s.failGenerationJob(ctx, jobID, err)
			return ThemeState{}, err
		}
		if err := s.emitGenerationProgress(ctx, jobID, "persist", sink); err != nil {
			_ = s.failGenerationJob(ctx, jobID, err)
			return ThemeState{}, err
		}
	}

	draft.Theme = siteconfig.BuildThemeWithBrand(selection, draft.Brand)
	if err := s.writer.SaveDraft(ctx, workspaceID, draft); err != nil {
		if jobID != "" {
			_ = s.failGenerationJob(ctx, jobID, err)
		}
		return ThemeState{}, err
	}

	state := themeStateFromDraft(draft)
	if jobID != "" {
		if err := s.completeGenerationJob(ctx, jobID, siteID, state); err != nil {
			return ThemeState{}, err
		}
	}
	return state, nil
}

func themeStateFromDraft(draft siteconfig.SiteDraft) ThemeState {
	return ThemeState{
		Theme:     draft.Theme,
		Selection: siteconfig.DetectThemeSelection(draft.Theme),
		Options:   siteconfig.ThemeEditorCatalogWithBrand(draft.Brand),
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
	if !hasThemeOption(catalog.TypeScales, selection.TypeScale) {
		return ErrThemeTypeScaleInvalid
	}
	if !hasThemeOption(catalog.SectionSpacings, selection.SectionSpacing) {
		return ErrThemeSpacingInvalid
	}
	if !hasThemeOption(catalog.ContentWidths, selection.ContentWidth) {
		return ErrThemeContentWidthInvalid
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

func (s *Service) pruneGenerationJobs(ctx context.Context) {
	if s == nil || s.db == nil {
		return
	}
	_, _ = s.db.Exec(ctx, `
		delete from generation_jobs
		where coalesce(completed_at, started_at, created_at) < now() - interval '1 hour'
		  and state in ('succeeded', 'failed', 'canceled')
	`)
}

func (s *Service) createGenerationJob(ctx context.Context, workspaceID string, siteID string, prompt string) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("theme generation jobs unavailable")
	}
	payloadJSON, err := json.Marshal(map[string]any{
		"scope":  "theme",
		"siteId": siteID,
		"prompt": prompt,
	})
	if err != nil {
		return "", fmt.Errorf("encode theme generation payload: %w", err)
	}

	var jobID string
	if err := s.db.QueryRow(ctx, `
		insert into generation_jobs (site_id, workspace_id, kind, state, status, prompt, input_context, payload)
		values ($1::uuid, $2::uuid, $3, 'pending', 'queued', $4, $5, $5)
		returning id::text
	`, siteID, workspaceID, generation.JobKindThemeRegenerate, prompt, payloadJSON).Scan(&jobID); err != nil {
		return "", fmt.Errorf("create theme generation job: %w", err)
	}
	return jobID, nil
}

func (s *Service) emitGenerationProgress(ctx context.Context, jobID string, stepName string, sink generation.ProgressSink) error {
	if s == nil || s.db == nil || strings.TrimSpace(jobID) == "" {
		return nil
	}
	step := generation.StepForJob(generation.JobKindThemeRegenerate, stepName)
	if step == nil {
		return nil
	}
	if _, err := s.db.Exec(ctx, `
		update generation_jobs
		set state = 'running',
		    status = 'running',
		    current_step = $1,
		    error_reason = null,
		    started_at = coalesce(started_at, now()),
		    completed_at = null,
		    updated_at = now()
		where id = $2::uuid
	`, step.Name, jobID); err != nil {
		return fmt.Errorf("update theme generation progress: %w", err)
	}
	if sink != nil {
		sink.OnProgress(*step)
	}
	return nil
}

func (s *Service) completeGenerationJob(ctx context.Context, jobID string, siteID string, state ThemeState) error {
	if s == nil || s.db == nil || strings.TrimSpace(jobID) == "" {
		return nil
	}
	outputJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode theme generation output: %w", err)
	}
	if _, err := s.db.Exec(ctx, `
		update generation_jobs
		set site_id = $1::uuid,
		    state = 'succeeded',
		    status = 'completed',
		    output_plan = $2,
		    current_step = 'persist',
		    error = null,
		    error_reason = null,
		    completed_at = now(),
		    updated_at = now()
		where id = $3::uuid
	`, siteID, outputJSON, jobID); err != nil {
		return fmt.Errorf("mark theme generation job complete: %w", err)
	}
	return nil
}

func (s *Service) failGenerationJob(ctx context.Context, jobID string, cause error) error {
	if s == nil || s.db == nil || strings.TrimSpace(jobID) == "" {
		return nil
	}
	errorJSON, err := json.Marshal(map[string]string{
		"reason":  themeFailureReason(cause),
		"message": cause.Error(),
	})
	if err != nil {
		return fmt.Errorf("encode theme generation error: %w", err)
	}
	if _, err := s.db.Exec(ctx, `
		update generation_jobs
		set state = 'failed',
		    status = 'failed',
		    error = $1,
		    error_reason = $2,
		    completed_at = now(),
		    updated_at = now()
		where id = $3::uuid
	`, errorJSON, themeFailureReason(cause), jobID); err != nil {
		return fmt.Errorf("mark theme generation job failed: %w", err)
	}
	return nil
}

func themeFailureReason(err error) string {
	switch {
	case errors.Is(err, ErrNotFound):
		return "site_not_found"
	case errors.Is(err, ErrThemePaletteInvalid):
		return "invalid_theme_palette"
	case errors.Is(err, ErrThemeFontPresetInvalid):
		return "invalid_theme_font_preset"
	case errors.Is(err, ErrThemeTypeScaleInvalid):
		return "invalid_theme_type_scale"
	case errors.Is(err, ErrThemeSpacingInvalid):
		return "invalid_theme_section_spacing"
	case errors.Is(err, ErrThemeContentWidthInvalid):
		return "invalid_theme_content_width"
	case errors.Is(err, ErrThemeRadiusInvalid):
		return "invalid_theme_radius"
	case errors.Is(err, ErrThemeButtonStyleInvalid):
		return "invalid_theme_button_style"
	case errors.Is(err, ErrThemeImageStyleInvalid):
		return "invalid_theme_image_style"
	case errors.Is(err, ErrThemeRegenerationOff):
		return "theme_regeneration_unavailable"
	default:
		return "theme_regeneration_failed"
	}
}
