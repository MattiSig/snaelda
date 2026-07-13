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
	FAQs     []rewriteFAQPayload     `json:"faqs"`
	Offers   []rewriteOfferPayload   `json:"offers"`
	People   []rewritePersonPayload  `json:"people"`
}

type rewriteServicePayload struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type rewriteFAQPayload struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type rewriteOfferPayload struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// rewritePersonPayload carries only a person's prose (role, bio). The name is a
// verbatim proper noun and is never sent through the rewrite.
type rewritePersonPayload struct {
	Role string `json:"role"`
	Bio  string `json:"bio"`
}

const rewriteSystemPrompt = `You rewrite a small business's own website copy into natural, native marketing copy in a target language, for a tool that rebuilds their site.

You are given the business's extracted copy in its source language and a target language. Rewrite ONLY the prose into copy that reads as if a native speaker wrote it fresh for this business — never a word-for-word translation.

Rules:
- Rewrite tagline, about, each service's name and description, each FAQ's question and answer, each offer's title and description, and each person's role and bio into the TARGET language.
- Keep the SAME number of services, faqs, offers, and people in the SAME order. For each, return the rewritten fields.
- Preserve the meaning and facts. Do not invent services, FAQs, offers, people, benefits, or claims that were not in the source.
- Keep proper nouns, brand names, place names, and people's names as-is (people names are not sent to you — only their role and bio).
- No hype, no superlatives, no anglicisms where a natural target-language word exists. Match the direct, warm, unpretentious register a small local business actually uses.
- If a field was empty in the source, return it empty. Never fill an empty field with invented copy.

Return only the rewritten prose fields.`

// icelandicRewriteGuidance reinforces the Spec 22 Icelandic register when the
// target language is Icelandic; it mirrors the generation-side language contract
// so re-spin copy and generated copy sound like the same product.
const icelandicRewriteGuidance = `

TARGET LANGUAGE IS ICELANDIC (íslenska): write every rewritten string in natural, native Icelandic in the direct, warm, unpretentious small-business register a Reykjavík salon, café, or contractor would use. Never translated-English phrasing, no hype, no anglicism where a natural Icelandic word exists.`

// rewriteCounts pins the array lengths the model must return so each rewritten
// item matches its source item by index.
type rewriteCounts struct {
	services int
	faqs     int
	offers   int
	people   int
}

func rewriteSchema(counts rewriteCounts) map[string]any {
	stringField := map[string]any{"type": "string"}
	fixedArray := func(n int, required []string, props map[string]any) map[string]any {
		return map[string]any{
			"type":     "array",
			"minItems": n,
			"maxItems": n,
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             required,
				"properties":           props,
			},
		}
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"tagline", "about", "services", "faqs", "offers", "people"},
		"properties": map[string]any{
			"tagline": stringField,
			"about":   stringField,
			"services": fixedArray(counts.services, []string{"name", "description"}, map[string]any{
				"name":        stringField,
				"description": stringField,
			}),
			"faqs": fixedArray(counts.faqs, []string{"question", "answer"}, map[string]any{
				"question": stringField,
				"answer":   stringField,
			}),
			"offers": fixedArray(counts.offers, []string{"title", "description"}, map[string]any{
				"title":       stringField,
				"description": stringField,
			}),
			"people": fixedArray(counts.people, []string{"role", "bio"}, map[string]any{
				"role": stringField,
				"bio":  stringField,
			}),
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
		FAQs:     make([]rewriteFAQPayload, len(fields.FAQs)),
		Offers:   make([]rewriteOfferPayload, len(fields.Offers)),
		People:   make([]rewritePersonPayload, len(fields.People)),
	}
	for i, s := range fields.Services {
		payloadIn.Services[i] = rewriteServicePayload{Name: s.Name, Description: s.Description}
	}
	for i, f := range fields.FAQs {
		payloadIn.FAQs[i] = rewriteFAQPayload{Question: f.Question, Answer: f.Answer}
	}
	for i, o := range fields.Offers {
		payloadIn.Offers[i] = rewriteOfferPayload{Title: o.Title, Description: o.Description}
	}
	for i, p := range fields.People {
		payloadIn.People[i] = rewritePersonPayload{Role: p.Role, Bio: p.Bio}
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
		Schema: rewriteSchema(rewriteCounts{services: len(fields.Services), faqs: len(fields.FAQs), offers: len(fields.Offers), people: len(fields.People)}),
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
	for _, f := range fields.FAQs {
		if strings.TrimSpace(f.Question) != "" || strings.TrimSpace(f.Answer) != "" {
			return true
		}
	}
	for _, o := range fields.Offers {
		if strings.TrimSpace(o.Title) != "" || strings.TrimSpace(o.Description) != "" {
			return true
		}
	}
	for _, p := range fields.People {
		if strings.TrimSpace(p.Role) != "" || strings.TrimSpace(p.Bio) != "" {
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
	for i := range fields.FAQs {
		if i >= len(rewrite.FAQs) {
			break
		}
		if q := strings.TrimSpace(rewrite.FAQs[i].Question); q != "" {
			fields.FAQs[i].Question = q
		}
		if ans := strings.TrimSpace(rewrite.FAQs[i].Answer); ans != "" {
			fields.FAQs[i].Answer = ans
		}
	}
	for i := range fields.Offers {
		if i >= len(rewrite.Offers) {
			break
		}
		if title := strings.TrimSpace(rewrite.Offers[i].Title); title != "" {
			fields.Offers[i].Title = title
		}
		if desc := strings.TrimSpace(rewrite.Offers[i].Description); desc != "" {
			fields.Offers[i].Description = desc
		}
	}
	for i := range fields.People {
		if i >= len(rewrite.People) {
			break
		}
		// The name is verbatim (never sent to the rewrite); only role/bio prose
		// comes back translated.
		if role := strings.TrimSpace(rewrite.People[i].Role); role != "" {
			fields.People[i].Role = role
		}
		if bio := strings.TrimSpace(rewrite.People[i].Bio); bio != "" {
			fields.People[i].Bio = bio
		}
	}
	fields.MissingFields = missingFieldsFor(fields)
	return fields
}
