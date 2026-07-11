package respin

import (
	"net/url"
	"sort"
	"strings"
)

// trackingParams are query keys stripped during URL normalization so that the
// same page shared with different campaign tags collapses to one cache key
// (Spec 21: repeated pastes of the same URL — a link travelling a Facebook
// group — must hit the cache).
var trackingParams = map[string]bool{
	"utm_source":   true,
	"utm_medium":   true,
	"utm_campaign": true,
	"utm_term":     true,
	"utm_content":  true,
	"utm_id":       true,
	"gclid":        true,
	"fbclid":       true,
	"msclkid":      true,
	"mc_cid":       true,
	"mc_eid":       true,
	"igshid":       true,
	"ref":          true,
	"ref_src":      true,
	"_hsenc":       true,
	"_hsmi":        true,
}

// NormalizeURL canonicalizes a pasted URL for use as the cache key: it defaults
// a bare host to https, lowercases the scheme and host, drops the fragment,
// strips a default port, removes tracking query params, sorts the remaining
// query, and trims a trailing slash on non-root paths. It rejects non-web
// schemes and credentials-in-URL up front so intake fails fast before any fetch.
func NormalizeURL(raw string) (normalized string, err error) {
	u, err := normalizeFetchURL(raw)
	if err != nil {
		return "", err
	}
	if err := validateURL(u, false); err != nil {
		return "", err
	}

	u.Host = strings.ToLower(u.Host)
	// Drop a redundant default port.
	if (u.Scheme == "http" && u.Port() == "80") || (u.Scheme == "https" && u.Port() == "443") {
		u.Host = u.Hostname()
	}

	// Strip tracking params; keep and sort the rest for a stable key.
	if u.RawQuery != "" {
		q := u.Query()
		for key := range q {
			if trackingParams[strings.ToLower(key)] {
				q.Del(key)
			}
		}
		if len(q) == 0 {
			u.RawQuery = ""
		} else {
			u.RawQuery = encodeSortedQuery(q)
		}
	}

	// Normalize the path: a trailing slash on a non-root path is cosmetic.
	if u.Path == "" {
		u.Path = "/"
	} else if u.Path != "/" {
		u.Path = strings.TrimRight(u.Path, "/")
		if u.Path == "" {
			u.Path = "/"
		}
	}

	if strings.TrimSpace(u.Hostname()) == "" {
		return "", ErrEmptyURL
	}
	return u.String(), nil
}

// encodeSortedQuery encodes query values with keys sorted so equivalent URLs
// with reordered params produce identical cache keys.
func encodeSortedQuery(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		values := q[k]
		sort.Strings(values)
		for _, v := range values {
			if b.Len() > 0 {
				b.WriteByte('&')
			}
			b.WriteString(url.QueryEscape(k))
			b.WriteByte('=')
			b.WriteString(url.QueryEscape(v))
		}
	}
	return b.String()
}
