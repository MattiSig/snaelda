package respin

import (
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// ContactSignals are the structured Name/Address/Phone (NAP) facts harvested
// from a whole source document — including the <header>/<footer>/<nav> chrome
// the readability copy pass deliberately skips. On most small-business sites the
// phone, email, and street address live only in that chrome (2026-07-12 QA: a
// 24/7 emergency plumber shipped with no way to call them because its phone was
// only in a tel: link in the header and footer). These signals are the highest
// precision contact facts a page carries and are fed to the extraction stage as
// candidate facts, then used to deterministically backfill the extracted contact
// when the model still misses them.
type ContactSignals struct {
	// Phones come from tel: hrefs, JSON-LD/microdata telephone properties, and —
	// lowest confidence — phone-shaped text in header/footer chrome.
	Phones []string `json:"phones,omitempty"`
	// Emails come from mailto: hrefs and JSON-LD/microdata email properties.
	Emails []string `json:"emails,omitempty"`
	// Addresses come from JSON-LD PostalAddress, address microdata, and
	// <address> elements.
	Addresses []string `json:"addresses,omitempty"`
}

// IsEmpty reports whether no contact signal was harvested.
func (s ContactSignals) IsEmpty() bool {
	return len(s.Phones) == 0 && len(s.Emails) == 0 && len(s.Addresses) == 0
}

// merge folds another set of signals into this one, deduplicating case- and
// whitespace-insensitively while preserving first-seen order (the source's own
// primary contact usually appears first).
func (s ContactSignals) merge(other ContactSignals) ContactSignals {
	return ContactSignals{
		Phones:    dedupeAppend(s.Phones, other.Phones),
		Emails:    dedupeAppend(s.Emails, other.Emails),
		Addresses: dedupeAppend(s.Addresses, other.Addresses),
	}
}

// phoneShapedText matches a plausible phone number: an optional leading +, then
// 7–20 characters of digits and common separators. It is intentionally loose;
// the digit-count guard in cleanPhone rejects the false positives (prices,
// dates, long id strings) it lets through.
var phoneShapedText = regexp.MustCompile(`\+?[0-9][0-9()\-.\s/]{5,18}[0-9]`)

// harvestContactSignals walks the whole parsed document (chrome included) and
// collects NAP facts in descending precision: tel:/mailto: hrefs and
// JSON-LD/microdata properties first, then phone-shaped text found only in the
// header/footer/nav regions the copy pass strips. Boilerplate stripping applies
// to prose, not to these structured contact signals.
func harvestContactSignals(doc *html.Node, base *url.URL) ContactSignals {
	var signals ContactSignals
	var chrome strings.Builder

	var walk func(n *html.Node, inChrome bool)
	walk = func(n *html.Node, inChrome bool) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Header, atom.Footer, atom.Nav:
				inChrome = true
			case atom.A:
				collectContactHref(attr(n, "href"), &signals)
			case atom.Script:
				if strings.Contains(strings.ToLower(attr(n, "type")), "ld+json") {
					collectJSONLD(collectText(n), &signals)
				}
			case atom.Address:
				if addr := squashSpaces(collectText(n)); addr != "" {
					signals.Addresses = append(signals.Addresses, addr)
				}
			}
			// Schema.org microdata / RDFa itemprop values are high precision.
			collectMicrodata(n, &signals)
		}
		if inChrome && n.Type == html.TextNode {
			chrome.WriteString(n.Data)
			chrome.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, inChrome)
		}
	}
	walk(doc, false)

	// Phone-shaped text in the chrome is the lowest-confidence signal; only mine
	// it when no higher-precision phone (tel:/microdata/JSON-LD) was found, so it
	// never overrides a real number and its false positives cannot win.
	if len(signals.Phones) == 0 {
		for _, m := range phoneShapedText.FindAllString(chrome.String(), -1) {
			if p := cleanPhone(m); p != "" {
				signals.Phones = append(signals.Phones, p)
			}
		}
	}

	signals.Phones = dedupeAppend(nil, signals.Phones)
	signals.Emails = dedupeAppend(nil, signals.Emails)
	signals.Addresses = dedupeAppend(nil, signals.Addresses)
	return signals
}

// collectContactHref pulls a phone or email out of a tel:/mailto: href — the
// highest-precision contact signals a page has, and exactly the ones resolveLink
// discards for navigation.
func collectContactHref(href string, signals *ContactSignals) {
	href = strings.TrimSpace(href)
	lower := strings.ToLower(href)
	switch {
	case strings.HasPrefix(lower, "tel:"):
		if p := cleanPhone(href[len("tel:"):]); p != "" {
			signals.Phones = append(signals.Phones, p)
		}
	case strings.HasPrefix(lower, "mailto:"):
		if e := cleanEmail(href[len("mailto:"):]); e != "" {
			signals.Emails = append(signals.Emails, e)
		}
	}
}

// microdataProps maps schema.org itemprop names to the signal they feed.
var microdataProps = map[string]string{
	"telephone": "phone",
	"phone":     "phone",
	"email":     "email",
}

// collectMicrodata reads schema.org itemprop values (telephone/email) off an
// element, preferring an explicit tel:/mailto: href on the same node when present.
func collectMicrodata(n *html.Node, signals *ContactSignals) {
	prop := strings.ToLower(strings.TrimSpace(attr(n, "itemprop")))
	if prop == "" {
		return
	}
	kind := microdataProps[prop]
	if kind == "" {
		return
	}
	value := firstNonEmptyString(attr(n, "content"), squashSpaces(collectText(n)))
	switch kind {
	case "phone":
		if p := cleanPhone(value); p != "" {
			signals.Phones = append(signals.Phones, p)
		}
	case "email":
		if e := cleanEmail(value); e != "" {
			signals.Emails = append(signals.Emails, e)
		}
	}
}

// collectJSONLD parses a JSON-LD block and pulls telephone/email/address out of
// it, recursively (a LocalBusiness graph nests the address inside the entity).
func collectJSONLD(raw string, signals *ContactSignals) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return
	}
	walkJSONLD(parsed, signals)
}

func walkJSONLD(node any, signals *ContactSignals) {
	switch v := node.(type) {
	case map[string]any:
		for key, child := range v {
			switch strings.ToLower(key) {
			case "telephone", "phone":
				if p := cleanPhone(jsonString(child)); p != "" {
					signals.Phones = append(signals.Phones, p)
				}
			case "email":
				if e := cleanEmail(jsonString(child)); e != "" {
					signals.Emails = append(signals.Emails, e)
				}
			case "address":
				if addr := jsonAddress(child); addr != "" {
					signals.Addresses = append(signals.Addresses, addr)
				}
			}
			walkJSONLD(child, signals)
		}
	case []any:
		for _, child := range v {
			walkJSONLD(child, signals)
		}
	}
}

// jsonAddress renders a JSON-LD address, which may be a plain string or a
// PostalAddress object, into a single readable line.
func jsonAddress(node any) string {
	switch v := node.(type) {
	case string:
		return squashSpaces(v)
	case map[string]any:
		parts := []string{
			jsonString(v["streetAddress"]),
			jsonString(v["postalCode"]),
			jsonString(v["addressLocality"]),
			jsonString(v["addressRegion"]),
			jsonString(v["addressCountry"]),
		}
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = squashSpaces(p); p != "" {
				out = append(out, p)
			}
		}
		return strings.Join(out, ", ")
	}
	return ""
}

// jsonString coerces a JSON-LD scalar to a trimmed string (some producers wrap
// values in {"@value": "…"} or arrays).
func jsonString(node any) string {
	switch v := node.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]any:
		return jsonString(v["@value"])
	case []any:
		if len(v) > 0 {
			return jsonString(v[0])
		}
	}
	return ""
}

// cleanPhone trims a phone candidate, strips a leading tel: query/params, and
// rejects anything without a plausible digit count (7–15), which filters the
// prices/dates/ids the loose regex and stray tel: values let through.
func cleanPhone(value string) string {
	value = strings.TrimSpace(value)
	if idx := strings.IndexAny(value, "?;"); idx >= 0 {
		value = value[:idx]
	}
	value = strings.TrimSpace(value)
	digits := 0
	for _, r := range value {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	if digits < 7 || digits > 15 {
		return ""
	}
	return value
}

// cleanEmail trims a mailto: value, strips any ?subject= query, and does a
// minimal shape check (one @, a dot in the domain).
func cleanEmail(value string) string {
	value = strings.TrimSpace(value)
	if idx := strings.IndexByte(value, '?'); idx >= 0 {
		value = value[:idx]
	}
	value = strings.TrimSpace(value)
	at := strings.IndexByte(value, '@')
	if at <= 0 || at == len(value)-1 {
		return ""
	}
	if !strings.Contains(value[at+1:], ".") {
		return ""
	}
	return value
}

// squashSpaces collapses all runs of whitespace (including newlines) to single
// spaces and trims, so a multi-line <address> reads as one line.
func squashSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// dedupeAppend appends src onto dst, dropping empties and case/space-insensitive
// duplicates while preserving first-seen order.
func dedupeAppend(dst, src []string) []string {
	seen := map[string]bool{}
	for _, v := range dst {
		seen[strings.ToLower(strings.TrimSpace(v))] = true
	}
	for _, v := range src {
		key := strings.ToLower(strings.TrimSpace(v))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		dst = append(dst, strings.TrimSpace(v))
	}
	return dst
}

// sortedForPrompt returns a stable, readable rendering of harvested signals for
// the extraction prompt's candidate-facts block.
func (s ContactSignals) sortedForPrompt() string {
	var b strings.Builder
	writeList := func(label string, values []string) {
		if len(values) == 0 {
			return
		}
		vs := append([]string(nil), values...)
		sort.Strings(vs)
		for _, v := range vs {
			b.WriteString("\n- ")
			b.WriteString(label)
			b.WriteString(": ")
			b.WriteString(v)
		}
	}
	writeList("phone", s.Phones)
	writeList("email", s.Emails)
	writeList("address", s.Addresses)
	return strings.TrimSpace(b.String())
}
