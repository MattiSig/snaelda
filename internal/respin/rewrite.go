package respin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// rewritePayload is the copy-rewrite stage output: only the prose fields are
// rewritten. Verbatim facts — business name, prices, hours, contact, and
// customer testimonials — are never sent through the rewrite and stay exactly as
// extracted, so the rewrite cannot fabricate or corrupt them.
type rewritePayload struct {
	Tagline  string                  `json:"tagline"`
	About    string                  `json:"about"`
	Services []rewriteServicePayload `json:"services"`
}

type rewriteServicePayload struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

const rewriteSystemPrompt = `You rewrite a small business's own website copy into natural, native marketing copy in a target language, for a tool that rebuilds their site.

You are given the business's extracted copy in its source language and a target language. Rewrite ONLY the prose into copy that reads as if a native speaker wrote it fresh for this business — never a word-for-word translation.

Rules:
- Rewrite tagline, about, and each service's name and description into the TARGET language.
- Keep the SAME number of services in the SAME order. For each, return the rewritten name and description.
- Preserve the meaning and facts. Do not invent services, benefits, or claims that were not in the source.
- Keep proper nouns, brand names, and place names as-is.
- No hype, no superlatives, no anglicisms where a natural target-language word exists. Match the direct, warm, unpretentious register a small local business actually uses.
- If a field was empty in the source, return it empty. Never fill an empty field with invented copy.

Return only the rewritten prose fields.`

// icelandicRewriteGuidance reinforces the Spec 22 Icelandic register when the
// target language is Icelandic; it mirrors the generation-side language contract
// so re-spin copy and generated copy sound like the same product.
const icelandicRewriteGuidance = `

TARGET LANGUAGE IS ICELANDIC (íslenska): write every rewritten string in natural, native Icelandic in the direct, warm, unpretentious small-business register a Reykjavík salon, café, or contractor would use. Never translated-English phrasing, no hype, no anglicism where a natural Icelandic word exists.`

func rewriteSchema(serviceCount int) map[string]any {
	stringField := map[string]any{"type": "string"}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"tagline", "about", "services"},
		"properties": map[string]any{
			"tagline": stringField,
			"about":   stringField,
			"services": map[string]any{
				"type":     "array",
				"minItems": serviceCount,
				"maxItems": serviceCount,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"name", "description"},
					"properties": map[string]any{
						"name":        stringField,
						"description": stringField,
					},
				},
			},
		},
	}
}

// RewriteCopy rewrites the extracted prose into natural native copy in the target
// language (Spec 21 step 8, Spec 22 tone). Verbatim fields pass through
// untouched. When there is no prose to rewrite, or the target language matches
// the detected source and no rewrite is warranted, it still runs so the copy is
// polished into the native register — but a completely empty extraction returns
// unchanged without spending budget.
func (a *Analyzer) RewriteCopy(ctx context.Context, fields ExtractedFields, targetLocale string) (ExtractedFields, error) {
	targetLocale = normalizeStageLocale(targetLocale)
	if !hasRewritableProse(fields) {
		return fields, nil
	}

	payloadIn := rewritePayload{
		Tagline:  fields.Tagline,
		About:    fields.About,
		Services: make([]rewriteServicePayload, len(fields.Services)),
	}
	for i, s := range fields.Services {
		payloadIn.Services[i] = rewriteServicePayload{Name: s.Name, Description: s.Description}
	}
	payloadJSON, err := json.Marshal(map[string]any{
		"targetLanguage": firstNonEmptyString(targetLocale, "is"),
		"copy":           payloadIn,
	})
	if err != nil {
		return ExtractedFields{}, fmt.Errorf("encode rewrite payload: %w", err)
	}

	system := rewriteSystemPrompt
	if targetLocale == "is" {
		system += icelandicRewriteGuidance
	}

	var result rewritePayload
	if err := a.runStage(ctx, CompletionRequest{
		Name:   "respin_rewrite",
		Schema: rewriteSchema(len(fields.Services)),
		System: system,
		User:   fmt.Sprintf("Rewrite this business copy.\n\n%s", string(payloadJSON)),
	}, &result); err != nil {
		return ExtractedFields{}, err
	}

	return mergeRewrite(fields, result), nil
}

// hasRewritableProse reports whether the extraction carries any prose worth
// rewriting; a bare extraction (no tagline/about/services) is left as-is so the
// rewrite call is skipped and no budget is spent.
func hasRewritableProse(fields ExtractedFields) bool {
	if strings.TrimSpace(fields.Tagline) != "" || strings.TrimSpace(fields.About) != "" {
		return true
	}
	for _, s := range fields.Services {
		if strings.TrimSpace(s.Name) != "" || strings.TrimSpace(s.Description) != "" {
			return true
		}
	}
	return false
}

// mergeRewrite folds the rewritten prose back onto the extracted fields,
// preserving verbatim facts (prices, hours, contact, testimonials, business
// name). A rewritten field that came back empty falls back to the original so a
// dropped value never blanks real content. Service rewrites are matched by index
// and default to the original when the model returned fewer rows.
func mergeRewrite(fields ExtractedFields, rewrite rewritePayload) ExtractedFields {
	if v := strings.TrimSpace(rewrite.Tagline); v != "" {
		fields.Tagline = v
	}
	if v := strings.TrimSpace(rewrite.About); v != "" {
		fields.About = v
	}
	for i := range fields.Services {
		if i >= len(rewrite.Services) {
			break
		}
		if name := strings.TrimSpace(rewrite.Services[i].Name); name != "" {
			fields.Services[i].Name = name
		}
		if desc := strings.TrimSpace(rewrite.Services[i].Description); desc != "" {
			fields.Services[i].Description = desc
		}
	}
	fields.MissingFields = missingFieldsFor(fields)
	return fields
}
