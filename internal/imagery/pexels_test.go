package imagery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPexelsClientSearchReturnsPhotos(t *testing.T) {
	var receivedAuth string
	var receivedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedQuery = r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"photos":[{"id":12345,"width":4000,"height":2667,"url":"https://www.pexels.com/photo/12345","photographer":"Test Photographer","photographer_url":"https://www.pexels.com/@test","alt":"A test alt","src":{"original":"https://images.pexels.com/photos/12345/original.jpg","large2x":"https://images.pexels.com/photos/12345/large2x.jpg","landscape":"https://images.pexels.com/photos/12345/landscape.jpg"}}]}`))
	}))
	defer server.Close()

	client := NewPexelsClient(PexelsConfig{APIKey: "test-key", BaseURL: server.URL})
	if client == nil {
		t.Fatal("expected non-nil client")
	}

	photos, err := client.Search(context.Background(), SearchInput{Query: "stockholm photography", Orientation: OrientationLandscape, Count: 5})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if receivedAuth != "test-key" {
		t.Fatalf("expected authorization header, got %q", receivedAuth)
	}
	if receivedQuery != "stockholm photography" {
		t.Fatalf("expected query forwarded, got %q", receivedQuery)
	}
	if len(photos) != 1 {
		t.Fatalf("expected single photo, got %d", len(photos))
	}
	photo := photos[0]
	if photo.Provider != ProviderPexels {
		t.Fatalf("expected provider=%q, got %q", ProviderPexels, photo.Provider)
	}
	if photo.ProviderID != "12345" {
		t.Fatalf("expected provider id 12345, got %q", photo.ProviderID)
	}
	if photo.Author != "Test Photographer" {
		t.Fatalf("expected author copied, got %q", photo.Author)
	}
	if photo.License != LicensePexels {
		t.Fatalf("expected pexels license, got %q", photo.License)
	}
	if photo.DownloadURL != "https://images.pexels.com/photos/12345/landscape.jpg" {
		t.Fatalf("expected landscape image, got %q", photo.DownloadURL)
	}
}

func TestPexelsClientSearchEmptyResultsReturnsErrNoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"photos":[]}`))
	}))
	defer server.Close()

	client := NewPexelsClient(PexelsConfig{APIKey: "test-key", BaseURL: server.URL})
	if _, err := client.Search(context.Background(), SearchInput{Query: "nothing here"}); err != ErrNoResults {
		t.Fatalf("expected ErrNoResults, got %v", err)
	}
}

func TestPexelsClientSearchPropagatesUpstreamFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limit"}`))
	}))
	defer server.Close()

	client := NewPexelsClient(PexelsConfig{APIKey: "test-key", BaseURL: server.URL})
	_, err := client.Search(context.Background(), SearchInput{Query: "anything"})
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("expected 429 error, got %v", err)
	}
}

func TestPexelsClientReturnsNilWhenKeyMissing(t *testing.T) {
	if NewPexelsClient(PexelsConfig{APIKey: ""}) != nil {
		t.Fatal("expected nil client when api key is empty")
	}
}

func TestPexelsClientDownloadBinary(t *testing.T) {
	body := []byte("fake-jpeg-bytes")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(body)
	}))
	defer server.Close()

	client := NewPexelsClient(PexelsConfig{APIKey: "test-key", BaseURL: server.URL})
	photo := Photo{Provider: ProviderPexels, ProviderID: "1", DownloadURL: server.URL + "/photo.jpg", ContentType: "image/jpeg"}
	data, err := client.Download(context.Background(), photo)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if string(data.Body) != string(body) {
		t.Fatalf("expected body, got %q", data.Body)
	}
	if data.ContentType != "image/jpeg" {
		t.Fatalf("expected image/jpeg content type, got %q", data.ContentType)
	}
}

func TestDedupedProviderSkipsAlreadyUsedPhotos(t *testing.T) {
	stub := &stubProvider{
		photos: []Photo{
			{Provider: ProviderPexels, ProviderID: "1", DownloadURL: "https://example/1"},
			{Provider: ProviderPexels, ProviderID: "2", DownloadURL: "https://example/2"},
		},
	}
	deduped := NewDedupedProvider(stub)

	first, err := deduped.PickFresh(context.Background(), []string{"a"}, OrientationLandscape)
	if err != nil {
		t.Fatalf("first pick: %v", err)
	}
	if first.ProviderID != "1" {
		t.Fatalf("expected first photo, got %q", first.ProviderID)
	}

	second, err := deduped.PickFresh(context.Background(), []string{"b"}, OrientationLandscape)
	if err != nil {
		t.Fatalf("second pick: %v", err)
	}
	if second.ProviderID != "2" {
		t.Fatalf("expected dedup to skip first photo, got %q", second.ProviderID)
	}
}

func TestDedupedProviderReturnsErrNoResultsWhenAllExhausted(t *testing.T) {
	stub := &stubProvider{photos: []Photo{{Provider: ProviderPexels, ProviderID: "x", DownloadURL: "https://example/x"}}}
	deduped := NewDedupedProvider(stub)
	if _, err := deduped.PickFresh(context.Background(), []string{"a"}, OrientationLandscape); err != nil {
		t.Fatalf("first pick: %v", err)
	}
	_, err := deduped.PickFresh(context.Background(), []string{"a"}, OrientationLandscape)
	if err != ErrNoResults {
		t.Fatalf("expected ErrNoResults, got %v", err)
	}
}

type stubProvider struct {
	photos []Photo
}

func (s *stubProvider) Name() string { return "stub" }

func (s *stubProvider) Search(ctx context.Context, input SearchInput) ([]Photo, error) {
	if len(s.photos) == 0 {
		return nil, ErrNoResults
	}
	return s.photos, nil
}

func (s *stubProvider) Download(ctx context.Context, photo Photo) (PhotoData, error) {
	return PhotoData{Photo: photo, Body: []byte("body")}, nil
}
