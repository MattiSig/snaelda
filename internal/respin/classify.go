package respin

import (
	"context"
	"fmt"
	"strings"
)

// classificationConfidenceFloor is the threshold below which a classification is
// treated as low-confidence: the pipeline proceeds with the generic block set
// and marks the import degraded rather than trusting a shaky vertical guess
// (Spec 21 graceful degradation).
const classificationConfidenceFloor = 0.55

// Classification is the business classification stage output (Spec 21 step 5):
// the detected vertical, the services the site offers, the content locale, the
// brand tone, and the model's confidence in the vertical call.
type Classification struct {
	// Vertical is a short lowercase industry token (e.g. "salon", "cafe",
	// "contractor"), or "" when the model cannot place the business.
	Vertical string `json:"vertical"`
	// Services is a compact list of offerings, used both as a classification
	// signal and to seed the composed brief.
	Services []string `json:"services"`
	// Locale is the detected content language as a primary subtag ("is", "en").
	Locale string `json:"locale"`
	// Tone captures the brand voice in a few words (e.g. "warm, homey").
	Tone string `json:"tone"`
	// Confidence is the model's confidence in the vertical, 0..1.
	Confidence float64 `json:"confidence"`
}

// LowConfidence reports whether the classification should be treated as a soft
// degradation: no vertical placed, or confidence below the floor.
func (c Classification) LowConfidence() bool {
	return strings.TrimSpace(c.Vertical) == "" || c.Confidence < classificationConfidenceFloor
}

const classifySystemPrompt = `You classify a small business from the readable text of its existing website, for a tool that rebuilds that business a new site.

Return ONLY the structured fields:
- vertical: a short lowercase industry token in English (examples: "salon", "barber", "cafe", "restaurant", "bakery", "contractor", "plumber", "photographer", "dentist", "gym", "florist", "consultant"). Use the closest fit. If the text is too thin or generic to place, return "".
- services: up to 8 concrete offerings the business provides, in the SAME language as the source text, deduplicated and specific (a haircut salon lists "klipping", "litun", not "quality service").
- locale: the primary language of the source content as a BCP-47 primary subtag ("is" for Icelandic, "en" for English, "sv" for Swedish, etc.). Infer from the actual copy, not the domain.
- tone: two to five words describing the brand voice as read from the copy (e.g. "warm and homey", "sharp and modern", "no-nonsense local").
- confidence: your confidence in the vertical from 0 to 1. Be honest: a clear services page scores high; a one-line placeholder scores low.

Do not invent facts. Base every field only on the provided text.`

func classificationSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"vertical", "services", "locale", "tone", "confidence"},
		"properties": map[string]any{
			"vertical": map[string]any{"type": "string"},
			"services": map[string]any{
				"type":     "array",
				"maxItems": 8,
				"items":    map[string]any{"type": "string"},
			},
			"locale":     map[string]any{"type": "string"},
			"tone":       map[string]any{"type": "string"},
			"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		},
	}
}

// Classify runs the business-classification stage over the fetched source
// content. It returns ErrCompleterUnavailable when no LLM is configured and
// ErrBudgetExhausted when the daily demo budget is spent — both degradation
// signals the caller routes into the prompt flow.
func (a *Analyzer) Classify(ctx context.Context, content SourceContent) (Classification, error) {
	if !content.HasText() {
		return Classification{}, ErrInsufficientContent
	}
	user := fmt.Sprintf("Classify this business from its website text.\n\n%s", content.promptDocument())

	var result Classification
	if err := a.runStage(ctx, CompletionRequest{
		Name:   "respin_classification",
		Schema: classificationSchema(),
		System: classifySystemPrompt,
		User:   user,
	}, &result); err != nil {
		return Classification{}, err
	}

	result.Vertical = strings.ToLower(strings.TrimSpace(result.Vertical))
	result.Locale = normalizeStageLocale(result.Locale)
	result.Tone = strings.TrimSpace(result.Tone)
	result.Services = cleanStringList(result.Services)
	if result.Confidence < 0 {
		result.Confidence = 0
	}
	if result.Confidence > 1 {
		result.Confidence = 1
	}
	return result, nil
}

// cleanStringList trims, drops empties, and de-duplicates a string slice while
// preserving order.
func cleanStringList(values []string) []string {
	seen := map[string]bool{}
	cleaned := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, v)
	}
	return cleaned
}
