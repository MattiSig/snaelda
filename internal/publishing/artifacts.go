package publishing

import (
	"context"
	"errors"

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
	Files         []string               `json:"files,omitempty"`
}

type ArtifactManifestPage struct {
	PageID       string `json:"pageId,omitempty"`
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
	Delete(ctx context.Context, siteID string, versionID string) error
}

var ErrArtifactNotFound = errors.New("published artifact not found")

// Closer is implemented by renderers and stores that hold long-lived resources
// (e.g., a render worker process or an HTTP client connection pool) and need to
// be released on graceful shutdown.
type Closer interface {
	Close() error
}
