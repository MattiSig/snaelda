package respin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testFetcher builds a Fetcher that permits loopback connections so httptest
// servers are reachable. Production construction never sets this.
func testFetcher() *Fetcher {
	return NewFetcher(FetcherConfig{allowPrivateHosts: true})
}

func TestFetchHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != UserAgent {
			t.Errorf("user-agent = %q, want %q", got, UserAgent)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body><main>hello world</main></body></html>"))
	}))
	defer srv.Close()

	res, err := testFetcher().Fetch(context.Background(), srv.URL, FetchOptions{})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if !strings.Contains(string(res.Body), "hello world") {
		t.Fatalf("body = %q", res.Body)
	}
}

func TestFetchRejectsScheme(t *testing.T) {
	_, err := testFetcher().Fetch(context.Background(), "ftp://example.com/file", FetchOptions{})
	if !errors.Is(err, ErrDisallowedScheme) {
		t.Fatalf("got %v, want ErrDisallowedScheme", err)
	}
}

func TestFetchRejectsCredentials(t *testing.T) {
	_, err := testFetcher().Fetch(context.Background(), "https://user:pass@example.com/", FetchOptions{})
	if !errors.Is(err, ErrCredentialsInURL) {
		t.Fatalf("got %v, want ErrCredentialsInURL", err)
	}
}

func TestFetchRejectsBadPort(t *testing.T) {
	// Production guard (no allowPrivateHosts) must reject a non-80/443 port.
	_, err := NewFetcher(FetcherConfig{}).Fetch(context.Background(), "https://example.com:8443/", FetchOptions{})
	if !errors.Is(err, ErrDisallowedPort) {
		t.Fatalf("got %v, want ErrDisallowedPort", err)
	}
}

func TestFetchSizeCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(strings.Repeat("x", 4096)))
	}))
	defer srv.Close()

	_, err := testFetcher().Fetch(context.Background(), srv.URL, FetchOptions{MaxBytes: 1024})
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("got %v, want ErrResponseTooLarge", err)
	}
}

func TestFetchSizeCapAtBoundary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(strings.Repeat("x", 1024)))
	}))
	defer srv.Close()

	res, err := testFetcher().Fetch(context.Background(), srv.URL, FetchOptions{MaxBytes: 1024})
	if err != nil {
		t.Fatalf("exact-cap body should pass: %v", err)
	}
	if len(res.Body) != 1024 {
		t.Fatalf("body len = %d, want 1024", len(res.Body))
	}
}

func TestFetchRedirectCap(t *testing.T) {
	var srv *httptest.Server
	hops := 0
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops++
		http.Redirect(w, r, srv.URL+"/next", http.StatusFound)
	}))
	defer srv.Close()

	_, err := testFetcher().Fetch(context.Background(), srv.URL, FetchOptions{})
	if !errors.Is(err, ErrTooManyRedirects) {
		t.Fatalf("got %v, want ErrTooManyRedirects", err)
	}
	if hops > maxRedirects+1 {
		t.Fatalf("followed %d hops, cap is %d", hops, maxRedirects)
	}
}

func TestFetchRedirectFollowedWithinCap(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/end", http.StatusFound)
	})
	mux.HandleFunc("/end", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>arrived</body></html>"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res, err := testFetcher().Fetch(context.Background(), srv.URL+"/start", FetchOptions{})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !strings.Contains(string(res.Body), "arrived") {
		t.Fatalf("did not follow redirect: %q", res.Body)
	}
	if !strings.HasSuffix(res.FinalURL, "/end") {
		t.Fatalf("final url = %q, want .../end", res.FinalURL)
	}
}

// TestFetchBlocksLoopbackByDefault confirms the production guard (no test
// escape hatch) refuses a loopback httptest server — the core SSRF property.
func TestFetchBlocksLoopbackByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("secret internal service"))
	}))
	defer srv.Close()

	_, err := NewFetcher(FetcherConfig{}).Fetch(context.Background(), srv.URL, FetchOptions{})
	if !errors.Is(err, ErrBlockedAddress) && !errors.Is(err, ErrDisallowedPort) {
		t.Fatalf("loopback fetch should be blocked, got %v", err)
	}
}

func TestFetchEmptyURL(t *testing.T) {
	_, err := testFetcher().Fetch(context.Background(), "   ", FetchOptions{})
	if !errors.Is(err, ErrEmptyURL) {
		t.Fatalf("got %v, want ErrEmptyURL", err)
	}
}

func TestValidatePublicURLRejectsBadShapes(t *testing.T) {
	f := NewFetcher(FetcherConfig{})
	cases := []struct {
		name string
		url  string
	}{
		{"scheme", "ftp://example.com"},
		{"credentials", "https://user:pass@example.com"},
		{"loopback-literal", "http://127.0.0.1/"},
		{"metadata", "http://169.254.169.254/latest/meta-data/"},
		{"private-literal", "http://10.0.0.5/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := f.ValidatePublicURL(context.Background(), tc.url); err == nil {
				t.Fatalf("expected %s to be rejected", tc.url)
			}
		})
	}
}

func TestValidatePublicURLAllowsPublicShape(t *testing.T) {
	// The test fetcher skips DNS/IP checks, so a well-formed https URL passes the
	// shape validation without a network dependency.
	if err := testFetcher().ValidatePublicURL(context.Background(), "https://example.com/path"); err != nil {
		t.Fatalf("expected well-formed url to pass, got %v", err)
	}
}
