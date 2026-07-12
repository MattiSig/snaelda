package respin

import (
	"context"
	"net/url"
	"sort"
	"strings"
)

// defaultMaxPages caps same-origin discovery. Re-spin fetches the given URL plus
// a small budget of additional same-origin pages (Spec 21: max 5) — never
// off-site, never a crawl.
const defaultMaxPages = 5

// SiteContent is the plain-fetch result for a source site: the root page plus
// the additional same-origin pages discovered from its navigation. FetchMode is
// always ModePlain here — the headless fallback is Spec 21 v1.1.
type SiteContent struct {
	Root      Page   `json:"root"`
	Pages     []Page `json:"pages"`
	FetchMode string `json:"fetchMode"`
}

// AllPages returns the root followed by the discovered pages.
func (s SiteContent) AllPages() []Page {
	out := make([]Page, 0, len(s.Pages)+1)
	out = append(out, s.Root)
	out = append(out, s.Pages...)
	return out
}

// FetchSite fetches the given URL and, when it yields sufficient content,
// follows up to maxPages additional same-origin links (honouring robots.txt)
// to gather richer source material. It returns ErrInsufficientContent when even
// the root page is too thin, so the caller degrades to the prompt flow.
//
// maxPages <= 0 uses the Spec 21 default of 5. It counts the additional pages,
// not the root.
func (f *Fetcher) FetchSite(ctx context.Context, rawURL string, maxPages int) (SiteContent, error) {
	if maxPages <= 0 {
		maxPages = defaultMaxPages
	}

	root, err := f.FetchPage(ctx, rawURL)
	if err != nil {
		return SiteContent{}, err
	}
	if !root.Sufficient() {
		return SiteContent{}, ErrInsufficientContent
	}

	site := SiteContent{Root: root, FetchMode: ModePlain, Pages: []Page{}}

	base, perr := url.Parse(root.FinalURL)
	if perr != nil || base == nil {
		return site, nil
	}

	robots := f.loadRobots(ctx, base)
	candidates := sameOriginCandidates(base, root.Links, robots)

	fetched := map[string]bool{canonicalKey(base): true}
	for _, candidate := range candidates {
		if len(site.Pages) >= maxPages {
			break
		}
		key := canonicalKeyString(candidate)
		if key == "" || fetched[key] {
			continue
		}
		fetched[key] = true

		page, err := f.FetchPage(ctx, candidate)
		if err != nil {
			continue // a bad sub-page never fails the whole import
		}
		if !page.Sufficient() {
			continue
		}
		site.Pages = append(site.Pages, page)
	}

	return site, nil
}

// sameOriginCandidates filters links down to fetchable same-origin pages,
// deduplicates by canonical key, drops the root, applies robots.txt disallow
// rules, and orders them by a small heuristic so the highest-value pages
// (about, services, contact) are fetched first within the page budget.
func sameOriginCandidates(base *url.URL, links []string, robots *robotsRules) []string {
	rootKey := canonicalKey(base)
	seen := map[string]bool{rootKey: true}
	type scored struct {
		url   string
		score int
	}
	var scoredLinks []scored

	for _, link := range links {
		u, err := url.Parse(link)
		if err != nil || u == nil {
			continue
		}
		if !sameOrigin(base, u) {
			continue
		}
		if !looksLikePage(u) {
			continue
		}
		if robots != nil && !robots.allowed(u.EscapedPath()) {
			continue
		}
		key := canonicalKey(u)
		if seen[key] {
			continue
		}
		seen[key] = true
		u.Fragment = ""
		scoredLinks = append(scoredLinks, scored{url: u.String(), score: pageScore(u.Path)})
	}

	sort.SliceStable(scoredLinks, func(i, j int) bool {
		return scoredLinks[i].score > scoredLinks[j].score
	})
	out := make([]string, len(scoredLinks))
	for i, s := range scoredLinks {
		out[i] = s.url
	}
	return out
}

// highValuePaths are the page slugs (Icelandic and English) most likely to hold
// re-spin-worthy content; matching pages are fetched before generic ones.
var highValuePaths = []string{
	"about", "um-okkur", "um",
	"service", "services", "thjonusta", "þjónusta", "vorur", "vörur",
	"contact", "hafa-samband", "hafdu-samband",
	"faq", "faqs", "spurningar", "algengar-spurningar",
	"price", "pricing", "verd", "verð",
	"menu", "matsedill",
}

// lowValuePaths are chrome/transactional slugs that carry no re-spin-worthy
// business content. They are explicitly down-scored so they never steal a fetch
// slot from a real content page within the small page budget (2026-07-12 QA:
// /faqs lost its slot to /cart and /privacy-policy).
var lowValuePaths = []string{
	"cart", "checkout", "basket", "karfa",
	"privacy", "personuvernd", "persónuvernd", "cookie", "cookies", "vafrakokur",
	"terms", "skilmalar", "skilmálar",
	"login", "signin", "sign-in", "innskraning", "innskráning", "account", "reikningur",
}

func pageScore(path string) int {
	p := strings.ToLower(path)
	// Down-scored chrome/transactional paths are checked first: a slug like
	// "/terms-of-service" contains the high-value "service" substring but is a
	// terms page, so the negative match must win.
	for _, key := range lowValuePaths {
		if strings.Contains(p, key) {
			// Negative so a chrome/transactional page always loses to a
			// zero-scored generic content page within the page budget.
			return -1
		}
	}
	for i, key := range highValuePaths {
		if strings.Contains(p, key) {
			// Earlier entries in the list are more valuable.
			return len(highValuePaths) - i
		}
	}
	return 0
}

// looksLikePage rejects links that point at assets or downloads rather than
// content pages, so the page budget is not spent on PDFs, images, or feeds.
func looksLikePage(u *url.URL) bool {
	path := strings.ToLower(u.Path)
	nonPageExt := []string{
		".pdf", ".jpg", ".jpeg", ".png", ".gif", ".svg", ".webp", ".ico",
		".css", ".js", ".json", ".xml", ".rss", ".zip", ".mp4", ".mp3",
		".woff", ".woff2", ".ttf", ".doc", ".docx", ".xls", ".xlsx",
	}
	for _, ext := range nonPageExt {
		if strings.HasSuffix(path, ext) {
			return false
		}
	}
	return true
}

func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

// canonicalKey identifies a page for dedup: scheme + host + path with a trailing
// slash normalized away and the query and fragment dropped.
func canonicalKey(u *url.URL) string {
	if u == nil {
		return ""
	}
	path := strings.TrimRight(u.Path, "/")
	if path == "" {
		path = "/"
	}
	return strings.ToLower(u.Scheme + "://" + u.Host + path)
}

func canonicalKeyString(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return canonicalKey(u)
}
