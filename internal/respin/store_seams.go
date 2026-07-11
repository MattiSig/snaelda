package respin

import (
	"context"
	"encoding/json"
	"time"
)

// pipelineStore is the subset of the respin store the pipeline drives. Narrowing
// it to an interface keeps the orchestration logic unit-testable without a
// database; *Service satisfies it.
type pipelineStore interface {
	UpdateStatus(ctx context.Context, id, status, mode string) (Import, error)
	SaveExtraction(ctx context.Context, id string, extractedContent, classification json.RawMessage) (Import, error)
	SavePulledAssets(ctx context.Context, id string, assetIDs []string) (Import, error)
	MarkDegraded(ctx context.Context, id, reason string) (Import, error)
	Fail(ctx context.Context, id string, errPayload json.RawMessage) (Import, error)
	AssignShareSlug(ctx context.Context, id, slug string) (Import, error)
	LinkGenerationJob(ctx context.Context, jobID, importID string) error
}

// handlerStore is the subset of the respin store the HTTP endpoints use.
type handlerStore interface {
	Create(ctx context.Context, input CreateInput) (Import, error)
	Get(ctx context.Context, id string) (Import, error)
	GetByShareSlug(ctx context.Context, slug string) (Import, error)
	FindCached(ctx context.Context, normalizedURL string, notBefore time.Time) (Import, error)
	Claim(ctx context.Context, id, workspaceID, guestSessionID string) (Import, error)
	LinkedGeneration(ctx context.Context, importID string) (GenerationLink, error)
}

var (
	_ pipelineStore = (*Service)(nil)
	_ handlerStore  = (*Service)(nil)
)
