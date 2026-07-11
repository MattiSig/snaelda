//go:build integration

package respin

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/MattiSig/snaelda/internal/platform/database"
	"github.com/MattiSig/snaelda/internal/platform/ids"
)

func TestRespinImportLifecycle(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is required for the integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := database.Open(ctx, url)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer pool.Close()

	store := NewService(pool)

	// A public-demo import exists before any workspace does.
	created, err := store.Create(ctx, CreateInput{
		SourceURL:     "https://Example.com/CafÉ?utm_source=fb#menu",
		NormalizedURL: "https://example.com/cafe",
	})
	if err != nil {
		t.Fatalf("create import: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from respin_imports where id = $1::uuid`, created.ID)
	})

	if created.ID == "" {
		t.Fatal("expected a generated import id")
	}
	if created.WorkspaceID != "" {
		t.Fatalf("expected an unclaimed import, got workspace %q", created.WorkspaceID)
	}
	if created.FetchStatus != StatusQueued {
		t.Fatalf("expected queued status, got %q", created.FetchStatus)
	}
	if len(created.PulledAssetIDs) != 0 {
		t.Fatalf("expected empty pulled asset ids, got %v", created.PulledAssetIDs)
	}

	// Round-trip a status transition that records the fetch mode.
	if _, err := store.UpdateStatus(ctx, created.ID, StatusFetching, ModePlain); err != nil {
		t.Fatalf("update status: %v", err)
	}
	fetched, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get import: %v", err)
	}
	if fetched.FetchStatus != StatusFetching || fetched.FetchMode != ModePlain {
		t.Fatalf("expected fetching/plain, got %q/%q", fetched.FetchStatus, fetched.FetchMode)
	}

	// A later status change without a mode must not clear the recorded mode.
	if _, err := store.UpdateStatus(ctx, created.ID, StatusExtracting, ""); err != nil {
		t.Fatalf("update status without mode: %v", err)
	}
	fetched, _ = store.Get(ctx, created.ID)
	if fetched.FetchMode != ModePlain {
		t.Fatalf("expected fetch mode preserved, got %q", fetched.FetchMode)
	}

	// Persist extraction + classification and pulled assets.
	extraction := json.RawMessage(`{"name":"Café Snælda","services":["kaffi"]}`)
	classification := json.RawMessage(`{"vertical":"cafe","locale":"is"}`)
	if _, err := store.SaveExtraction(ctx, created.ID, extraction, classification); err != nil {
		t.Fatalf("save extraction: %v", err)
	}
	assetIDs := []string{mustID(t), mustID(t)}
	if _, err := store.SavePulledAssets(ctx, created.ID, assetIDs); err != nil {
		t.Fatalf("save pulled assets: %v", err)
	}
	if _, err := store.UpdateStatus(ctx, created.ID, StatusSucceeded, ""); err != nil {
		t.Fatalf("mark succeeded: %v", err)
	}

	loaded, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("reload import: %v", err)
	}
	if len(loaded.ExtractedContent) == 0 || len(loaded.Classification) == 0 {
		t.Fatal("expected extraction and classification to persist")
	}
	if len(loaded.PulledAssetIDs) != 2 || loaded.PulledAssetIDs[0] != assetIDs[0] {
		t.Fatalf("expected pulled asset ids to round-trip, got %v", loaded.PulledAssetIDs)
	}

	// Cache lookup by normalized URL within the TTL window.
	cached, err := store.FindCached(ctx, "https://example.com/cafe", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("find cached: %v", err)
	}
	if cached.ID != created.ID {
		t.Fatalf("expected cached hit for created import, got %q", cached.ID)
	}
	// A window that excludes the import must miss.
	if _, err := store.FindCached(ctx, "https://example.com/cafe", time.Now().Add(time.Hour)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected cache miss for future window, got %v", err)
	}

	// Share slug assignment + lookup.
	slug := "cafe-" + created.ID[:8]
	if _, err := store.AssignShareSlug(ctx, created.ID, slug); err != nil {
		t.Fatalf("assign share slug: %v", err)
	}
	bySlug, err := store.GetByShareSlug(ctx, slug)
	if err != nil {
		t.Fatalf("get by share slug: %v", err)
	}
	if bySlug.ID != created.ID {
		t.Fatalf("expected share slug lookup to resolve, got %q", bySlug.ID)
	}

	// Claim binds the import to a real workspace.
	workspaceID := mustID(t)
	if _, err := pool.Exec(ctx, `insert into workspaces (id, name) values ($1::uuid, 'Respin claim integration')`, workspaceID); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from workspaces where id = $1::uuid`, workspaceID)
	})

	claimed, err := store.Claim(ctx, created.ID, workspaceID, "")
	if err != nil {
		t.Fatalf("claim import: %v", err)
	}
	if claimed.WorkspaceID != workspaceID {
		t.Fatalf("expected claimed workspace %q, got %q", workspaceID, claimed.WorkspaceID)
	}
	// Re-claiming an already-claimed import is rejected.
	if _, err := store.Claim(ctx, created.ID, mustID(t), ""); !errors.Is(err, ErrAlreadyClaimed) {
		t.Fatalf("expected ErrAlreadyClaimed, got %v", err)
	}

	// A generation job can carry the provenance linkage.
	jobID := mustID(t)
	if _, err := pool.Exec(ctx, `
		insert into generation_jobs (id, workspace_id, kind, prompt, respin_import_id)
		values ($1::uuid, $2::uuid, 'site', 'composed brief', $3::uuid)
	`, jobID, workspaceID, created.ID); err != nil {
		t.Fatalf("insert linked generation job: %v", err)
	}
	var linkedImportID string
	if err := pool.QueryRow(ctx, `select respin_import_id::text from generation_jobs where id = $1::uuid`, jobID).Scan(&linkedImportID); err != nil {
		t.Fatalf("read generation job linkage: %v", err)
	}
	if linkedImportID != created.ID {
		t.Fatalf("expected job linked to import %q, got %q", created.ID, linkedImportID)
	}

	// Missing lookups map to ErrNotFound.
	if _, err := store.Get(ctx, mustID(t)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing id, got %v", err)
	}
}

func TestRespinDegradeFailAndGC(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is required for the integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := database.Open(ctx, url)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer pool.Close()

	store := NewService(pool)

	degraded, err := store.Create(ctx, CreateInput{SourceURL: "https://thin.example", NormalizedURL: "https://thin.example"})
	if err != nil {
		t.Fatalf("create degraded import: %v", err)
	}
	failed, err := store.Create(ctx, CreateInput{SourceURL: "https://boom.example", NormalizedURL: "https://boom.example"})
	if err != nil {
		t.Fatalf("create failed import: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `delete from respin_imports where id = any($1::uuid[])`, []string{degraded.ID, failed.ID})
	})

	got, err := store.MarkDegraded(ctx, degraded.ID, "thin content")
	if err != nil {
		t.Fatalf("mark degraded: %v", err)
	}
	if !got.Degraded || got.FetchStatus != StatusDegraded || got.DegradationReason != "thin content" {
		t.Fatalf("unexpected degraded state: %+v", got)
	}

	got, err = store.Fail(ctx, failed.ID, json.RawMessage(`{"code":"fetch_timeout"}`))
	if err != nil {
		t.Fatalf("fail import: %v", err)
	}
	if got.FetchStatus != StatusFailed || len(got.Error) == 0 {
		t.Fatalf("unexpected failed state: %+v", got)
	}

	// GC removes unclaimed imports created before the cutoff.
	removed, err := store.DeleteUnclaimedBefore(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("gc unclaimed: %v", err)
	}
	if removed < 2 {
		t.Fatalf("expected at least 2 unclaimed imports removed, got %d", removed)
	}
}

func mustID(t *testing.T) string {
	t.Helper()
	value, err := ids.New()
	if err != nil {
		t.Fatalf("generate id: %v", err)
	}
	return value
}
