package publishing

import (
	"strings"
	"sync"
)

type publishedSiteCache interface {
	LoadDomain(hostname string) (publishedSiteLookup, bool)
	StoreDomain(hostname string, lookup publishedSiteLookup)
	LoadPage(siteID string, versionID string, pagePath string) (PublishedPageArtifact, bool)
	StorePage(siteID string, versionID string, pagePath string, page PublishedPageArtifact)
	InvalidateSite(siteID string)
}

type memoryPublishedSiteCache struct {
	mu            sync.RWMutex
	domainLookups map[string]publishedSiteLookup
	domainSiteIDs map[string]string
	pages         map[string]PublishedPageArtifact
}

func newMemoryPublishedSiteCache() *memoryPublishedSiteCache {
	return &memoryPublishedSiteCache{
		domainLookups: map[string]publishedSiteLookup{},
		domainSiteIDs: map[string]string{},
		pages:         map[string]PublishedPageArtifact{},
	}
}

func (c *memoryPublishedSiteCache) LoadDomain(hostname string) (publishedSiteLookup, bool) {
	if c == nil {
		return publishedSiteLookup{}, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	lookup, ok := c.domainLookups[normalizeHostname(hostname)]
	return lookup, ok
}

func (c *memoryPublishedSiteCache) StoreDomain(hostname string, lookup publishedSiteLookup) {
	if c == nil {
		return
	}

	normalized := normalizeHostname(hostname)
	if normalized == "" || lookup.Version.SiteID == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.domainLookups[normalized] = lookup
	c.domainSiteIDs[normalized] = lookup.Version.SiteID
}

func (c *memoryPublishedSiteCache) LoadPage(siteID string, versionID string, pagePath string) (PublishedPageArtifact, bool) {
	if c == nil {
		return PublishedPageArtifact{}, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	page, ok := c.pages[pageCacheKey(siteID, versionID, normalizePublishedPagePath(pagePath))]
	return page, ok
}

func (c *memoryPublishedSiteCache) StorePage(siteID string, versionID string, pagePath string, page PublishedPageArtifact) {
	if c == nil {
		return
	}
	if siteID == "" || versionID == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.pages[pageCacheKey(siteID, versionID, normalizePublishedPagePath(pagePath))] = page
}

func (c *memoryPublishedSiteCache) InvalidateSite(siteID string) {
	if c == nil || siteID == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for hostname, cachedSiteID := range c.domainSiteIDs {
		if cachedSiteID != siteID {
			continue
		}
		delete(c.domainSiteIDs, hostname)
		delete(c.domainLookups, hostname)
	}

	prefix := siteID + ":"
	for key := range c.pages {
		if strings.HasPrefix(key, prefix) {
			delete(c.pages, key)
		}
	}
}

func pageCacheKey(siteID string, versionID string, pagePath string) string {
	return siteID + ":" + versionID + ":" + normalizePublishedPagePath(pagePath)
}
