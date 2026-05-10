package themes

import (
	"context"
	"errors"
	"testing"

	"github.com/MattiSig/snaelda/internal/siteconfig"
	"github.com/MattiSig/snaelda/internal/sites"
)

type stubReader struct {
	draft siteconfig.SiteDraft
	err   error
}

func (r stubReader) LoadDraft(context.Context, string) (siteconfig.SiteDraft, error) {
	return r.draft, r.err
}

type stubWriter struct {
	workspaceID string
	draft       siteconfig.SiteDraft
	err         error
}

func (w *stubWriter) SaveDraft(_ context.Context, workspaceID string, draft siteconfig.SiteDraft) error {
	w.workspaceID = workspaceID
	w.draft = draft
	return w.err
}

func TestLoadReturnsDetectedThemeState(t *testing.T) {
	service := Service{
		reader: stubReader{draft: sampleThemeDraft()},
		writer: &stubWriter{},
	}

	state, err := service.Load(context.Background(), "site_demo")
	if err != nil {
		t.Fatalf("load theme: %v", err)
	}
	if state.Selection.Palette != siteconfig.ThemePaletteMeanerDark {
		t.Fatalf("expected detected palette, got %#v", state.Selection)
	}
	if len(state.Options.Palettes) == 0 {
		t.Fatal("expected theme palette options")
	}
}

func TestUpdateRebuildsThemeFromSelection(t *testing.T) {
	writer := &stubWriter{}
	service := Service{
		reader: stubReader{draft: sampleThemeDraft()},
		writer: writer,
	}

	state, err := service.Update(context.Background(), "workspace-1", "site_demo", UpdateInput{
		Palette:        stringPointer(siteconfig.ThemePalettePlayfulRibbon),
		FontPreset:     stringPointer(siteconfig.ThemeFontStudioSans),
		SectionSpacing: stringPointer(siteconfig.ThemeSpacingSnug),
		Radius:         stringPointer(siteconfig.ThemeRadiusPillowy),
	})
	if err != nil {
		t.Fatalf("update theme: %v", err)
	}
	if writer.workspaceID != "workspace-1" {
		t.Fatalf("expected workspace id to reach writer, got %q", writer.workspaceID)
	}
	if writer.draft.Theme.Tokens.Colors["background"] != "#fff7ee" {
		t.Fatalf("expected playful palette background, got %#v", writer.draft.Theme.Tokens.Colors)
	}
	if state.Selection.Palette != siteconfig.ThemePalettePlayfulRibbon {
		t.Fatalf("expected updated selection, got %#v", state.Selection)
	}
}

func TestUpdateRejectsUnknownPalette(t *testing.T) {
	service := Service{
		reader: stubReader{draft: sampleThemeDraft()},
		writer: &stubWriter{},
	}

	_, err := service.Update(context.Background(), "workspace-1", "site_demo", UpdateInput{
		Palette: stringPointer("unknown"),
	})
	if !errors.Is(err, ErrThemePaletteInvalid) {
		t.Fatalf("expected invalid palette error, got %v", err)
	}
}

func TestUpdateMapsMissingDraftToNotFound(t *testing.T) {
	service := Service{
		reader: stubReader{err: sites.ErrNotFound},
		writer: &stubWriter{},
	}

	_, err := service.Update(context.Background(), "workspace-1", "site_demo", UpdateInput{
		Radius: stringPointer(siteconfig.ThemeRadiusSoft),
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func sampleThemeDraft() siteconfig.SiteDraft {
	return siteconfig.SiteDraft{
		Site: siteconfig.DraftSite{
			ID:     "site_demo",
			Name:   "Nordic Studio",
			Slug:   "nordic-studio",
			Status: "draft",
		},
		Theme:      siteconfig.ThemePreset(siteconfig.ThemePaletteMeanerDark),
		Navigation: siteconfig.NavigationConfig{Primary: []siteconfig.NavigationItem{{Label: "Home", PageID: "page_home"}}},
		Pages: []siteconfig.PageDraft{{
			ID:    "page_home",
			Title: "Home",
			Slug:  "/",
			Blocks: []siteconfig.BlockInstance{{
				ID:      "block_hero",
				Type:    "hero",
				Version: siteconfig.BlockVersionV1,
				Props: map[string]any{
					"headline": "Structured sites for small studios",
				},
			}},
		}},
	}
}

func stringPointer(value string) *string {
	return &value
}
