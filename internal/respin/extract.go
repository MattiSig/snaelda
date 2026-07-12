package respin

import (
	"context"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// defaultMinWords is the content-sufficiency threshold: a page whose extracted
// main text falls below it is treated as too thin to re-spin from a plain fetch,
// and the pipeline degrades to the prompt flow (Spec 21 graceful degradation).
// It is tuned to separate a near-empty client-rendered shell (a handful of
// words) from a genuinely thin but real small-business homepage (~30+ words),
// not to demand a rich page.
const defaultMinWords = 25

// PageMeta carries the head-level signals a plain HTML parse yields for free.
// Copy extraction only needs the text, but the brand-asset stage reuses the
// logo/colour hints, so the single parse captures them here rather than forcing
// a re-parse later.
type PageMeta struct {
	OGImage    string      `json:"ogImage,omitempty"`
	ThemeColor string      `json:"themeColor,omitempty"`
	IconHrefs  []string    `json:"iconHrefs,omitempty"`
	Icons      []IconRef   `json:"icons,omitempty"`
	Images     []ImageRef  `json:"images,omitempty"`
	CSSColors  []ColorHint `json:"cssColors,omitempty"`
	Lang       string      `json:"lang,omitempty"`
}

// IconRef is a <link rel="…icon…"> declaration with the rel and sizes hints the
// brand stage scores logo candidates by (apple-touch-icons and larger sizes
// make better logos than a 16px favicon).
type IconRef struct {
	Href  string `json:"href"`
	Rel   string `json:"rel,omitempty"`
	Sizes string `json:"sizes,omitempty"`
}

// Page is a fetched-and-cleaned source page: its readable main text with
// boilerplate stripped, plus the head metadata and the same-origin links the
// discovery step follows.
type Page struct {
	URL         string   `json:"url"`
	FinalURL    string   `json:"finalUrl,omitempty"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Text        string   `json:"text"`
	WordCount   int      `json:"wordCount"`
	Links       []string `json:"links,omitempty"`
	Meta        PageMeta `json:"meta"`
	// Contact holds the NAP signals harvested from the whole document (chrome
	// included), which the readability text pass strips. It is the highest
	// precision contact source a page carries.
	Contact ContactSignals `json:"contact,omitempty"`
}

// Sufficient reports whether the page carries enough readable text to extract
// from without falling back to the prompt flow.
func (p Page) Sufficient() bool {
	return p.WordCount >= defaultMinWords
}

// boilerplateTags are structural/chrome elements whose text is nav, cookie
// banners, scripts, or styling rather than business content. Their subtrees are
// skipped during text extraction (readability-style boilerplate stripping).
var boilerplateTags = map[atom.Atom]bool{
	atom.Script:   true,
	atom.Style:    true,
	atom.Noscript: true,
	atom.Template: true,
	atom.Svg:      true,
	atom.Nav:      true,
	atom.Header:   true,
	atom.Footer:   true,
	atom.Aside:    true,
	atom.Form:     true,
	atom.Button:   true,
	atom.Iframe:   true,
}

// FetchPage fetches a URL through the SSRF-guarded client and extracts its
// readable content. A non-HTML or error response yields an error the caller can
// treat as a degradation signal.
func (f *Fetcher) FetchPage(ctx context.Context, rawURL string) (Page, error) {
	res, err := f.Fetch(ctx, rawURL, FetchOptions{
		MaxBytes: defaultMaxHTMLBytes,
		Accept:   "text/html,application/xhtml+xml",
	})
	if err != nil {
		return Page{}, err
	}
	if res.StatusCode >= 400 {
		return Page{}, &FetchStatusError{StatusCode: res.StatusCode, URL: res.FinalURL}
	}
	if !isHTMLContentType(res.ContentType) {
		return Page{}, &ContentTypeError{ContentType: res.ContentType, URL: res.FinalURL}
	}

	base, perr := url.Parse(res.FinalURL)
	if perr != nil {
		base, _ = normalizeFetchURL(rawURL)
	}
	page := extractPage(res.Body, base)
	page.URL = rawURL
	page.FinalURL = res.FinalURL
	return page, nil
}

// extractPage parses an HTML document and pulls out title, meta description,
// head-level brand hints, cleaned main text, and same-origin links resolved
// against base.
func extractPage(body []byte, base *url.URL) Page {
	page := Page{Links: []string{}}
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return page
	}

	seenLinks := map[string]bool{}
	var text strings.Builder
	// contentRoot prefers a <main>/<article> subtree when present; otherwise the
	// whole document body is walked with boilerplate skipped.
	contentRoot := findContentRoot(doc)

	var walkMeta func(*html.Node)
	walkMeta = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Html:
				if v := attr(n, "lang"); v != "" && page.Meta.Lang == "" {
					page.Meta.Lang = strings.TrimSpace(v)
				}
			case atom.Title:
				if page.Title == "" {
					page.Title = strings.TrimSpace(collectText(n))
				}
			case atom.Meta:
				readMeta(n, base, &page)
			case atom.Link:
				readLink(n, base, &page)
			case atom.A:
				if href := resolveLink(base, attr(n, "href")); href != "" && !seenLinks[href] {
					seenLinks[href] = true
					page.Links = append(page.Links, href)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkMeta(c)
		}
	}
	walkMeta(doc)

	extractText(contentRoot, &text)
	page.Text = normalizeWhitespace(text.String())
	page.WordCount = len(strings.Fields(page.Text))

	// The brand-asset stage needs images and declared colours that the copy walk
	// above deliberately discards (logos live in the <header> chrome the text
	// pass skips). Gather them from the whole document in one extra pass.
	collectBrandHints(doc, base, &page)

	// Harvest NAP contact signals from the whole document — including the
	// header/footer/nav chrome the copy pass strips, where the phone, email, and
	// address usually live on a small-business site.
	page.Contact = harvestContactSignals(doc, base)
	return page
}

// findContentRoot returns the best subtree to read business copy from: the first
// <main> or <article> element if the document has one, otherwise the document
// root. Preferring these landmark elements is a lightweight readability signal
// that drops most surrounding chrome before the tag-level filter even runs.
func findContentRoot(doc *html.Node) *html.Node {
	var main, article *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Main:
				if main == nil {
					main = n
				}
			case atom.Article:
				if article == nil {
					article = n
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if main != nil {
		return main
	}
	if article != nil {
		return article
	}
	return doc
}

// extractText appends the visible text of n's subtree to out, skipping
// boilerplate and hidden elements and inserting soft breaks at block edges.
func extractText(n *html.Node, out *strings.Builder) {
	if n == nil {
		return
	}
	if n.Type == html.ElementNode {
		if boilerplateTags[n.DataAtom] {
			return
		}
		if isHidden(n) {
			return
		}
	}
	if n.Type == html.TextNode {
		if t := strings.TrimSpace(n.Data); t != "" {
			out.WriteString(n.Data)
			out.WriteByte(' ')
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractText(c, out)
	}
	if n.Type == html.ElementNode && isBlockElement(n.DataAtom) {
		out.WriteByte('\n')
	}
}

func readMeta(n *html.Node, base *url.URL, page *Page) {
	name := strings.ToLower(strings.TrimSpace(attr(n, "name")))
	prop := strings.ToLower(strings.TrimSpace(attr(n, "property")))
	content := strings.TrimSpace(attr(n, "content"))
	if content == "" {
		return
	}
	switch {
	case name == "description" && page.Description == "":
		page.Description = content
	case prop == "og:description" && page.Description == "":
		page.Description = content
	case prop == "og:image" && page.Meta.OGImage == "":
		// Resolve to an absolute URL so the brand-asset stage can ingest it
		// directly through the SSRF client.
		if resolved := resolveLink(base, content); resolved != "" {
			page.Meta.OGImage = resolved
		}
	case name == "theme-color" && page.Meta.ThemeColor == "":
		page.Meta.ThemeColor = content
	}
}

func readLink(n *html.Node, base *url.URL, page *Page) {
	rel := strings.ToLower(strings.TrimSpace(attr(n, "rel")))
	if !strings.Contains(rel, "icon") {
		return
	}
	href := resolveLink(base, attr(n, "href"))
	if href == "" {
		return
	}
	page.Meta.IconHrefs = append(page.Meta.IconHrefs, href)
	page.Meta.Icons = append(page.Meta.Icons, IconRef{
		Href:  href,
		Rel:   rel,
		Sizes: strings.ToLower(strings.TrimSpace(attr(n, "sizes"))),
	})
}

// resolveLink resolves href against base and returns an absolute http(s) URL
// with the fragment stripped, or "" if it is empty, non-absolute-resolvable, or
// a non-web scheme (mailto:, tel:, javascript:, data:).
func resolveLink(base *url.URL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") {
		return ""
	}
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	var abs *url.URL
	if base != nil {
		abs = base.ResolveReference(ref)
	} else {
		abs = ref
	}
	if abs.Scheme != "http" && abs.Scheme != "https" {
		return ""
	}
	abs.Fragment = ""
	return abs.String()
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func collectText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// isHidden reports whether an element is explicitly hidden (hidden attribute,
// aria-hidden, or an inline display:none / visibility:hidden style). It is a
// cheap best-effort filter, not a full CSS engine.
func isHidden(n *html.Node) bool {
	for _, a := range n.Attr {
		switch strings.ToLower(a.Key) {
		case "hidden":
			return true
		case "aria-hidden":
			if strings.EqualFold(strings.TrimSpace(a.Val), "true") {
				return true
			}
		case "style":
			s := strings.ToLower(a.Val)
			if strings.Contains(s, "display:none") || strings.Contains(s, "display: none") ||
				strings.Contains(s, "visibility:hidden") || strings.Contains(s, "visibility: hidden") {
				return true
			}
		}
	}
	return false
}

func isBlockElement(a atom.Atom) bool {
	switch a {
	case atom.P, atom.Div, atom.Section, atom.Li, atom.Br, atom.H1, atom.H2,
		atom.H3, atom.H4, atom.H5, atom.H6, atom.Tr, atom.Blockquote, atom.Article,
		atom.Main, atom.Ul, atom.Ol, atom.Table, atom.Dd, atom.Dt:
		return true
	}
	return false
}

// normalizeWhitespace collapses runs of intra-line whitespace to single spaces
// while preserving paragraph breaks, so the cleaned text reads as coherent
// blocks rather than one wrapped wall or a ragged token list.
func normalizeWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		if f := strings.Join(strings.Fields(line), " "); f != "" {
			cleaned = append(cleaned, f)
		}
	}
	return strings.Join(cleaned, "\n")
}

func isHTMLContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if ct == "" {
		return true // servers that omit Content-Type; parser tolerates non-HTML
	}
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml")
}
