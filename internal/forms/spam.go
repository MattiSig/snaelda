package forms

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// SpamAssessment is the deterministic spam analysis of a submission payload.
type SpamAssessment struct {
	Score   float64
	Signals []string
}

// IsSpam reports whether the assessment crosses the auto-spam threshold.
func (a SpamAssessment) IsSpam() bool {
	return a.Score >= spamAutoThreshold
}

const (
	spamAutoThreshold = 1.0
	honeypotPrefix    = "hp_"
)

var (
	urlPattern             = regexp.MustCompile(`(?i)\bhttps?://\S+|\bwww\.\S+\.[a-z]{2,}`)
	bbcodeLinkPattern      = regexp.MustCompile(`\[(?:url|link)=`)
	htmlAnchorPattern      = regexp.MustCompile(`(?i)<a\s+[^>]*href`)
	scriptPattern          = regexp.MustCompile(`(?i)<script\b`)
	cyrillicLatinHybridPat = regexp.MustCompile(`[A-Za-z]+[\p{Cyrillic}]+|[\p{Cyrillic}]+[A-Za-z]+`)
)

var spamKeywords = []string{
	"viagra",
	"cialis",
	"casino",
	"crypto giveaway",
	"forex",
	"investment opportunity",
	"loan offer",
	"seo services",
	"backlinks",
	"buy followers",
	"adult dating",
	"escort",
	"work from home",
	"earn $",
	"make money online",
	"limited time offer",
	"click here",
	"meet hot",
	"weight loss",
	"miracle cure",
}

// assessPayload returns a deterministic spam assessment for the given
// payload plus any honeypot fields collected from the raw submission.
func assessPayload(payload map[string]any, honeypotFields map[string]string) SpamAssessment {
	score := 0.0
	signals := []string{}

	for name, value := range honeypotFields {
		if strings.TrimSpace(value) != "" {
			score += 1.0
			signals = append(signals, "honeypot:"+name)
			break
		}
	}

	combined := combinePayloadText(payload)
	combinedLower := strings.ToLower(combined)

	if linkCount := countURLs(combined); linkCount >= 5 {
		score += 0.7
		signals = append(signals, "links:high")
	} else if linkCount >= 3 {
		score += 0.4
		signals = append(signals, "links:elevated")
	}

	if bbcodeLinkPattern.MatchString(combinedLower) || htmlAnchorPattern.MatchString(combined) {
		score += 0.4
		signals = append(signals, "markup:link")
	}

	if scriptPattern.MatchString(combined) {
		score += 1.0
		signals = append(signals, "markup:script")
	}

	if isAllCapsLong(combined) {
		score += 0.3
		signals = append(signals, "casing:all_caps")
	}

	if hasRepeatedRun(combined, 6) {
		score += 0.2
		signals = append(signals, "noise:repeated_chars")
	}

	if cyrillicLatinHybridPat.MatchString(combined) {
		score += 0.3
		signals = append(signals, "noise:mixed_scripts")
	}

	if keywordHits := countKeywordHits(combinedLower); keywordHits > 0 {
		keywordScore := 0.3 * float64(keywordHits)
		if keywordScore > 0.6 {
			keywordScore = 0.6
		}
		score += keywordScore
		signals = append(signals, "keywords:matched")
	}

	if score > 1.0 {
		score = 1.0
	}

	sort.Strings(signals)
	return SpamAssessment{
		Score:   roundTo(score, 2),
		Signals: signals,
	}
}

// extractHoneypotFields strips honeypot keys from the raw payload and returns
// them separately so they never reach normal field validation.
func extractHoneypotFields(payload map[string]any) (map[string]any, map[string]string) {
	if payload == nil {
		return map[string]any{}, map[string]string{}
	}
	cleaned := map[string]any{}
	honeypot := map[string]string{}
	for name, value := range payload {
		if strings.HasPrefix(strings.ToLower(name), honeypotPrefix) {
			if text, ok := value.(string); ok {
				honeypot[name] = text
			} else if value != nil {
				honeypot[name] = "non_string"
			}
			continue
		}
		cleaned[name] = value
	}
	return cleaned, honeypot
}

func combinePayloadText(payload map[string]any) string {
	parts := []string{}
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if text, ok := payload[key].(string); ok {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " \n ")
}

func countURLs(text string) int {
	return len(urlPattern.FindAllString(text, -1))
}

// hasRepeatedRun reports whether text contains a run of the same rune that
// is at least minRun long. RE2 has no backreferences, so we scan manually.
func hasRepeatedRun(text string, minRun int) bool {
	if minRun <= 1 {
		return false
	}
	var previous rune
	count := 0
	for _, r := range text {
		if r == previous {
			count++
			if count >= minRun {
				return true
			}
			continue
		}
		previous = r
		count = 1
	}
	return false
}

func isAllCapsLong(text string) bool {
	letterCount := 0
	upperCount := 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			letterCount++
			if unicode.IsUpper(r) {
				upperCount++
			}
		}
	}
	if letterCount < 25 {
		return false
	}
	return float64(upperCount)/float64(letterCount) >= 0.75
}

func countKeywordHits(lowerText string) int {
	hits := 0
	for _, keyword := range spamKeywords {
		if strings.Contains(lowerText, keyword) {
			hits++
		}
	}
	return hits
}

func roundTo(value float64, places int) float64 {
	factor := 1.0
	for i := 0; i < places; i++ {
		factor *= 10
	}
	rounded := value * factor
	if rounded >= 0 {
		rounded += 0.5
	} else {
		rounded -= 0.5
	}
	return float64(int64(rounded)) / factor
}
