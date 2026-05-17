package domains

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("site not found")

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Service struct {
	db               DB
	appBaseURL       string
	publicBaseURL    string
	publicBaseDomain string
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
	ID        string `json:"id"`
	Hostname  string `json:"hostname"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	PublicURL string `json:"publicUrl,omitempty"`
}

type ServiceConfig struct {
	AppBaseURL       string
	PublicBaseURL    string
	PublicBaseDomain string
}

func NewService(db DB, cfg ServiceConfig) *Service {
	return &Service{
		db:               db,
		appBaseURL:       strings.TrimRight(strings.TrimSpace(cfg.AppBaseURL), "/"),
		publicBaseURL:    strings.TrimSpace(cfg.PublicBaseURL),
		publicBaseDomain: normalizeHostname(cfg.PublicBaseDomain),
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
		       status
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
		if err := rows.Scan(&entry.ID, &entry.Hostname, &entry.Type, &entry.Status); err != nil {
			return SiteDomainsResult{}, fmt.Errorf("scan site domain: %w", err)
		}
		if entry.Status == "active" {
			entry.PublicURL = buildPublicURL(s.appBaseURL, s.publicBaseURL, entry.Hostname)
		}
		result.Domains = append(result.Domains, entry)
	}
	if err := rows.Err(); err != nil {
		return SiteDomainsResult{}, fmt.Errorf("iterate site domains: %w", err)
	}

	return result, nil
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
