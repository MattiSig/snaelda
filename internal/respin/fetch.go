package respin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// UserAgent identifies the re-spin fetcher honestly (Spec 21 bot posture). It is
// a one-shot agent acting on the site owner's explicit request, not a crawler.
const UserAgent = "SnaeldaRespin/1.0 (+https://snaelda.io/respin-bot)"

// Default resource caps for the plain-fetch path (Spec 21 security contract).
const (
	defaultFetchTimeout = 10 * time.Second
	defaultMaxHTMLBytes = 5 << 20  // 5 MB per HTML document
	defaultMaxImgBytes  = 10 << 20 // 10 MB per ingested image
	maxRedirects        = 5
)

// Fetch-guard errors. They are sentinels so the pipeline can map a refusal to a
// specific degradation reason rather than a generic failure.
var (
	ErrDisallowedScheme = errors.New("respin: only http and https urls are allowed")
	ErrDisallowedPort   = errors.New("respin: only ports 80 and 443 are allowed")
	ErrCredentialsInURL = errors.New("respin: credentials in url are not allowed")
	ErrBlockedAddress   = errors.New("respin: url resolves to a blocked address")
	ErrTooManyRedirects = errors.New("respin: too many redirects")
	ErrResponseTooLarge = errors.New("respin: response exceeded the size cap")
	ErrEmptyURL         = errors.New("respin: url is required")
)

// FetcherConfig tunes the SSRF-guarded fetcher. The zero value is usable; every
// field falls back to a Spec 21 default.
type FetcherConfig struct {
	// UserAgent overrides the honest default identification string.
	UserAgent string
	// Timeout bounds a single fetch (DNS + connect + read). Defaults to 10s.
	Timeout time.Duration
	// Resolver overrides DNS resolution, primarily for tests. Production uses
	// the default system resolver.
	Resolver *net.Resolver

	// allowPrivateHosts disables the private/loopback address guard. It exists
	// solely so in-package tests can exercise the fetch/extract paths against a
	// loopback httptest server; production construction never sets it.
	allowPrivateHosts bool
}

// Fetcher performs SSRF-guarded HTTP fetches of attacker-controlled URLs. A
// single Fetcher is safe for concurrent use and backs page fetch, asset ingest,
// and same-origin discovery alike (Spec 21).
type Fetcher struct {
	userAgent    string
	timeout      time.Duration
	client       *http.Client
	resolver     *net.Resolver // nil means the default system resolver
	allowPrivate bool          // test-only; mirrors FetcherConfig.allowPrivateHosts
}

// NewFetcher builds a Fetcher whose transport rejects any connection to a
// private, loopback, link-local, or otherwise non-routable address. The guard
// runs on the dialer's Control hook, so it validates the exact IP the socket
// connects to — a DNS-rebinding flip between check and connect cannot redirect
// the request to an internal host.
func NewFetcher(cfg FetcherConfig) *Fetcher {
	ua := strings.TrimSpace(cfg.UserAgent)
	if ua == "" {
		ua = UserAgent
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultFetchTimeout
	}

	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 0, // one-shot fetches; no connection reuse benefit
		Resolver:  cfg.Resolver,
		// Control runs after DNS resolution with the concrete "ip:port" the
		// socket is about to dial. Rejecting a blocked address here is the
		// rebinding-proof chokepoint.
		Control: func(network, address string, _ syscall.RawConn) error {
			return guardDialAddress(network, address, cfg.allowPrivateHosts)
		},
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     false,
		Proxy:                 nil, // never route attacker URLs through a proxy
		MaxIdleConns:          0,
		IdleConnTimeout:       timeout,
		TLSHandshakeTimeout:   timeout,
		ExpectContinueTimeout: time.Second,
		DisableKeepAlives:     true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return ErrTooManyRedirects
			}
			// The IP guard re-runs automatically on each hop's dial; here we
			// re-validate the URL-level invariants (scheme, port, no creds)
			// that Control cannot see.
			return validateURL(req.URL, cfg.allowPrivateHosts)
		},
	}

	return &Fetcher{userAgent: ua, timeout: timeout, client: client, resolver: cfg.Resolver, allowPrivate: cfg.allowPrivateHosts}
}

// ValidatePublicURL is the intake-time SSRF pre-check (Spec 21 security
// contract): it enforces the URL-shape invariants (http/https, ports 80/443, no
// credentials) and resolves DNS to reject any hostname that points at a private,
// loopback, link-local, or cloud-metadata range — before an expensive import
// slot is spent. It is advisory hardening on top of the rebinding-proof
// dial-time guard, which remains the authoritative check on every connection.
func (f *Fetcher) ValidatePublicURL(ctx context.Context, rawURL string) error {
	u, err := normalizeFetchURL(rawURL)
	if err != nil {
		return err
	}
	if err := validateURL(u, f.allowPrivate); err != nil {
		return err
	}
	if f.allowPrivate {
		return nil
	}
	resolver := f.resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	ips, err := resolver.LookupIP(ctx, "ip", u.Hostname())
	if err != nil {
		return fmt.Errorf("%w: resolve %q: %v", ErrBlockedAddress, u.Hostname(), err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("%w: no addresses for %q", ErrBlockedAddress, u.Hostname())
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return ErrBlockedAddress
		}
	}
	return nil
}

// guardDialAddress rejects a resolved dial address that falls in a blocked
// range or uses a disallowed port. allowPrivate is a test-only escape hatch for
// loopback httptest servers and is never set in production.
func guardDialAddress(network, address string, allowPrivate bool) error {
	if !strings.HasPrefix(network, "tcp") {
		return fmt.Errorf("%w: network %q", ErrBlockedAddress, network)
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBlockedAddress, err)
	}
	if allowPrivate {
		// Test mode: loopback httptest servers bind an ephemeral port on a
		// loopback IP, both of which the production guards reject.
		return nil
	}
	if port != "80" && port != "443" {
		return ErrDisallowedPort
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// The dialer hands us a literal IP at this stage; a non-IP host means
		// resolution did not happen as expected — refuse rather than trust it.
		return fmt.Errorf("%w: unresolved host %q", ErrBlockedAddress, host)
	}
	if isBlockedIP(ip) {
		return ErrBlockedAddress
	}
	return nil
}

// validateURL enforces the URL-shape guards: scheme allow-list, port allow-list,
// and no credentials-in-URL. IP-range validation happens at dial time.
// allowAnyPort is a test-only relaxation for loopback httptest servers.
func validateURL(u *url.URL, allowAnyPort bool) error {
	switch u.Scheme {
	case "http", "https":
	default:
		return ErrDisallowedScheme
	}
	if u.User != nil {
		return ErrCredentialsInURL
	}
	if port := u.Port(); !allowAnyPort && port != "" && port != "80" && port != "443" {
		return ErrDisallowedPort
	}
	if strings.TrimSpace(u.Hostname()) == "" {
		return ErrEmptyURL
	}
	return nil
}

// FetchOptions tunes a single fetch.
type FetchOptions struct {
	// MaxBytes caps the response body. A body larger than the cap yields
	// ErrResponseTooLarge. Defaults to the 5 MB HTML cap.
	MaxBytes int64
	// Accept sets the Accept header sent upstream.
	Accept string
}

// FetchResult is the outcome of a guarded fetch.
type FetchResult struct {
	// FinalURL is the URL actually served after following redirects.
	FinalURL string
	// StatusCode is the HTTP status of the final response.
	StatusCode int
	// ContentType is the response Content-Type (lowercased, parameters kept).
	ContentType string
	// Body is the response body, capped at MaxBytes.
	Body []byte
}

// Fetch performs a guarded GET and returns the (size-capped) response body. It
// is the low-level primitive behind page fetch and asset ingest; callers that
// want HTML extraction use FetchPage.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string, opts FetchOptions) (FetchResult, error) {
	u, err := normalizeFetchURL(rawURL)
	if err != nil {
		return FetchResult{}, err
	}
	if err := validateURL(u, f.allowPrivate); err != nil {
		return FetchResult{}, err
	}

	maxBytes := opts.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxHTMLBytes
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("respin: build request: %w", err)
	}
	req.Header.Set("User-Agent", f.userAgent)
	if accept := strings.TrimSpace(opts.Accept); accept != "" {
		req.Header.Set("Accept", accept)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return FetchResult{}, unwrapFetchError(err)
	}
	defer resp.Body.Close()

	// Read one byte past the cap so an over-limit body is detected rather than
	// silently truncated into a valid-looking result.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return FetchResult{}, fmt.Errorf("respin: read body: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return FetchResult{}, ErrResponseTooLarge
	}

	return FetchResult{
		FinalURL:    resp.Request.URL.String(),
		StatusCode:  resp.StatusCode,
		ContentType: strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type"))),
		Body:        body,
	}, nil
}

// normalizeFetchURL parses and normalizes a raw URL for fetching: it lowercases
// the scheme and host, strips the fragment, and rejects obviously invalid
// inputs. Query-level tracking-param stripping is the intake layer's job (it
// also feeds the cache key); this keeps fetch honest to the given URL.
func normalizeFetchURL(rawURL string) (*url.URL, error) {
	raw := strings.TrimSpace(rawURL)
	if raw == "" {
		return nil, ErrEmptyURL
	}
	// A bare host ("example.com") is a common paste; default to https.
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("respin: parse url: %w", err)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	return u, nil
}

// unwrapFetchError surfaces our sentinel guard errors from the layers of
// wrapping that net/http and the dialer add, so callers can match on them.
func unwrapFetchError(err error) error {
	for _, sentinel := range []error{
		ErrBlockedAddress,
		ErrDisallowedPort,
		ErrDisallowedScheme,
		ErrCredentialsInURL,
		ErrTooManyRedirects,
	} {
		if errors.Is(err, sentinel) {
			return sentinel
		}
	}
	return fmt.Errorf("respin: fetch failed: %w", err)
}
