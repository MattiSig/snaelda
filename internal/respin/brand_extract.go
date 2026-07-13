package respin

import (
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// ImageRef is an <img> found on the page, carried with the context signals the
// brand-asset stage scores logos and hero photos by: the structural region it
// sits in, its class/id hint text, and any declared dimensions.
type ImageRef struct {
	URL    string `json:"url"`
	Alt    string `json:"alt,omitempty"`
	Region string `json:"region,omitempty"` // header | nav | footer | content
	Hint   string `json:"hint,omitempty"`   // lowercased class+id, for logo/hero matching
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// collectBrandHints walks the parsed document once more to gather the brand
// signals the copy-text pass discards: every <img> tagged with the structural
// region it appears in (a header <img> is a logo signal; a large content <img>
// is a hero signal), and the colour values declared in <style> blocks and
// inline style attributes. It is deliberately separate from the copy walk,
// which skips <header>/<nav>/<footer> — exactly where logos live.
func collectBrandHints(doc *html.Node, base *url.URL, page *Page) {
	var styleText strings.Builder

	var walk func(n *html.Node, region string)
	walk = func(n *html.Node, region string) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Header:
				region = "header"
			case atom.Footer:
				region = "footer"
			case atom.Nav:
				if region == "" || region == "content" {
					region = "nav"
				}
			case atom.Main, atom.Article, atom.Section:
				if region == "" {
					region = "content"
				}
			case atom.Img:
				if ref, ok := imageRefFromNode(n, base, region); ok {
					page.Meta.Images = append(page.Meta.Images, ref)
				}
			case atom.Style:
				styleText.WriteString(collectText(n))
				styleText.WriteByte('\n')
			}
			// Inline colour declarations (style="--brand:#…" or "color:#…").
			if s := attr(n, "style"); s != "" {
				styleText.WriteString(s)
				styleText.WriteByte('\n')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, region)
		}
	}
	walk(doc, "")

	// Cap the retained inline CSS so a page that inlines megabytes of critical
	// CSS cannot balloon the in-memory page. The brand palette lives in a handful
	// of custom-property declarations, well within this bound.
	css := styleText.String()
	if len(css) > maxInlineCSSBytes {
		css = css[:maxInlineCSSBytes]
	}
	page.Meta.StyleText = css
	page.Meta.CSSColors = extractCSSColors(css)
}

// maxInlineCSSBytes caps the inline CSS text retained per page for brand-colour
// resolution.
const maxInlineCSSBytes = 1 << 20 // 1 MB

// imageRefFromNode builds an ImageRef from an <img> node, resolving its source
// against base. It returns ok=false for images with no resolvable http(s)
// source (data: URIs, empty src, tracking beacons with non-web schemes).
func imageRefFromNode(n *html.Node, base *url.URL, region string) (ImageRef, bool) {
	src := firstNonEmptyString(attr(n, "src"), attr(n, "data-src"))
	resolved := resolveLink(base, src)
	if resolved == "" {
		return ImageRef{}, false
	}
	if region == "" {
		region = "content"
	}
	hint := strings.ToLower(strings.TrimSpace(attr(n, "class") + " " + attr(n, "id")))
	return ImageRef{
		URL:    resolved,
		Alt:    strings.TrimSpace(attr(n, "alt")),
		Region: region,
		Hint:   strings.Join(strings.Fields(hint), " "),
		Width:  parseDimAttr(attr(n, "width")),
		Height: parseDimAttr(attr(n, "height")),
	}, true
}

// parseDimAttr reads an HTML width/height attribute as a pixel count, tolerating
// a trailing "px" and ignoring percentages or other non-integer forms.
func parseDimAttr(value string) int {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimSuffix(value, "px")
	if value == "" {
		return 0
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
