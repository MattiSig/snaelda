// Package imagery contains backend-owned providers that supply starter
// imagery for generated drafts. Today only Pexels is wired up. The package
// keeps provider details behind the Provider interface so the upstream
// generation flow does not need to know which photo service is in use, and
// so the provider can be swapped without rewriting callers.
package imagery

import (
	"context"
	"errors"
	"strings"
	"sync"
)

const (
	ProviderPexels = "pexels"
	LicensePexels  = "Pexels License"
)

// ErrNoResults is returned when the provider succeeds but finds no images
// matching the supplied query. Callers fall back to placeholder content.
var ErrNoResults = errors.New("imagery: no results")

// ErrUnavailable signals that the provider is not configured (for example
// no API key) and the caller should silently fall back to placeholders.
var ErrUnavailable = errors.New("imagery: provider unavailable")

// Orientation hints which aspect the generation flow needs for a slot.
type Orientation string

const (
	OrientationAny       Orientation = ""
	OrientationLandscape Orientation = "landscape"
	OrientationPortrait  Orientation = "portrait"
	OrientationSquare    Orientation = "square"
)

// Photo describes a single image candidate returned by a provider.
type Photo struct {
	Provider    string
	ProviderID  string
	Width       int
	Height      int
	DownloadURL string
	ContentType string
	SourceURL   string
	Author      string
	AuthorURL   string
	License     string
	Description string
}

// PhotoData carries the binary payload of a downloaded photo.
type PhotoData struct {
	Photo
	Body []byte
}

// SearchInput describes a single search call.
type SearchInput struct {
	Query       string
	Orientation Orientation
	Count       int
}

// Provider is the contract every starter-image source implements.
type Provider interface {
	Name() string
	Search(ctx context.Context, input SearchInput) ([]Photo, error)
	Download(ctx context.Context, photo Photo) (PhotoData, error)
}

// DedupedProvider wraps a provider with a per-instance set of provider IDs
// that have already been picked. The generation flow creates one of these
// per draft run so the same hero image does not appear in multiple blocks.
type DedupedProvider struct {
	inner Provider
	mu    sync.Mutex
	seen  map[string]bool
}

// NewDedupedProvider returns a DedupedProvider wrapped around inner. The
// returned value is safe for concurrent use within a single generation run.
func NewDedupedProvider(inner Provider) *DedupedProvider {
	return &DedupedProvider{
		inner: inner,
		seen:  map[string]bool{},
	}
}

func (d *DedupedProvider) Name() string {
	if d == nil || d.inner == nil {
		return ""
	}
	return d.inner.Name()
}

// PickFresh searches with each query in order and returns the first photo
// whose provider id has not yet been used in this run. The picked photo is
// recorded so subsequent calls cannot reuse it.
func (d *DedupedProvider) PickFresh(ctx context.Context, queries []string, orientation Orientation) (Photo, error) {
	if d == nil || d.inner == nil {
		return Photo{}, ErrUnavailable
	}

	var lastErr error
	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}

		photos, err := d.inner.Search(ctx, SearchInput{
			Query:       query,
			Orientation: orientation,
			Count:       6,
		})
		if err != nil {
			lastErr = err
			continue
		}

		d.mu.Lock()
		for _, photo := range photos {
			if photo.ProviderID == "" {
				continue
			}
			if d.seen[photo.ProviderID] {
				continue
			}
			d.seen[photo.ProviderID] = true
			d.mu.Unlock()
			return photo, nil
		}
		d.mu.Unlock()
	}

	if lastErr != nil {
		return Photo{}, lastErr
	}
	return Photo{}, ErrNoResults
}

// Download proxies to the inner provider.
func (d *DedupedProvider) Download(ctx context.Context, photo Photo) (PhotoData, error) {
	if d == nil || d.inner == nil {
		return PhotoData{}, ErrUnavailable
	}
	return d.inner.Download(ctx, photo)
}

// Inner exposes the underlying provider for tests.
func (d *DedupedProvider) Inner() Provider {
	if d == nil {
		return nil
	}
	return d.inner
}
