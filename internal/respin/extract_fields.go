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
	// FAQs are the real question/answer pairs a source often keeps on a dedicated
	// page (2026-07-12 QA: ~10 real Q&As had no field to land in and were
	// discarded; the model invented one instead).
	FAQs []ExtractFAQ `json:"faqs,omitempty"`
	// ServiceAreas are the towns/regions the business serves, kept verbatim (place
	// names, not translated).
	ServiceAreas []string `json:"serviceAreas,omitempty"`
	// ClientTypes are the customer segments the business names (e.g. "homeowners",
	// "restaurants").
	ClientTypes []string `json:"clientTypes,omitempty"`
	// Offers are current promotions/announcements ("Spring Maintenance Special"),
	// kept as written.
	Offers []ExtractOffer `json:"offers,omitempty"`
	// People are the named team members, staff, or authors a source lists — lab
	// group members, theatre casts/ensembles, founders/team. Names are verbatim;
	// role and bio are prose (rewritten into the target language).
	People []ExtractPerson `json:"people,omitempty"`
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

// ExtractFAQ is one real question/answer pair read from the source.
type ExtractFAQ struct {
	Question string `json:"question"`
	Answer   string `json:"answer,omitempty"`
}

// ExtractOffer is one current promotion or announcement, kept as written.
type ExtractOffer struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// ExtractPerson is one named team member/staff/author. Name is a verbatim proper
// noun; role and bio are prose the rewrite stage translates into the target
// language.
type ExtractPerson struct {
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
	Bio  string `json:"bio,omitempty"`
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
- contact: phone, email, and postal address exactly as written, each "" if absent. If the text ends with a "CANDIDATE CONTACT SIGNALS" block, those values were harvested verbatim from the site's own contact links/markup — prefer them for the matching contact field.
- testimonials: customer quotes VERBATIM with the attributed author if given. Never write or embellish a testimonial.
- faqs: real question/answer pairs present on the source (often on a dedicated FAQ page). Keep the question and answer as written; never invent a Q&A.
- serviceAreas: the towns, regions, or neighbourhoods the business says it serves. Keep place names verbatim.
- clientTypes: the customer segments the business names it serves (e.g. "homeowners", "restaurants", "property managers").
- offers: current promotions or announcements as written (e.g. "Spring Maintenance Special"), with the descriptive line if one is given. Never invent an offer.
- people: named team members, staff, or authors the source lists, each with a name (verbatim) and, if stated, their role/title and a short bio. Never invent a person.
- missingFields: list the field names you could not populate from the source (from: businessName, tagline, about, services, hours, contact, testimonials, faqs, serviceAreas, clientTypes, offers, people).

Base everything strictly on the provided text.`

func extractionSchema() map[string]any {
	stringField := map[string]any{"type": "string"}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"businessName", "tagline", "about", "services", "hours", "contact", "testimonials", "faqs", "serviceAreas", "clientTypes", "offers", "people", "missingFields"},
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
			"faqs": map[string]any{
				"type":     "array",
				"maxItems": 20,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"question", "answer"},
					"properties": map[string]any{
						"question": stringField,
						"answer":   stringField,
					},
				},
			},
			"serviceAreas": map[string]any{
				"type":     "array",
				"maxItems": 40,
				"items":    stringField,
			},
			"clientTypes": map[string]any{
				"type":     "array",
				"maxItems": 20,
				"items":    stringField,
			},
			"offers": map[string]any{
				"type":     "array",
				"maxItems": 10,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"title", "description"},
					"properties": map[string]any{
						"title":       stringField,
						"description": stringField,
					},
				},
			},
			"people": map[string]any{
				"type":     "array",
				"maxItems": 40,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"name", "role", "bio"},
					"properties": map[string]any{
						"name": stringField,
						"role": stringField,
						"bio":  stringField,
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

	fields := normalizeExtraction(result)
	return backfillContact(fields, content.Contact), nil
}

// backfillContact fills any contact field the model left empty from the
// high-precision harvested NAP signals (tel:/mailto:/JSON-LD/microdata). The
// model's own value always wins when present; this only rescues the case the
// 2026-07-12 QA hit — a real phone/email/address that lived only in chrome the
// readability pass stripped, so the model never saw it as prose. MissingFields
// is recomputed so the honesty flag reflects the backfilled result.
func backfillContact(fields ExtractedFields, harvested ContactSignals) ExtractedFields {
	if harvested.IsEmpty() {
		return fields
	}
	if fields.Contact.Phone == "" && len(harvested.Phones) > 0 {
		fields.Contact.Phone = harvested.Phones[0]
	}
	if fields.Contact.Email == "" && len(harvested.Emails) > 0 {
		fields.Contact.Email = harvested.Emails[0]
	}
	if fields.Contact.Address == "" && len(harvested.Addresses) > 0 {
		fields.Contact.Address = harvested.Addresses[0]
	}
	fields.MissingFields = missingFieldsFor(fields)
	return fields
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

	faqs := make([]ExtractFAQ, 0, len(fields.FAQs))
	for _, f := range fields.FAQs {
		q := strings.TrimSpace(f.Question)
		if q == "" {
			continue
		}
		faqs = append(faqs, ExtractFAQ{Question: q, Answer: strings.TrimSpace(f.Answer)})
	}
	fields.FAQs = faqs

	offers := make([]ExtractOffer, 0, len(fields.Offers))
	for _, o := range fields.Offers {
		title := strings.TrimSpace(o.Title)
		if title == "" {
			continue
		}
		offers = append(offers, ExtractOffer{Title: title, Description: strings.TrimSpace(o.Description)})
	}
	fields.Offers = offers

	people := make([]ExtractPerson, 0, len(fields.People))
	for _, p := range fields.People {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		people = append(people, ExtractPerson{
			Name: name,
			Role: strings.TrimSpace(p.Role),
			Bio:  strings.TrimSpace(p.Bio),
		})
	}
	fields.People = people

	fields.ServiceAreas = dedupeAppend(nil, fields.ServiceAreas)
	fields.ClientTypes = dedupeAppend(nil, fields.ClientTypes)

	fields.MissingFields = missingFieldsFor(fields)
	return fields
}

// missingFieldsFor derives the missing-field flags from the cleaned result, so
// the honesty signal matches reality rather than the model's self-report.
func missingFieldsFor(fields ExtractedFields) []string {
	missing := make([]string, 0, 12)
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
	if len(fields.FAQs) == 0 {
		missing = append(missing, "faqs")
	}
	if len(fields.ServiceAreas) == 0 {
		missing = append(missing, "serviceAreas")
	}
	if len(fields.ClientTypes) == 0 {
		missing = append(missing, "clientTypes")
	}
	if len(fields.Offers) == 0 {
		missing = append(missing, "offers")
	}
	if len(fields.People) == 0 {
		missing = append(missing, "people")
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
