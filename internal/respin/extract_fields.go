package respin

import (
	"context"
	"fmt"
	"strings"
)

// ExtractedFields is the typed structured-extraction output (Spec 21 step 7).
// Every field is optional — the model reports what it can read and lists what it
// could not find in MissingFields rather than fabricating. Empty strings, empty
// slices, and a zero ContactDetails all mean "not found".
type ExtractedFields struct {
	BusinessName string           `json:"businessName,omitempty"`
	Tagline      string           `json:"tagline,omitempty"`
	About        string           `json:"about,omitempty"`
	Services     []ExtractService `json:"services,omitempty"`
	Hours        []ExtractHours   `json:"hours,omitempty"`
	Contact      ContactDetails   `json:"contact,omitempty"`
	Testimonials []Testimonial    `json:"testimonials,omitempty"`
	// MissingFields flags the top-level fields the model could not populate from
	// the source (e.g. "hours", "testimonials"), so the demo can be honest about
	// what it read and the composer can pre-fill gaps.
	MissingFields []string `json:"missingFields,omitempty"`
}

// ExtractService is one offering: a name, optional short description, and
// optional price as written on the source (kept verbatim, not normalized).
type ExtractService struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Price       string `json:"price,omitempty"`
}

// ExtractHours is one opening-hours row, aligned with the structured footer
// contract (canonical lowercase weekday key, HH:MM clock times, closed marker).
type ExtractHours struct {
	Day    string `json:"day"`
	Opens  string `json:"opens,omitempty"`
	Closes string `json:"closes,omitempty"`
	Closed bool   `json:"closed,omitempty"`
}

// ContactDetails holds the reachable-business fields, each optional.
type ContactDetails struct {
	Phone   string `json:"phone,omitempty"`
	Email   string `json:"email,omitempty"`
	Address string `json:"address,omitempty"`
}

// Testimonial is one customer quote with an optional attribution. Quotes are
// verbatim source content and are never fabricated or paraphrased.
type Testimonial struct {
	Quote  string `json:"quote"`
	Author string `json:"author,omitempty"`
}

// IsEmpty reports whether no contact field was found.
func (c ContactDetails) IsEmpty() bool {
	return strings.TrimSpace(c.Phone) == "" &&
		strings.TrimSpace(c.Email) == "" &&
		strings.TrimSpace(c.Address) == ""
}

const extractSystemPrompt = `You extract structured business facts from the readable text of an existing small-business website, for a tool that rebuilds that business a new site.

Extract ONLY what is actually present in the text. Never invent, guess, or fill a plausible-sounding value. A missing field is expected and fine.

Fields:
- businessName: the business's own name. Not the tagline, not the page title suffix.
- tagline: a short slogan/positioning line if one is present, else "".
- about: the business's own description of itself, cleaned into 1-3 coherent sentences in the SOURCE language. Do not translate here. Do not add marketing you did not read.
- services: concrete offerings, each with a name and (if stated) a short description and price. Keep prices verbatim as written (currency, format, and all).
- hours: opening hours if present. day is the lowercase English weekday ("monday".."sunday"). opens/closes are 24h "HH:MM". Set closed=true (and leave opens/closes "") for days marked closed. Emit one row per stated day; omit days not mentioned.
- contact: phone, email, and postal address exactly as written, each "" if absent.
- testimonials: customer quotes VERBATIM with the attributed author if given. Never write or embellish a testimonial.
- missingFields: list the field names you could not populate from the source (from: businessName, tagline, about, services, hours, contact, testimonials).

Base everything strictly on the provided text.`

func extractionSchema() map[string]any {
	stringField := map[string]any{"type": "string"}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"businessName", "tagline", "about", "services", "hours", "contact", "testimonials", "missingFields"},
		"properties": map[string]any{
			"businessName": stringField,
			"tagline":      stringField,
			"about":        stringField,
			"services": map[string]any{
				"type":     "array",
				"maxItems": 20,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"name", "description", "price"},
					"properties": map[string]any{
						"name":        stringField,
						"description": stringField,
						"price":       stringField,
					},
				},
			},
			"hours": map[string]any{
				"type":     "array",
				"maxItems": 7,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"day", "opens", "closes", "closed"},
					"properties": map[string]any{
						"day":    stringField,
						"opens":  stringField,
						"closes": stringField,
						"closed": map[string]any{"type": "boolean"},
					},
				},
			},
			"contact": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"phone", "email", "address"},
				"properties": map[string]any{
					"phone":   stringField,
					"email":   stringField,
					"address": stringField,
				},
			},
			"testimonials": map[string]any{
				"type":     "array",
				"maxItems": 12,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"quote", "author"},
					"properties": map[string]any{
						"quote":  stringField,
						"author": stringField,
					},
				},
			},
			"missingFields": map[string]any{
				"type":  "array",
				"items": stringField,
			},
		},
	}
}

// ExtractFields runs the structured field-extraction stage. The classification
// is passed as context so the model extracts against the right vertical's
// expectations, but only the source text is treated as ground truth.
func (a *Analyzer) ExtractFields(ctx context.Context, content SourceContent, classification Classification) (ExtractedFields, error) {
	if !content.HasText() {
		return ExtractedFields{}, ErrInsufficientContent
	}

	var contextLine strings.Builder
	if v := strings.TrimSpace(classification.Vertical); v != "" {
		fmt.Fprintf(&contextLine, "Likely vertical: %s. ", v)
	}
	if len(classification.Services) > 0 {
		fmt.Fprintf(&contextLine, "Signals of services: %s. ", strings.Join(classification.Services, ", "))
	}

	user := fmt.Sprintf("%sExtract the structured business facts from this website text.\n\n%s",
		contextLine.String(), content.promptDocument())

	var result ExtractedFields
	if err := a.runStage(ctx, CompletionRequest{
		Name:   "respin_extraction",
		Schema: extractionSchema(),
		System: extractSystemPrompt,
		User:   user,
	}, &result); err != nil {
		return ExtractedFields{}, err
	}

	return normalizeExtraction(result), nil
}

// normalizeExtraction trims strings, drops empty rows, canonicalizes weekday
// keys and clock times, and recomputes MissingFields from the cleaned result so
// the flag reflects what actually survived (not just what the model claimed).
func normalizeExtraction(fields ExtractedFields) ExtractedFields {
	fields.BusinessName = strings.TrimSpace(fields.BusinessName)
	fields.Tagline = strings.TrimSpace(fields.Tagline)
	fields.About = strings.TrimSpace(fields.About)

	services := make([]ExtractService, 0, len(fields.Services))
	for _, s := range fields.Services {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			continue
		}
		services = append(services, ExtractService{
			Name:        name,
			Description: strings.TrimSpace(s.Description),
			Price:       strings.TrimSpace(s.Price),
		})
	}
	fields.Services = services

	hours := make([]ExtractHours, 0, len(fields.Hours))
	seenDay := map[string]bool{}
	for _, h := range fields.Hours {
		day := canonicalWeekday(h.Day)
		if day == "" || seenDay[day] {
			continue
		}
		seenDay[day] = true
		row := ExtractHours{Day: day, Closed: h.Closed}
		if !h.Closed {
			row.Opens = canonicalClock(h.Opens)
			row.Closes = canonicalClock(h.Closes)
			if row.Opens == "" && row.Closes == "" {
				// A row with neither a time nor a closed flag carries no
				// information; drop it rather than emit an empty day.
				continue
			}
		}
		hours = append(hours, row)
	}
	fields.Hours = hours

	fields.Contact = ContactDetails{
		Phone:   strings.TrimSpace(fields.Contact.Phone),
		Email:   strings.TrimSpace(fields.Contact.Email),
		Address: strings.TrimSpace(fields.Contact.Address),
	}

	testimonials := make([]Testimonial, 0, len(fields.Testimonials))
	for _, t := range fields.Testimonials {
		quote := strings.TrimSpace(t.Quote)
		if quote == "" {
			continue
		}
		testimonials = append(testimonials, Testimonial{Quote: quote, Author: strings.TrimSpace(t.Author)})
	}
	fields.Testimonials = testimonials

	fields.MissingFields = missingFieldsFor(fields)
	return fields
}

// missingFieldsFor derives the missing-field flags from the cleaned result, so
// the honesty signal matches reality rather than the model's self-report.
func missingFieldsFor(fields ExtractedFields) []string {
	missing := make([]string, 0, 7)
	if fields.BusinessName == "" {
		missing = append(missing, "businessName")
	}
	if fields.Tagline == "" {
		missing = append(missing, "tagline")
	}
	if fields.About == "" {
		missing = append(missing, "about")
	}
	if len(fields.Services) == 0 {
		missing = append(missing, "services")
	}
	if len(fields.Hours) == 0 {
		missing = append(missing, "hours")
	}
	if fields.Contact.IsEmpty() {
		missing = append(missing, "contact")
	}
	if len(fields.Testimonials) == 0 {
		missing = append(missing, "testimonials")
	}
	return missing
}

var canonicalWeekdays = map[string]string{
	"monday": "monday", "mon": "monday",
	"tuesday": "tuesday", "tue": "tuesday", "tues": "tuesday",
	"wednesday": "wednesday", "wed": "wednesday",
	"thursday": "thursday", "thu": "thursday", "thur": "thursday", "thurs": "thursday",
	"friday": "friday", "fri": "friday",
	"saturday": "saturday", "sat": "saturday",
	"sunday": "sunday", "sun": "sunday",
	// Icelandic weekday names, so a native source that leaks through survives.
	"mánudagur": "monday", "manudagur": "monday",
	"þriðjudagur": "tuesday", "thridjudagur": "tuesday",
	"miðvikudagur": "wednesday", "midvikudagur": "wednesday",
	"fimmtudagur": "thursday",
	"föstudagur":  "friday", "fostudagur": "friday",
	"laugardagur": "saturday",
	"sunnudagur":  "sunday",
}

// canonicalWeekday maps a stated day to its canonical lowercase English key, or
// "" if it is not a recognizable weekday.
func canonicalWeekday(day string) string {
	return canonicalWeekdays[strings.ToLower(strings.TrimSpace(day))]
}

// canonicalClock normalizes a clock time to "HH:MM" (24h), returning "" when the
// value is not a parseable time. It accepts "9", "9:00", "09.00", "09:00".
func canonicalClock(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, ".", ":")
	var hh, mm int
	switch parts := strings.SplitN(value, ":", 2); len(parts) {
	case 1:
		if _, err := fmt.Sscanf(parts[0], "%d", &hh); err != nil {
			return ""
		}
	case 2:
		if _, err := fmt.Sscanf(parts[0], "%d", &hh); err != nil {
			return ""
		}
		if strings.TrimSpace(parts[1]) != "" {
			if _, err := fmt.Sscanf(parts[1], "%d", &mm); err != nil {
				return ""
			}
		}
	}
	if hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return ""
	}
	return fmt.Sprintf("%02d:%02d", hh, mm)
}
