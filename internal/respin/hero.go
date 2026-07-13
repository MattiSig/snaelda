package respin

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// SourceHero is the source site's above-the-fold hero, extracted deterministically
// from the DOM (never by an LLM). It carries the source's own headline register,
// supporting line, CTA intent, and background image so the generation home page
// can match the source's energy instead of inventing generic category copy
// (Spec 21 step 7, Spec 07 optionalHints.sourceHero). Every field is optional; a
// missing hero simply omits the hint.
type SourceHero struct {
	Headline    string `json:"headline,omitempty"`
	Subheadline string `json:"subheadline,omitempty"`
	CTALabel    string `json:"ctaLabel,omitempty"`
	// ImageURL is the resolved absolute source URL of the hero's background image
	// (an <img> in the hero section or a CSS background-image). It is the seam the
	// brand pull maps to an ingested asset id; it is not part of the generation
	// contract itself.
	ImageURL string `json:"imageUrl,omitempty"`
	// ImageAssetID is the ingested asset id for ImageURL, filled by the composer
	// after the brand pull when the hero image was pulled. It is what the
	// generation input's optionalHints.sourceHero carries.
	ImageAssetID string `json:"imageAssetId,omitempty"`
	// TextOnly is true when the source hero carried no background image — a
	// type-led hero the layout stage can match with the statement hero variant
	// (Spec 04) rather than an image-led one.
	TextOnly bool `json:"textOnly"`
}

// IsEmpty reports whether the hero carries no usable signal.
func (h SourceHero) IsEmpty() bool {
	return strings.TrimSpace(h.Headline) == "" &&
		strings.TrimSpace(h.Subheadline) == "" &&
		strings.TrimSpace(h.CTALabel) == "" &&
		strings.TrimSpace(h.ImageURL) == ""
}

// heroSectionKeywords mark a container element (by class/id) as the hero region
// even when it is not the ancestor of the first <h1> — site builders wrap the
// opener in a named section (Squarespace's section-background, hero banners).
var heroSectionKeywords = []string{
	"hero", "banner", "masthead", "jumbotron", "showcase",
	"section-background", "intro-section", "page-header", "site-header",
}

// ctaExcludedLabels are link/button texts that read as chrome, not a hero CTA.
var ctaExcludedLabels = map[string]bool{
	"home": true, "menu": true, "skip to content": true, "search": true,
	"cart": true, "login": true, "log in": true, "sign in": true,
	"heim": true, "valmynd": true, "leita": true, "karfa": true, "innskráning": true,
}

// maxHeroTextLen bounds how long a candidate subheadline may be before it is
// treated as body copy rather than a hero supporting line.
const maxHeroTextLen = 240

// extractSourceHero locates the source hero — the first section-like container
// holding the page's <h1>, or the first element tagged with a hero class — and
// reads its headline, supporting line, CTA label, and background image straight
// from the DOM. It is a best-effort structural read: a page with no <h1> and no
// hero-named container yields an empty hero, which simply omits the hint.
func extractSourceHero(doc *html.Node, base *url.URL) SourceHero {
	root := findHeroRoot(doc)
	if root == nil {
		return SourceHero{}
	}

	var hero SourceHero
	hero.Headline = firstHeadingText(root)

	if sub := firstSupportingLine(root, hero.Headline); sub != "" {
		hero.Subheadline = sub
	}
	if cta := firstCTALabel(root); cta != "" {
		hero.CTALabel = cta
	}
	if img := firstHeroImage(root, base); img != "" {
		hero.ImageURL = img
	}
	hero.TextOnly = hero.ImageURL == ""

	if hero.IsEmpty() {
		return SourceHero{}
	}
	return hero
}

// findHeroRoot returns the hero container: the first element whose class/id names
// it a hero, otherwise the nearest section/header/main ancestor of the first
// <h1> (its parent when the <h1> has no such ancestor).
func findHeroRoot(doc *html.Node) *html.Node {
	if named := firstElementByHint(doc, heroSectionKeywords); named != nil {
		return named
	}
	h1 := firstElementWithAtom(doc, atom.H1)
	if h1 == nil {
		return nil
	}
	for n := h1.Parent; n != nil; n = n.Parent {
		if n.Type != html.ElementNode {
			continue
		}
		switch n.DataAtom {
		case atom.Section, atom.Header, atom.Main, atom.Article:
			return n
		}
	}
	if h1.Parent != nil {
		return h1.Parent
	}
	return h1
}

// firstElementByHint returns the first element whose lowercased class+id contains
// any of the keywords. It skips the <body>/<html> shell so a body class of
// "home" (matched by "home"-adjacent hints elsewhere) cannot swallow the page.
func firstElementByHint(root *html.Node, keywords []string) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.DataAtom != atom.Body && n.DataAtom != atom.Html {
			hint := strings.ToLower(attr(n, "class") + " " + attr(n, "id"))
			if containsAny(hint, keywords) {
				found = n
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

// firstElementWithAtom returns the first element with the given tag in document
// order.
func firstElementWithAtom(root *html.Node, a atom.Atom) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.DataAtom == a {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

// firstHeadingText returns the cleaned text of the first h1 (falling back to the
// first h2) inside root.
func firstHeadingText(root *html.Node) string {
	for _, a := range []atom.Atom{atom.H1, atom.H2} {
		if n := firstElementWithAtom(root, a); n != nil {
			if t := cleanNodeText(n); t != "" {
				return t
			}
		}
	}
	return ""
}

// firstSupportingLine returns the first paragraph or subordinate heading inside
// root that reads as a hero supporting line: non-empty, distinct from the
// headline, and short enough not to be body copy.
func firstSupportingLine(root *html.Node, headline string) string {
	headlineKey := strings.ToLower(strings.TrimSpace(headline))
	var found string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != "" {
			return
		}
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.P, atom.H2, atom.H3, atom.H4:
				t := cleanNodeText(n)
				if t != "" && len(t) <= maxHeroTextLen && strings.ToLower(t) != headlineKey {
					found = t
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

// firstCTALabel returns the label of the first link or button inside root that
// reads as a hero call to action (short, non-empty, not obvious chrome).
func firstCTALabel(root *html.Node) string {
	var found string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != "" {
			return
		}
		if n.Type == html.ElementNode && (n.DataAtom == atom.A || n.DataAtom == atom.Button) {
			label := cleanNodeText(n)
			key := strings.ToLower(label)
			if label != "" && len(label) <= 40 && !ctaExcludedLabels[key] {
				found = label
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

// firstHeroImage returns the resolved source URL of the hero's background image:
// the first <img> in root, else the first CSS background-image declared on an
// element's style attribute in root. Returns "" for a text-only hero.
func firstHeroImage(root *html.Node, base *url.URL) string {
	if img := firstElementWithAtom(root, atom.Img); img != nil {
		if resolved := resolveLink(base, firstNonEmptyString(attr(img, "src"), attr(img, "data-src"))); resolved != "" {
			return resolved
		}
	}
	var found string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != "" {
			return
		}
		if n.Type == html.ElementNode {
			if bg := cssBackgroundURL(attr(n, "style")); bg != "" {
				if resolved := resolveLink(base, bg); resolved != "" {
					found = resolved
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

// cssBackgroundURL extracts the url(...) target of a background / background-image
// declaration from an inline style string, or "" when there is none.
func cssBackgroundURL(style string) string {
	lower := strings.ToLower(style)
	idx := strings.Index(lower, "background")
	if idx < 0 {
		return ""
	}
	open := strings.Index(lower[idx:], "url(")
	if open < 0 {
		return ""
	}
	rest := style[idx+open+len("url("):]
	close := strings.IndexByte(rest, ')')
	if close < 0 {
		return ""
	}
	raw := strings.TrimSpace(rest[:close])
	raw = strings.Trim(raw, `"'`)
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(strings.ToLower(raw), "data:") {
		return ""
	}
	return raw
}

// cleanNodeText collects a node's text and collapses whitespace to a single
// coherent line.
func cleanNodeText(n *html.Node) string {
	return strings.TrimSpace(strings.Join(strings.Fields(collectText(n)), " "))
}

// markHeroImage tags the ImageRef matching the hero's background image URL with
// the "hero" region so the brand stage scores it above the og:image social card
// (the actual hero photograph, not the logo lockup). When the hero image was a
// CSS background with no <img> element, a synthetic hero-region ref is appended
// so the brand stage can still pull it.
func markHeroImage(page *Page, heroURL string) {
	if heroURL == "" {
		return
	}
	for i := range page.Meta.Images {
		if page.Meta.Images[i].URL == heroURL {
			page.Meta.Images[i].Region = "hero"
			return
		}
	}
	page.Meta.Images = append(page.Meta.Images, ImageRef{URL: heroURL, Region: "hero"})
}
