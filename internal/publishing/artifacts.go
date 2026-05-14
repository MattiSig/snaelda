package publishing

import (
	"context"
	"encoding/json"
	"errors"
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

type ArtifactManifest struct {
	SchemaVersion string                 `json:"schemaVersion"`
	SiteSlug      string                 `json:"siteSlug"`
	Hostname      string                 `json:"hostname,omitempty"`
	Version       VersionSummary         `json:"version"`
	Pages         []ArtifactManifestPage `json:"pages"`
}

type ArtifactManifestPage struct {
	PagePath     string `json:"pagePath"`
	FilePath     string `json:"filePath"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	CanonicalURL string `json:"canonicalUrl"`
}

type ArtifactRenderInput struct {
	PublicBaseURL string                       `json:"publicBaseURL"`
	SiteSlug      string                       `json:"siteSlug"`
	Hostname      string                       `json:"hostname,omitempty"`
	Version       VersionSummary               `json:"version"`
	Snapshot      siteconfig.PublishedSnapshot `json:"snapshot"`
}

type ArtifactRenderer interface {
	Render(ctx context.Context, input ArtifactRenderInput) (ArtifactBundle, error)
}

type ArtifactStore interface {
	Save(ctx context.Context, siteID string, versionID string, bundle ArtifactBundle) error
	Load(ctx context.Context, siteID string, versionID string, path string) (ArtifactFile, error)
}

var ErrArtifactNotFound = errors.New("published artifact not found")

type commandArtifactRenderer struct {
	publicBaseURL string
}

func newCommandArtifactRenderer(publicBaseURL string) ArtifactRenderer {
	return &commandArtifactRenderer{publicBaseURL: strings.TrimSpace(publicBaseURL)}
}

func (r *commandArtifactRenderer) Render(ctx context.Context, input ArtifactRenderInput) (ArtifactBundle, error) {
	payload := input
	if strings.TrimSpace(payload.PublicBaseURL) == "" {
		payload.PublicBaseURL = r.publicBaseURL
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
