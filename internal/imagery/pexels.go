package imagery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// PexelsConfig controls how the Pexels client talks to the upstream API.
type PexelsConfig struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// PexelsClient implements Provider against api.pexels.com.
type PexelsClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewPexelsClient returns a configured Pexels Provider. When the API key is
// empty NewPexelsClient returns nil, which callers must treat as "provider
// unavailable" and fall back to placeholders.
func NewPexelsClient(cfg PexelsConfig) *PexelsClient {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.pexels.com/v1"
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}

	return &PexelsClient{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

func (p *PexelsClient) Name() string {
	return ProviderPexels
}

type pexelsSearchResponse struct {
	Photos []pexelsPhoto `json:"photos"`
	Error  string        `json:"error,omitempty"`
}

type pexelsPhoto struct {
	ID              int          `json:"id"`
	Width           int          `json:"width"`
	Height          int          `json:"height"`
	URL             string       `json:"url"`
	Photographer    string       `json:"photographer"`
	PhotographerURL string       `json:"photographer_url"`
	Alt             string       `json:"alt"`
	Source          pexelsSource `json:"src"`
}

type pexelsSource struct {
	Original  string `json:"original"`
	Large2X   string `json:"large2x"`
	Large     string `json:"large"`
	Medium    string `json:"medium"`
	Landscape string `json:"landscape"`
	Portrait  string `json:"portrait"`
}

func (p *PexelsClient) Search(ctx context.Context, input SearchInput) ([]Photo, error) {
	if p == nil {
		return nil, ErrUnavailable
	}
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return nil, ErrNoResults
	}

	count := input.Count
	if count <= 0 || count > 30 {
		count = 6
	}

	requestURL, err := url.Parse(p.baseURL + "/search")
	if err != nil {
		return nil, fmt.Errorf("imagery/pexels: build url: %w", err)
	}
	values := requestURL.Query()
	values.Set("query", query)
	values.Set("per_page", strconv.Itoa(count))
	if input.Orientation != OrientationAny {
		values.Set("orientation", string(input.Orientation))
	}
	requestURL.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("imagery/pexels: build request: %w", err)
	}
	req.Header.Set("Authorization", p.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("imagery/pexels: search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("imagery/pexels: read search response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("imagery/pexels: search failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload pexelsSearchResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("imagery/pexels: decode search response: %w", err)
	}
	if strings.TrimSpace(payload.Error) != "" {
		return nil, fmt.Errorf("imagery/pexels: %s", payload.Error)
	}
	if len(payload.Photos) == 0 {
		return nil, ErrNoResults
	}

	photos := make([]Photo, 0, len(payload.Photos))
	for _, photo := range payload.Photos {
		downloadURL := pickPexelsDownloadURL(photo.Source, input.Orientation)
		if downloadURL == "" {
			continue
		}
		photos = append(photos, Photo{
			Provider:    ProviderPexels,
			ProviderID:  strconv.Itoa(photo.ID),
			Width:       photo.Width,
			Height:      photo.Height,
			DownloadURL: downloadURL,
			ContentType: "image/jpeg",
			SourceURL:   photo.URL,
			Author:      photo.Photographer,
			AuthorURL:   photo.PhotographerURL,
			License:     LicensePexels,
			Description: photo.Alt,
		})
	}
	if len(photos) == 0 {
		return nil, ErrNoResults
	}
	return photos, nil
}

func (p *PexelsClient) Download(ctx context.Context, photo Photo) (PhotoData, error) {
	if p == nil {
		return PhotoData{}, ErrUnavailable
	}
	if strings.TrimSpace(photo.DownloadURL) == "" {
		return PhotoData{}, fmt.Errorf("imagery/pexels: photo missing download url")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, photo.DownloadURL, nil)
	if err != nil {
		return PhotoData{}, fmt.Errorf("imagery/pexels: build download request: %w", err)
	}
	req.Header.Set("Accept", "image/jpeg")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return PhotoData{}, fmt.Errorf("imagery/pexels: download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return PhotoData{}, fmt.Errorf("imagery/pexels: download failed (%d)", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return PhotoData{}, fmt.Errorf("imagery/pexels: read download body: %w", err)
	}
	if len(body) == 0 {
		return PhotoData{}, fmt.Errorf("imagery/pexels: empty download body")
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = photo.ContentType
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}

	final := photo
	final.ContentType = contentType
	return PhotoData{Photo: final, Body: body}, nil
}

func pickPexelsDownloadURL(source pexelsSource, orientation Orientation) string {
	switch orientation {
	case OrientationLandscape:
		if source.Landscape != "" {
			return source.Landscape
		}
	case OrientationPortrait:
		if source.Portrait != "" {
			return source.Portrait
		}
	}
	switch {
	case source.Large2X != "":
		return source.Large2X
	case source.Large != "":
		return source.Large
	case source.Medium != "":
		return source.Medium
	case source.Original != "":
		return source.Original
	}
	return ""
}
