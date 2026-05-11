package publishing

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/MattiSig/snaelda/internal/siteconfig"
)

type ArtifactFile struct {
	Path        string `json:"path"`
	ContentType string `json:"contentType"`
	Body        string `json:"body"`
}

type ArtifactBundle struct {
	SchemaVersion string         `json:"schemaVersion"`
	Files         []ArtifactFile `json:"files"`
}

type ArtifactRenderInput struct {
	AppBaseURL string                       `json:"appBaseURL"`
	SiteSlug   string                       `json:"siteSlug"`
	Hostname   string                       `json:"hostname,omitempty"`
	Version    VersionSummary               `json:"version"`
	Snapshot   siteconfig.PublishedSnapshot `json:"snapshot"`
}

type ArtifactRenderer interface {
	Render(ctx context.Context, input ArtifactRenderInput) (ArtifactBundle, error)
}

type ArtifactStore interface {
	Save(ctx context.Context, siteID string, versionID string, bundle ArtifactBundle) error
}

type commandArtifactRenderer struct {
	appBaseURL string
}

func newCommandArtifactRenderer(appBaseURL string) ArtifactRenderer {
	return &commandArtifactRenderer{appBaseURL: strings.TrimSpace(appBaseURL)}
}

func (r *commandArtifactRenderer) Render(ctx context.Context, input ArtifactRenderInput) (ArtifactBundle, error) {
	payload := input
	if strings.TrimSpace(payload.AppBaseURL) == "" {
		payload.AppBaseURL = r.appBaseURL
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ArtifactBundle{}, fmt.Errorf("encode artifact render input: %w", err)
	}

	cmd := exec.CommandContext(
		ctx,
		"npm",
		"run",
		"--workspace",
		"@snaelda/web",
		"--silent",
		"render:artifacts",
	)
	cmd.Stdin = strings.NewReader(string(body))

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return ArtifactBundle{}, fmt.Errorf("render published artifacts: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return ArtifactBundle{}, fmt.Errorf("render published artifacts: %w", err)
	}

	var bundle ArtifactBundle
	if err := json.Unmarshal(output, &bundle); err != nil {
		return ArtifactBundle{}, fmt.Errorf("decode rendered artifacts: %w", err)
	}
	if bundle.SchemaVersion == "" {
		return ArtifactBundle{}, fmt.Errorf("decode rendered artifacts: missing schema version")
	}
	return bundle, nil
}
