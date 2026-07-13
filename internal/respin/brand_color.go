package respin

import (
	"fmt"
	"image"
	"math"
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

// varRefPattern matches a `var(--name[, fallback])` reference.
var varRefPattern = regexp.MustCompile(`var\(\s*--([a-zA-Z0-9-]+)\s*(?:,\s*([^()]*))?\)`)

// hexColorPattern matches a 3- or 6-digit hex colour literal.
var hexColorPattern = regexp.MustCompile(`#([0-9a-fA-F]{6}|[0-9a-fA-F]{3})\b`)

// rgbColorPattern matches an rgb()/rgba() functional colour.
var rgbColorPattern = regexp.MustCompile(`rgba?\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})`)

// hslFuncPattern matches an hsl()/hsla() functional colour, tolerating both the
// comma and the whitespace component separators.
var hslFuncPattern = regexp.MustCompile(`hsla?\(\s*(-?[0-9.]+)(?:deg)?\s*[,\s]\s*([0-9.]+)\s*%\s*[,\s]\s*([0-9.]+)\s*%`)

// hslTriplePattern matches a bare `H, S%, L%` triple (no hsl() wrapper) — the
// form site builders store in `*-hsl` custom properties (Squarespace's
// `--accent-hsl: 324.37,79.12%,51.18%`). It is only trusted when the property
// name declares itself HSL, since a bare triple carries no colour syntax.
var hslTriplePattern = regexp.MustCompile(`^\s*(-?[0-9.]+)(?:deg)?\s*[,\s]\s*([0-9.]+)\s*%\s*[,\s]\s*([0-9.]+)\s*%\s*$`)

// maxVarResolveDepth bounds var() chain resolution so a self-referential or
// mutually-referential custom-property graph cannot loop forever.
const maxVarResolveDepth = 12

// brandColorKeywords score a custom-property name by how strongly it signals the
// brand's primary colour. The first substring match wins, so the strongest
// signals are listed first. A variable that paints a button/CTA background is
// the strongest signal of all — the call-to-action colour *is* the brand colour
// (verified on the QA target, where the CTA button background resolves to the
// site's magenta accent).
var brandColorKeywords = []struct {
	keyword string
	score   int
}{
	{"button-background", 130},
	{"button-bg", 130},
	{"btn-background", 130},
	{"btn-bg", 130},
	{"cta", 120},
	{"primary", 100},
	{"brand", 95},
	{"accent", 80},
	{"theme", 70},
	{"main", 60},
	{"color-1", 55},
	{"color1", 55},
	{"secondary", 40},
}

// colorRoleDemotions are name fragments that mark a colour as a non-primary role
// (a hover/disabled state, a border, a secondary surface). A variable matching
// one is capped low so it never outranks a genuine primary/CTA colour.
var colorRoleDemotions = []string{"secondary", "hover", "active", "disabled", "muted", "inactive", "border", "outline", "shadow"}

const demotedColorScoreCap = 45

// nonVividColorScoreCap caps the score of a resolved colour that is effectively
// greyscale, near-white, or near-black. Chrome backgrounds (white surfaces, grey
// borders) frequently sit behind brand-named variables; capping them keeps a
// vivid brand colour on top while still letting a greyscale value win if the
// site truly has no vivid colour at all.
const nonVividColorScoreCap = 15

// extractCSSColors parses custom-property colour declarations out of collected
// CSS text and returns them ranked, most-brand-like first, deduplicated by
// value. Only custom properties are considered — ordinary `color:`/`background:`
// rules are too noisy to attribute to the brand. Values that reference another
// custom property (`var(--accent-hsl)`) are resolved through the declaration
// graph, so a button-background variable pointing at the accent colour still
// yields the accent's concrete value.
func extractCSSColors(css string) []ColorHint {
	if strings.TrimSpace(css) == "" {
		return nil
	}

	// Collect every custom-property declaration first (last wins, matching the
	// CSS cascade) so var() references can resolve regardless of source order.
	raw := map[string]string{}
	var order []string
	for _, m := range customPropPattern.FindAllStringSubmatch(css, -1) {
		name := strings.ToLower(strings.TrimSpace(m[1]))
		if _, seen := raw[name]; !seen {
			order = append(order, name)
		}
		raw[name] = strings.TrimSpace(m[2])
	}

	best := map[string]int{}
	for _, name := range order {
		hex, ok := resolveCustomPropColor(name, raw, map[string]bool{}, 0)
		if !ok {
			continue
		}
		score := scoreColorName(name)
		if score <= 0 {
			continue
		}
		if !isVividHex(hex) && score > nonVividColorScoreCap {
			score = nonVividColorScoreCap
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

// resolveCustomPropColor resolves a custom property to a concrete `#rrggbb`
// colour, following var() references through the declaration graph. The visiting
// set guards against reference cycles.
func resolveCustomPropColor(name string, raw map[string]string, visiting map[string]bool, depth int) (string, bool) {
	if depth > maxVarResolveDepth || visiting[name] {
		return "", false
	}
	value, ok := raw[name]
	if !ok {
		return "", false
	}
	visiting[name] = true
	defer delete(visiting, name)
	return resolveColorValue(name, value, raw, visiting, depth)
}

// resolveColorValue resolves a raw CSS value to a colour, given the property
// name as an interpretation hint (a bare HSL triple is only read as HSL when the
// name declares itself HSL). It handles direct literals and var() references,
// carrying the referenced variable's name forward as an additional hint so a
// triple stored behind an `*-hsl` variable is recognised even when reached from
// a differently-named variable (e.g. `--button-bg: var(--accent-hsl)`).
func resolveColorValue(hintName, value string, raw map[string]string, visiting map[string]bool, depth int) (string, bool) {
	value = strings.TrimSpace(value)
	if hex, ok := parseCSSColorNamed(hintName, value); ok {
		return hex, true
	}
	m := varRefPattern.FindStringSubmatch(value)
	if m == nil {
		return "", false
	}
	refName := strings.ToLower(strings.TrimSpace(m[1]))
	fallback := strings.TrimSpace(m[2])
	if refValue, ok := raw[refName]; ok && !visiting[refName] {
		visiting[refName] = true
		hex, ok := resolveColorValue(hintName+" "+refName, refValue, raw, visiting, depth+1)
		delete(visiting, refName)
		if ok {
			return hex, true
		}
	}
	if fallback != "" {
		return resolveColorValue(hintName, fallback, raw, visiting, depth+1)
	}
	return "", false
}

func scoreColorName(name string) int {
	base := 0
	for _, kw := range brandColorKeywords {
		if strings.Contains(name, kw.keyword) {
			base = kw.score
			break
		}
	}
	if base == 0 {
		// A named custom property that at least holds a colour is a weak signal.
		if strings.Contains(name, "color") || strings.Contains(name, "colour") {
			base = 10
		}
	}
	if base == 0 {
		return 0
	}
	for _, d := range colorRoleDemotions {
		if strings.Contains(name, d) {
			if base > demotedColorScoreCap {
				base = demotedColorScoreCap
			}
			break
		}
	}
	return base
}

// parseCSSColorNamed parses a colour literal, additionally accepting a bare HSL
// triple when the property name declares itself HSL.
func parseCSSColorNamed(name, value string) (string, bool) {
	if hex, ok := parseCSSColor(value); ok {
		return hex, true
	}
	if strings.Contains(strings.ToLower(name), "hsl") {
		if m := hslTriplePattern.FindStringSubmatch(strings.TrimSpace(value)); m != nil {
			return hslToHex(m[1], m[2], m[3])
		}
	}
	return "", false
}

// parseCSSColor extracts the first hex, rgb(), or hsl() colour from a CSS value
// and normalizes it to `#rrggbb`. It ignores values that reference another
// variable (var(--x)) or a named colour, which carry no usable literal.
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
	if m := hslFuncPattern.FindStringSubmatch(value); m != nil {
		return hslToHex(m[1], m[2], m[3])
	}
	return "", false
}

// hslToHex converts CSS HSL component strings (hue in degrees, saturation and
// lightness as percentages) to a normalized `#rrggbb` colour.
func hslToHex(hs, ss, ls string) (string, bool) {
	h, err1 := strconv.ParseFloat(hs, 64)
	s, err2 := strconv.ParseFloat(ss, 64)
	l, err3 := strconv.ParseFloat(ls, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return "", false
	}
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	s = clamp01(s / 100)
	l = clamp01(l / 100)

	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	mm := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return fmt.Sprintf("#%02x%02x%02x", to8bit(r+mm), to8bit(g+mm), to8bit(b+mm)), true
}

func to8bit(v float64) int {
	n := int(math.Round(v * 255))
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}
	return n
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// isVividHex reports whether a `#rrggbb` colour is a saturated, non-greyscale
// brand-like colour rather than white, black, or grey chrome.
func isVividHex(hex string) bool {
	r, g, b, ok := hexToRGB(hex)
	if !ok {
		return false
	}
	sat, val := saturationValue(r, g, b)
	return sat >= 0.25 && val >= 0.15
}

func hexToRGB(hex string) (r, g, b int, ok bool) {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return int(v>>16) & 0xff, int(v>>8) & 0xff, int(v) & 0xff, true
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
