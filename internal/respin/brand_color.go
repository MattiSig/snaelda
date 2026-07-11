package respin

import (
	"fmt"
	"image"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ColorHint is a colour value pulled from CSS, ranked by how likely its
// declaration names the brand's primary colour. The brand stage prefers the
// highest-scoring hint, so a `--primary` beats a stray `--border-grey`.
type ColorHint struct {
	Value string `json:"value"` // normalized #rrggbb
	Score int    `json:"score"`
}

// customPropPattern matches a CSS custom-property declaration: `--name: value`.
// The value runs to the next `;` or block close.
var customPropPattern = regexp.MustCompile(`--([a-zA-Z0-9-]+)\s*:\s*([^;}]+)`)

// hexColorPattern matches a 3- or 6-digit hex colour literal.
var hexColorPattern = regexp.MustCompile(`#([0-9a-fA-F]{6}|[0-9a-fA-F]{3})\b`)

// rgbColorPattern matches an rgb()/rgba() functional colour.
var rgbColorPattern = regexp.MustCompile(`rgba?\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})`)

// brandColorKeywords score a custom-property name by how strongly it signals the
// brand's primary colour. Longer, more specific matches are checked first.
var brandColorKeywords = []struct {
	keyword string
	score   int
}{
	{"primary", 100},
	{"brand", 95},
	{"accent", 80},
	{"theme", 70},
	{"main", 60},
	{"secondary", 40},
	{"color-1", 55},
	{"color1", 55},
}

// extractCSSColors parses custom-property colour declarations out of collected
// CSS text and returns them ranked, most-brand-like first, deduplicated by
// value. Only custom properties are considered — ordinary `color:`/`background:`
// rules are too noisy to attribute to the brand.
func extractCSSColors(css string) []ColorHint {
	if strings.TrimSpace(css) == "" {
		return nil
	}
	best := map[string]int{}
	for _, m := range customPropPattern.FindAllStringSubmatch(css, -1) {
		name := strings.ToLower(m[1])
		hex, ok := parseCSSColor(m[2])
		if !ok {
			continue
		}
		score := scoreColorName(name)
		if score <= 0 {
			continue
		}
		if existing, seen := best[hex]; !seen || score > existing {
			best[hex] = score
		}
	}
	if len(best) == 0 {
		return nil
	}
	hints := make([]ColorHint, 0, len(best))
	for hex, score := range best {
		hints = append(hints, ColorHint{Value: hex, Score: score})
	}
	sort.SliceStable(hints, func(i, j int) bool {
		if hints[i].Score != hints[j].Score {
			return hints[i].Score > hints[j].Score
		}
		return hints[i].Value < hints[j].Value
	})
	return hints
}

func scoreColorName(name string) int {
	for _, kw := range brandColorKeywords {
		if strings.Contains(name, kw.keyword) {
			return kw.score
		}
	}
	// A named custom property that at least holds a colour is a weak signal.
	if strings.Contains(name, "color") || strings.Contains(name, "colour") {
		return 10
	}
	return 0
}

// parseCSSColor extracts the first hex or rgb() colour from a CSS value and
// normalizes it to `#rrggbb`. It ignores values that reference another variable
// (var(--x)) or a named colour, which carry no usable literal.
func parseCSSColor(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if m := hexColorPattern.FindString(value); m != "" {
		return normalizeHex(m)
	}
	if m := rgbColorPattern.FindStringSubmatch(value); m != nil {
		r, _ := strconv.Atoi(m[1])
		g, _ := strconv.Atoi(m[2])
		b, _ := strconv.Atoi(m[3])
		if r > 255 || g > 255 || b > 255 {
			return "", false
		}
		return fmt.Sprintf("#%02x%02x%02x", r, g, b), true
	}
	return "", false
}

// normalizeHex lowercases a hex colour and expands the 3-digit shorthand to the
// canonical 6-digit `#rrggbb` form.
func normalizeHex(hex string) (string, bool) {
	hex = strings.ToLower(strings.TrimSpace(hex))
	hex = strings.TrimPrefix(hex, "#")
	switch len(hex) {
	case 3:
		return fmt.Sprintf("#%c%c%c%c%c%c", hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]), true
	case 6:
		return "#" + hex, true
	default:
		return "", false
	}
}

// dominantColor derives a representative brand colour from a decoded logo image:
// the most frequent vivid (saturated, mid-brightness) colour, ignoring the
// transparent, near-white, and near-black pixels that dominate most logos. It
// returns ok=false when the image is effectively monochrome/greyscale, in which
// case the brand stage leaves the colour to Spec 07's palette derivation.
func dominantColor(img image.Image) (string, bool) {
	if img == nil {
		return "", false
	}
	bounds := img.Bounds()
	if bounds.Empty() {
		return "", false
	}

	// Sample at most ~64x64 points regardless of image size to keep this O(1).
	const samples = 64
	stepX := max1(bounds.Dx() / samples)
	stepY := max1(bounds.Dy() / samples)

	type bucket struct {
		count      int
		r, g, b    uint64
		saturation float64
	}
	buckets := map[uint32]*bucket{}
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			r16, g16, b16, a16 := img.At(x, y).RGBA()
			if a16 < 0x8000 { // mostly transparent
				continue
			}
			r, g, b := int(r16>>8), int(g16>>8), int(b16>>8)
			sat, val := saturationValue(r, g, b)
			// A low saturation already rejects near-white and greyscale chrome;
			// a low value rejects near-black. A bright, saturated colour (pure
			// red has value 1.0) is exactly what we want to keep.
			if sat < 0.20 || val < 0.15 {
				continue
			}
			// Quantize to a 4-bit-per-channel bucket so shades merge.
			key := uint32(r>>4)<<8 | uint32(g>>4)<<4 | uint32(b>>4)
			bk := buckets[key]
			if bk == nil {
				bk = &bucket{}
				buckets[key] = bk
			}
			bk.count++
			bk.r += uint64(r)
			bk.g += uint64(g)
			bk.b += uint64(b)
			bk.saturation = sat
		}
	}
	if len(buckets) == 0 {
		return "", false
	}

	var best *bucket
	for _, bk := range buckets {
		if best == nil || bk.count > best.count ||
			(bk.count == best.count && bk.saturation > best.saturation) {
			best = bk
		}
	}
	n := uint64(best.count)
	return fmt.Sprintf("#%02x%02x%02x", best.r/n, best.g/n, best.b/n), true
}

// saturationValue returns the HSV saturation and value of an 8-bit RGB colour,
// each in [0,1]. It is enough to tell a vivid brand colour from chrome grey.
func saturationValue(r, g, b int) (sat, val float64) {
	mx := max(r, g, b)
	mn := min(r, g, b)
	val = float64(mx) / 255
	if mx == 0 {
		return 0, 0
	}
	return float64(mx-mn) / float64(mx), val
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
