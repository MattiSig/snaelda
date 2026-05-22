package domains

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	domainTypeSubdomain = "subdomain"
	domainTypeCustom    = "custom"

	domainStatusActive  = "active"
	domainStatusPending = "pending"

	verificationPrefix = "_snaelda-verify"
)

var (
	ErrNotFound             = errors.New("site not found")
	ErrDomainNotFound       = errors.New("domain not found")
	ErrInvalidHostname      = errors.New("invalid hostname")
	ErrHostnameConflict     = errors.New("hostname is already in use")
	ErrReservedHostname     = errors.New("hostname is reserved for hosted subdomains")
	ErrManagedDomain        = errors.New("managed hosted domain cannot be changed")
	ErrVerificationNotReady = errors.New("verification record was not found")
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type CacheInvalidator interface {
	InvalidateSite(siteID string)
	InvalidateHostname(hostname string)
}

type dnsLookupTXT func(ctx context.Context, name string) ([]string, error)

type Service struct {
	db               DB
	appBaseURL       string
	publicBaseURL    string
	publicBaseDomain string
	cache            CacheInvalidator
	lookupTXT        dnsLookupTXT
}

type SiteDomainsResult struct {
	SiteID               string        `json:"siteId"`
	SiteSlug             string        `json:"siteSlug"`
	Published            bool          `json:"published"`
	HostedHostname       string        `json:"hostedHostname"`
	PublicURL            string        `json:"publicUrl,omitempty"`
	CustomDomainsEnabled bool          `json:"customDomainsEnabled"`
	Domains              []DomainEntry `json:"domains"`
}

type DomainEntry struct {
	ID                   string `json:"id"`
	Hostname             string `json:"hostname"`
	Type                 string `json:"type"`
	Status               string `json:"status"`
	PublicURL            string `json:"publicUrl,omitempty"`
	VerificationHostname string `json:"verificationHostname,omitempty"`
	VerificationValue    string `json:"verificationValue,omitempty"`
}

type ServiceConfig struct {
	AppBaseURL       string
	PublicBaseURL    string
	PublicBaseDomain string
	Cache            CacheInvalidator
	LookupTXT        dnsLookupTXT
}

func NewService(db DB, cfg ServiceConfig) *Service {
	lookup := cfg.LookupTXT
	if lookup == nil {
		resolver := net.DefaultResolver
		lookup = resolver.LookupTXT
	}
	return &Service{
		db:               db,
		appBaseURL:       strings.TrimRight(strings.TrimSpace(cfg.AppBaseURL), "/"),
		publicBaseURL:    strings.TrimSpace(cfg.PublicBaseURL),
		publicBaseDomain: normalizeHostname(cfg.PublicBaseDomain),
		cache:            cfg.Cache,
		lookupTXT:        lookup,
	}
}

func (s *Service) List(ctx context.Context, siteID string) (SiteDomainsResult, error) {
	siteID = strings.TrimSpace(siteID)
	if siteID == "" {
		return SiteDomainsResult{}, ErrNotFound
	}

	var result SiteDomainsResult
	if err := s.db.QueryRow(ctx, `
		select id::text,
		       slug,
		       published_version_id is not null
		from sites
		where id = $1
	`, siteID).Scan(&result.SiteID, &result.SiteSlug, &result.Published); errors.Is(err, pgx.ErrNoRows) {
		return SiteDomainsResult{}, ErrNotFound
	} else if err != nil {
		return SiteDomainsResult{}, fmt.Errorf("load site domain summary: %w", err)
	}

	result.HostedHostname = buildHostedHostname(result.SiteSlug, s.publicBaseDomain)
	if result.Published {
		result.PublicURL = buildPublicURL(s.appBaseURL, s.publicBaseURL, result.HostedHostname)
	}
	_ = s.db.QueryRow(ctx, `
		select custom_domains_enabled
		from billing_entitlements
		where workspace_id = (
			select workspace_id
			from sites
			where id = $1
		)
	`, siteID).Scan(&result.CustomDomainsEnabled)

	rows, err := s.db.Query(ctx, `
		select id::text,
		       hostname,
		       type,
		       status,
		       coalesce(verification_token, '')
		from site_domains
		where site_id = $1
		order by
		  case when type = 'subdomain' then 0 else 1 end,
		  created_at asc
	`, siteID)
	if err != nil {
		return SiteDomainsResult{}, fmt.Errorf("list site domains: %w", err)
	}
	defer rows.Close()

	result.Domains = []DomainEntry{}
	for rows.Next() {
		var entry DomainEntry
		var verificationToken string
		if err := rows.Scan(&entry.ID, &entry.Hostname, &entry.Type, &entry.Status, &verificationToken); err != nil {
			return SiteDomainsResult{}, fmt.Errorf("scan site domain: %w", err)
		}
		if entry.Status == domainStatusActive {
			entry.PublicURL = buildPublicURL(s.appBaseURL, s.publicBaseURL, entry.Hostname)
			if entry.Type == domainTypeCustom && result.Published {
				result.PublicURL = entry.PublicURL
			}
		} else if entry.Type == domainTypeCustom && verificationToken != "" {
			entry.VerificationHostname = verificationHostname(entry.Hostname)
			entry.VerificationValue = verificationValue(verificationToken)
		}
		result.Domains = append(result.Domains, entry)
	}
	if err := rows.Err(); err != nil {
		return SiteDomainsResult{}, fmt.Errorf("iterate site domains: %w", err)
	}

	return result, nil
}

func (s *Service) Create(ctx context.Context, siteID string, hostname string) error {
	siteID = strings.TrimSpace(siteID)
	if siteID == "" {
		return ErrNotFound
	}

	normalized, err := validateCustomHostname(hostname, s.publicBaseDomain)
	if err != nil {
		return err
	}

	var existingID string
	var existingSiteID string
	var existingType string
	var existingStatus string
	err = s.db.QueryRow(ctx, `
		select id::text,
		       site_id::text,
		       type,
		       status
		from site_domains
		where lower(hostname) = lower($1)
	`, normalized).Scan(&existingID, &existingSiteID, &existingType, &existingStatus)
	switch {
	case err == nil:
		if existingSiteID != siteID {
			return ErrHostnameConflict
		}
		if existingType != domainTypeCustom {
			return ErrReservedHostname
		}
		if existingStatus == domainStatusActive {
			return nil
		}
		token, tokenErr := generateVerificationToken()
		if tokenErr != nil {
			return tokenErr
		}
		if _, execErr := s.db.Exec(ctx, `
			update site_domains
			set status = $2,
			    verification_token = $3,
			    updated_at = now()
			where id = $1
		`, existingID, domainStatusPending, token); execErr != nil {
			return fmt.Errorf("refresh site domain verification: %w", execErr)
		}
		s.invalidate(siteID, normalized)
		return nil
	case !errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("load existing hostname: %w", err)
	}

	token, err := generateVerificationToken()
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(ctx, `
		insert into site_domains (site_id, hostname, type, status, verification_token)
		values ($1, $2, $3, $4, $5)
	`, siteID, normalized, domainTypeCustom, domainStatusPending, token); err != nil {
		if isUniqueViolation(err) {
			return ErrHostnameConflict
		}
		return fmt.Errorf("create site domain: %w", err)
	}

	s.invalidate(siteID, normalized)
	return nil
}

func (s *Service) Verify(ctx context.Context, siteID string, domainID string) error {
	domainID = strings.TrimSpace(domainID)
	siteID = strings.TrimSpace(siteID)
	if siteID == "" {
		return ErrNotFound
	}
	if domainID == "" {
		return ErrDomainNotFound
	}

	var hostname string
	var domainType string
	var status string
	var token string
	if err := s.db.QueryRow(ctx, `
		select hostname,
		       type,
		       status,
		       coalesce(verification_token, '')
		from site_domains
		where id = $1
		  and site_id = $2
	`, domainID, siteID).Scan(&hostname, &domainType, &status, &token); errors.Is(err, pgx.ErrNoRows) {
		return ErrDomainNotFound
	} else if err != nil {
		return fmt.Errorf("load site domain: %w", err)
	}

	if domainType != domainTypeCustom {
		return ErrManagedDomain
	}
	if status == domainStatusActive {
		return nil
	}
	if token == "" {
		return ErrVerificationNotReady
	}

	records, err := s.lookupTXT(ctx, verificationHostname(hostname))
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return ErrVerificationNotReady
		}
		return fmt.Errorf("lookup verification record: %w", err)
	}
	expected := verificationValue(token)
	found := false
	for _, record := range records {
		if strings.TrimSpace(record) == expected {
			found = true
			break
		}
	}
	if !found {
		return ErrVerificationNotReady
	}

	if _, err := s.db.Exec(ctx, `
		update site_domains
		set status = $2,
		    updated_at = now()
		where id = $1
	`, domainID, domainStatusActive); err != nil {
		return fmt.Errorf("activate site domain: %w", err)
	}

	s.invalidate(siteID, hostname)
	return nil
}

func (s *Service) Delete(ctx context.Context, siteID string, domainID string) error {
	domainID = strings.TrimSpace(domainID)
	siteID = strings.TrimSpace(siteID)
	if siteID == "" {
		return ErrNotFound
	}
	if domainID == "" {
		return ErrDomainNotFound
	}

	var hostname string
	var domainType string
	if err := s.db.QueryRow(ctx, `
		select hostname, type
		from site_domains
		where id = $1
		  and site_id = $2
	`, domainID, siteID).Scan(&hostname, &domainType); errors.Is(err, pgx.ErrNoRows) {
		return ErrDomainNotFound
	} else if err != nil {
		return fmt.Errorf("load site domain: %w", err)
	}

	if domainType != domainTypeCustom {
		return ErrManagedDomain
	}

	if _, err := s.db.Exec(ctx, `
		delete from site_domains
		where id = $1
		  and site_id = $2
	`, domainID, siteID); err != nil {
		return fmt.Errorf("delete site domain: %w", err)
	}

	s.invalidate(siteID, hostname)
	return nil
}

func (s *Service) invalidate(siteID string, hostname string) {
	if s.cache == nil {
		return
	}
	if normalized := normalizeHostname(hostname); normalized != "" {
		s.cache.InvalidateHostname(normalized)
	}
	if siteID != "" {
		s.cache.InvalidateSite(siteID)
	}
}

func buildHostedHostname(siteSlug string, publicBaseDomain string) string {
	slug := strings.TrimSpace(siteSlug)
	baseDomain := normalizeHostname(publicBaseDomain)
	if slug == "" || baseDomain == "" {
		return ""
	}
	return normalizeHostname(slug + "." + baseDomain)
}

func buildPublicURL(appBaseURL string, publicBaseURL string, hostname string) string {
	normalizedHostname := normalizeHostname(hostname)
	if normalizedHostname == "" {
		return ""
	}

	baseURL := strings.TrimSpace(publicBaseURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(appBaseURL)
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	host := normalizedHostname
	if port := parsed.Port(); port != "" {
		host = net.JoinHostPort(normalizedHostname, port)
	}
	parsed.Host = host
	parsed.Path = "/"
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func normalizeHostname(value string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
}

func validateCustomHostname(value string, publicBaseDomain string) (string, error) {
	hostname := normalizeHostname(value)
	switch {
	case hostname == "":
		return "", fmt.Errorf("%w: hostname is required", ErrInvalidHostname)
	case strings.Contains(hostname, "://"):
		return "", fmt.Errorf("%w: hostname must not include a URL scheme", ErrInvalidHostname)
	case strings.ContainsAny(hostname, "/?#@"):
		return "", fmt.Errorf("%w: hostname must not include a path, query, or user info", ErrInvalidHostname)
	case strings.Contains(hostname, "*"):
		return "", fmt.Errorf("%w: wildcard hostnames are not supported", ErrInvalidHostname)
	case strings.Contains(hostname, ":"):
		return "", fmt.Errorf("%w: hostname must not include a port", ErrInvalidHostname)
	}

	if ip := net.ParseIP(hostname); ip != nil {
		return "", fmt.Errorf("%w: IP addresses are not supported", ErrInvalidHostname)
	}

	labels := strings.Split(hostname, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("%w: hostname must include a registrable domain", ErrInvalidHostname)
	}
	for _, label := range labels {
		if label == "" {
			return "", fmt.Errorf("%w: hostname labels must not be empty", ErrInvalidHostname)
		}
		if len(label) > 63 {
			return "", fmt.Errorf("%w: hostname labels must be 63 characters or shorter", ErrInvalidHostname)
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", fmt.Errorf("%w: hostname labels must not start or end with a hyphen", ErrInvalidHostname)
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return "", fmt.Errorf("%w: hostname labels may only contain letters, numbers, and hyphens", ErrInvalidHostname)
			}
		}
	}

	baseDomain := normalizeHostname(publicBaseDomain)
	if baseDomain != "" && (hostname == baseDomain || strings.HasSuffix(hostname, "."+baseDomain)) {
		return "", ErrReservedHostname
	}

	return hostname, nil
}

func verificationHostname(hostname string) string {
	normalized := normalizeHostname(hostname)
	if normalized == "" {
		return ""
	}
	return verificationPrefix + "." + normalized
}

func verificationValue(token string) string {
	return "snaelda-site-verification=" + strings.TrimSpace(token)
}

func generateVerificationToken() (string, error) {
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return "", fmt.Errorf("generate verification token: %w", err)
	}
	return hex.EncodeToString(token), nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
