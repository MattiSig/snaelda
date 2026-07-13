package siteconfig

import (
	"fmt"
	"net/mail"
	"strconv"
	"strings"
)

const BlockVersionV1 = "1.0.0"

// HeroBlockVersion is the hero's current version. 1.1.0 added the "statement"
// variant (Spec 04) — an additive change, so 1.0.0 stays registered with the
// same schema and validator and stored drafts keep resolving untouched.
const HeroBlockVersion = "1.1.0"

func heroBlockDefinition() BlockDefinition {
	editorSchema := []EditorField{
		{
			Name:        "variant",
			Label:       "Variant",
			Control:     "select",
			Options:     []string{"standard", "full-page", "statement"},
			Description: "Choose 'full-page' for an immersive image-backed hero that fills the viewport, or 'statement' for a bold type-led hero on a solid brand-color background.",
		},
		{Name: "eyebrow", Label: "Eyebrow", Control: "text"},
		{Name: "headline", Label: "Headline", Control: "text"},
		{Name: "subheadline", Label: "Subheadline", Control: "textarea"},
		{
			Name:    "primaryCta",
			Label:   "Primary CTA",
			Control: "link",
			Fields: []EditorField{
				{Name: "label", Label: "Label", Control: "text"},
				{Name: "href", Label: "Link", Control: "text", Placeholder: "/contact"},
			},
		},
		{
			Name:    "secondaryCta",
			Label:   "Secondary CTA",
			Control: "link",
			Fields: []EditorField{
				{Name: "label", Label: "Label", Control: "text"},
				{Name: "href", Label: "Link", Control: "text", Placeholder: "/about"},
			},
		},
		{
			Name:        "image",
			Label:       "Hero image",
			Control:     "asset",
			Description: "Pick an uploaded image for split, supporting, or full-page hero layouts. Required for full-page variant.",
		},
		{Name: "layout", Label: "Layout", Control: "select", Options: []string{"centered", "split-left", "split-right"}, Description: "Applies to the standard variant. Ignored when variant is full-page or statement."},
	}
	return BlockDefinition{
		Type:        "hero",
		Version:     HeroBlockVersion,
		DisplayName: "Hero",
		Category:    BlockCategoryHero,
		Tagline:     "Page-leading attention grabber with headline, optional supporting image, and primary call-to-action.",
		DefaultProps: map[string]any{
			"variant":  "standard",
			"headline": "A focused website starts here",
			"layout":   "centered",
		},
		EditorSchema:  editorSchema,
		PropSchema:    generationPropsSchema(editorSchema, "headline"),
		MigrateProps:  migrateBlockPropsPassthrough,
		ValidateProps: validateHeroProps,
	}
}

// heroBlockDefinitionV1 keeps hero@1.0.0 registered so drafts stored before the
// statement variant landed still resolve. The 1.1.0 change was additive, so both
// versions share the current schema and validator.
func heroBlockDefinitionV1() BlockDefinition {
	definition := heroBlockDefinition()
	definition.Version = BlockVersionV1
	return definition
}

func textSectionBlockDefinition() BlockDefinition {
	editorSchema := []EditorField{
		{Name: "heading", Label: "Heading", Control: "text"},
		{Name: "body", Label: "Body", Control: "textarea"},
		{Name: "alignment", Label: "Alignment", Control: "select", Options: []string{"left", "center", "right"}},
		{Name: "width", Label: "Width", Control: "select", Options: []string{"narrow", "default", "wide"}},
	}
	return BlockDefinition{
		Type:        "text_section",
		Version:     BlockVersionV1,
		DisplayName: "Text section",
		Category:    BlockCategoryContent,
		Tagline:     "Plain prose section for narrative copy.",
		DefaultProps: map[string]any{
			"heading":   "About",
			"body":      "Add focused supporting copy here.",
			"alignment": "left",
			"width":     "default",
		},
		EditorSchema:  editorSchema,
		PropSchema:    generationPropsSchema(editorSchema, "heading", "body"),
		MigrateProps:  migrateBlockPropsPassthrough,
		ValidateProps: validateTextSectionProps,
	}
}

func imageTextBlockDefinition() BlockDefinition {
	editorSchema := []EditorField{
		{Name: "heading", Label: "Heading", Control: "text"},
		{Name: "body", Label: "Body", Control: "textarea"},
		{
			Name:        "image",
			Label:       "Image",
			Control:     "asset",
			Description: "Choose one uploaded image from the site asset library.",
		},
		{
			Name:    "cta",
			Label:   "CTA",
			Control: "link",
			Fields: []EditorField{
				{Name: "label", Label: "Label", Control: "text"},
				{Name: "href", Label: "Link", Control: "text", Placeholder: "/contact"},
			},
		},
		{Name: "imagePosition", Label: "Image position", Control: "select", Options: []string{"left", "right"}},
	}
	return BlockDefinition{
		Type:        "image_text",
		Version:     BlockVersionV1,
		DisplayName: "Image and text",
		Category:    BlockCategoryMedia,
		Tagline:     "A side-by-side image and copy pairing.",
		DefaultProps: map[string]any{
			"heading":       "Built with structure",
			"body":          "Pair a short message with a supporting image.",
			"imagePosition": "right",
		},
		EditorSchema:  editorSchema,
		PropSchema:    generationPropsSchema(editorSchema, "heading", "body"),
		MigrateProps:  migrateBlockPropsPassthrough,
		ValidateProps: validateImageTextProps,
	}
}

func featuresGridBlockDefinition() BlockDefinition {
	editorSchema := []EditorField{
		{Name: "heading", Label: "Heading", Control: "text"},
		{Name: "intro", Label: "Intro", Control: "textarea"},
		{
			Name:    "items",
			Label:   "Items",
			Control: "repeater",
			ItemFields: []EditorField{
				{Name: "title", Label: "Title", Control: "text"},
				{Name: "body", Label: "Body", Control: "textarea"},
				{Name: "icon", Label: "Icon label", Control: "text"},
			},
		},
		{Name: "columns", Label: "Columns", Control: "select", ValueType: "integer", Options: []string{"2", "3", "4"}},
	}
	return BlockDefinition{
		Type:        "features_grid",
		Version:     BlockVersionV1,
		DisplayName: "Features grid",
		Category:    BlockCategoryContent,
		Tagline:     "A 2-4 column grid of short benefit cards.",
		DefaultProps: map[string]any{
			"heading": "What you get",
			"items": []any{
				map[string]any{"title": "Fast", "body": "A concise benefit statement."},
			},
			"columns": 3,
		},
		EditorSchema:  editorSchema,
		PropSchema:    generationPropsSchema(editorSchema, "heading", "items"),
		MigrateProps:  migrateBlockPropsPassthrough,
		ValidateProps: validateFeaturesGridProps,
	}
}

func ctaBandBlockDefinition() BlockDefinition {
	editorSchema := []EditorField{
		{Name: "heading", Label: "Heading", Control: "text"},
		{Name: "body", Label: "Body", Control: "textarea"},
		{
			Name:    "cta",
			Label:   "CTA",
			Control: "link",
			Fields: []EditorField{
				{Name: "label", Label: "Label", Control: "text"},
				{Name: "href", Label: "Link", Control: "text", Placeholder: "/contact"},
			},
		},
		{Name: "variant", Label: "Variant", Control: "select", Options: []string{"primary", "secondary", "accent"}},
	}
	return BlockDefinition{
		Type:        "cta_band",
		Version:     BlockVersionV1,
		DisplayName: "CTA band",
		Category:    BlockCategoryConversion,
		Tagline:     "A wide call-to-action band between sections.",
		DefaultProps: map[string]any{
			"heading": "Ready to begin?",
			"body":    "Invite visitors into the next step.",
			"variant": "primary",
		},
		EditorSchema:  editorSchema,
		PropSchema:    generationPropsSchema(editorSchema, "heading", "body"),
		MigrateProps:  migrateBlockPropsPassthrough,
		ValidateProps: validateCTABandProps,
	}
}

func contactFormBlockDefinition() BlockDefinition {
	editorSchema := []EditorField{
		{Name: "heading", Label: "Heading", Control: "text"},
		{Name: "intro", Label: "Intro", Control: "textarea"},
		{Name: "submitLabel", Label: "Submit label", Control: "text"},
		{
			Name:    "fields",
			Label:   "Form fields",
			Control: "repeater",
			ItemFields: []EditorField{
				{Name: "name", Label: "Field name", Control: "text", Placeholder: "email"},
				{Name: "label", Label: "Label", Control: "text"},
				{Name: "type", Label: "Field type", Control: "select", Options: []string{"name", "email", "phone", "message", "select"}},
				{Name: "required", Label: "Required", Control: "checkbox"},
				{
					Name:        "options",
					Label:       "Select options",
					Control:     "string_list",
					Description: "One option per line. Only used when the field type is select.",
				},
			},
		},
		{Name: "successMessage", Label: "Success message", Control: "textarea"},
		{Name: "notificationEmail", Label: "Notification email", Control: "text"},
	}
	return BlockDefinition{
		Type:        "contact_form",
		Version:     BlockVersionV1,
		DisplayName: "Contact form",
		Category:    BlockCategoryConversion,
		Tagline:     "Lead-capture form with custom fields.",
		DefaultProps: map[string]any{
			"heading":     "Start the conversation",
			"intro":       "Share a few details and I will get back to you shortly.",
			"submitLabel": "Send inquiry",
			"fields": []any{
				map[string]any{
					"name":     "name",
					"label":    "Name",
					"type":     "name",
					"required": true,
				},
				map[string]any{
					"name":     "email",
					"label":    "Email",
					"type":     "email",
					"required": true,
				},
				map[string]any{
					"name":     "message",
					"label":    "Message",
					"type":     "message",
					"required": true,
				},
			},
			"successMessage": "Thanks. Your message is on its way.",
		},
		EditorSchema:  editorSchema,
		PropSchema:    generationPropsSchema(editorSchema, "heading", "submitLabel", "fields"),
		MigrateProps:  migrateBlockPropsPassthrough,
		ValidateProps: validateContactFormProps,
	}
}

func galleryBlockDefinition() BlockDefinition {
	editorSchema := []EditorField{
		{Name: "heading", Label: "Heading", Control: "text"},
		{Name: "intro", Label: "Intro", Control: "textarea"},
		{Name: "layout", Label: "Layout", Control: "select", Options: []string{"grid", "masonry", "spotlight"}},
		{
			Name:    "images",
			Label:   "Images",
			Control: "repeater",
			ItemFields: []EditorField{
				{Name: "title", Label: "Title", Control: "text"},
				{Name: "caption", Label: "Caption", Control: "textarea"},
				{
					Name:        "image",
					Label:       "Image",
					Control:     "asset",
					Description: "Choose one uploaded image from the site asset library.",
				},
			},
		},
	}
	return BlockDefinition{
		Type:        "gallery",
		Version:     BlockVersionV1,
		DisplayName: "Gallery",
		Category:    BlockCategoryMedia,
		Tagline:     "A visual grid of curated images.",
		DefaultProps: map[string]any{
			"heading": "Selected work",
			"images": []any{
				map[string]any{
					"title":   "Feature image",
					"caption": "Describe the work, material, or project shown here.",
				},
			},
			"layout": "grid",
		},
		EditorSchema:  editorSchema,
		PropSchema:    generationPropsSchema(editorSchema, "heading", "images"),
		MigrateProps:  migrateBlockPropsPassthrough,
		ValidateProps: validateGalleryProps,
	}
}

func testimonialsBlockDefinition() BlockDefinition {
	editorSchema := []EditorField{
		{Name: "heading", Label: "Heading", Control: "text"},
		{Name: "intro", Label: "Intro", Control: "textarea"},
		{
			Name:    "items",
			Label:   "Testimonials",
			Control: "repeater",
			ItemFields: []EditorField{
				{Name: "quote", Label: "Quote", Control: "textarea"},
				{Name: "name", Label: "Name", Control: "text"},
				{Name: "role", Label: "Role", Control: "text"},
				{Name: "avatar", Label: "Avatar", Control: "asset"},
			},
		},
	}
	return BlockDefinition{
		Type:        "testimonials",
		Version:     BlockVersionV1,
		DisplayName: "Testimonials",
		Category:    BlockCategoryContent,
		Tagline:     "Customer quotes with attribution.",
		DefaultProps: map[string]any{
			"heading": "What clients say",
			"items": []any{
				map[string]any{
					"quote": "Add a short testimonial that sounds specific, calm, and believable.",
					"name":  "Client name",
					"role":  "Client role or company",
				},
			},
		},
		EditorSchema:  editorSchema,
		PropSchema:    generationPropsSchema(editorSchema, "heading", "items"),
		MigrateProps:  migrateBlockPropsPassthrough,
		ValidateProps: validateTestimonialsProps,
	}
}

func pricingPackagesBlockDefinition() BlockDefinition {
	editorSchema := []EditorField{
		{Name: "heading", Label: "Heading", Control: "text"},
		{Name: "intro", Label: "Intro", Control: "textarea"},
		{
			Name:    "plans",
			Label:   "Plans",
			Control: "repeater",
			ItemFields: []EditorField{
				{Name: "name", Label: "Name", Control: "text"},
				{Name: "price", Label: "Price", Control: "text"},
				{Name: "description", Label: "Description", Control: "textarea"},
				{
					Name:    "features",
					Label:   "Features",
					Control: "repeater",
					ItemFields: []EditorField{
						{Name: "text", Label: "Feature", Control: "text"},
					},
				},
				{
					Name:    "cta",
					Label:   "CTA",
					Control: "link",
					Fields: []EditorField{
						{Name: "label", Label: "Label", Control: "text"},
						{Name: "href", Label: "Link", Control: "text", Placeholder: "/contact"},
					},
				},
			},
		},
	}
	return BlockDefinition{
		Type:        "pricing_packages",
		Version:     BlockVersionV1,
		DisplayName: "Pricing packages",
		Category:    BlockCategoryConversion,
		Tagline:     "Stacked pricing tiers with feature lists.",
		DefaultProps: map[string]any{
			"heading": "Packages",
			"plans": []any{
				map[string]any{
					"name":        "Starter",
					"price":       "$450",
					"description": "A focused entry package for simple needs.",
					"features": []any{
						map[string]any{"text": "One clear outcome"},
						map[string]any{"text": "Friendly handoff"},
					},
				},
			},
		},
		EditorSchema:  editorSchema,
		PropSchema:    generationPropsSchema(editorSchema, "heading", "plans"),
		MigrateProps:  migrateBlockPropsPassthrough,
		ValidateProps: validatePricingPackagesProps,
	}
}

func faqBlockDefinition() BlockDefinition {
	editorSchema := []EditorField{
		{Name: "heading", Label: "Heading", Control: "text"},
		{Name: "intro", Label: "Intro", Control: "textarea"},
		{
			Name:    "items",
			Label:   "Questions",
			Control: "repeater",
			ItemFields: []EditorField{
				{Name: "question", Label: "Question", Control: "text"},
				{Name: "answer", Label: "Answer", Control: "textarea"},
			},
		},
	}
	return BlockDefinition{
		Type:        "faq",
		Version:     BlockVersionV1,
		DisplayName: "FAQ",
		Category:    BlockCategoryContent,
		Tagline:     "Question and answer accordion.",
		DefaultProps: map[string]any{
			"heading": "Questions people ask first",
			"items": []any{
				map[string]any{
					"question": "What should someone know before they reach out?",
					"answer":   "Use this answer to reduce hesitation and set expectations clearly.",
				},
			},
		},
		EditorSchema:  editorSchema,
		PropSchema:    generationPropsSchema(editorSchema, "heading", "items"),
		MigrateProps:  migrateBlockPropsPassthrough,
		ValidateProps: validateFAQProps,
	}
}

func teamProfileCardsBlockDefinition() BlockDefinition {
	editorSchema := []EditorField{
		{Name: "heading", Label: "Heading", Control: "text"},
		{Name: "intro", Label: "Intro", Control: "textarea"},
		{
			Name:    "people",
			Label:   "People",
			Control: "repeater",
			ItemFields: []EditorField{
				{Name: "name", Label: "Name", Control: "text"},
				{Name: "role", Label: "Role", Control: "text"},
				{Name: "bio", Label: "Bio", Control: "textarea"},
				{Name: "photo", Label: "Photo", Control: "asset"},
				{
					Name:    "links",
					Label:   "Links",
					Control: "repeater",
					ItemFields: []EditorField{
						{Name: "label", Label: "Label", Control: "text"},
						{Name: "href", Label: "Link", Control: "text", Placeholder: "/about"},
					},
				},
			},
		},
	}
	return BlockDefinition{
		Type:        "team_profile_cards",
		Version:     BlockVersionV1,
		DisplayName: "Team profile cards",
		Category:    BlockCategoryContent,
		Tagline:     "Team intro cards with photos.",
		DefaultProps: map[string]any{
			"heading": "The people behind the work",
			"people": []any{
				map[string]any{
					"name": "Team member",
					"role": "Role",
					"bio":  "Add a short bio that explains the person’s focus and perspective.",
					"links": []any{
						map[string]any{
							"label": "Profile",
							"href":  "/about",
						},
					},
				},
			},
		},
		EditorSchema:  editorSchema,
		PropSchema:    generationPropsSchema(editorSchema, "heading", "people"),
		MigrateProps:  migrateBlockPropsPassthrough,
		ValidateProps: validateTeamProfileCardsProps,
	}
}

func statsBlockDefinition() BlockDefinition {
	editorSchema := []EditorField{
		{Name: "heading", Label: "Heading", Control: "text"},
		{Name: "intro", Label: "Intro", Control: "textarea"},
		{
			Name:    "items",
			Label:   "Stats",
			Control: "repeater",
			ItemFields: []EditorField{
				{Name: "value", Label: "Value", Control: "text"},
				{Name: "label", Label: "Label", Control: "text"},
				{Name: "description", Label: "Description", Control: "textarea"},
			},
		},
	}
	return BlockDefinition{
		Type:        "stats",
		Version:     BlockVersionV1,
		DisplayName: "Stats",
		Category:    BlockCategoryContent,
		Tagline:     "Punchy metrics with short labels.",
		DefaultProps: map[string]any{
			"heading": "By the numbers",
			"items": []any{
				map[string]any{"value": "120+", "label": "Projects delivered"},
				map[string]any{"value": "98%", "label": "Client satisfaction"},
				map[string]any{"value": "24/7", "label": "Support available"},
			},
		},
		EditorSchema:  editorSchema,
		PropSchema:    generationPropsSchema(editorSchema, "heading", "items"),
		MigrateProps:  migrateBlockPropsPassthrough,
		ValidateProps: validateStatsProps,
	}
}

func collectionListBlockDefinition() BlockDefinition {
	editorSchema := []EditorField{
		{Name: "heading", Label: "Heading", Control: "text"},
		{Name: "intro", Label: "Intro", Control: "textarea"},
		{Name: "collection", Label: "Collection", Control: "collection-picker"},
		{Name: "limit", Label: "Items to show", Control: "number"},
		{Name: "layout", Label: "Layout", Control: "select", Options: []string{"grid", "list"}},
		{Name: "showFilters", Label: "Show filter chips", Control: "boolean"},
		{
			Name:    "cta",
			Label:   "CTA",
			Control: "link",
			Fields: []EditorField{
				{Name: "label", Label: "Label", Control: "text"},
				{Name: "href", Label: "Link", Control: "text", Placeholder: "/services"},
			},
		},
	}
	return BlockDefinition{
		Type:        "collection_list",
		Version:     BlockVersionV1,
		DisplayName: "Collection list",
		Category:    BlockCategoryContent,
		Tagline:     "Live list of entries from a collection.",
		DefaultProps: map[string]any{
			"heading":     "Featured",
			"collection":  "",
			"limit":       6,
			"showFilters": false,
		},
		EditorSchema:  editorSchema,
		PropSchema:    generationPropsSchema(editorSchema),
		MigrateProps:  migrateBlockPropsPassthrough,
		ValidateProps: validateCollectionListProps,
	}
}

func collectionIndexBlockDefinition() BlockDefinition {
	editorSchema := []EditorField{
		{Name: "heading", Label: "Heading", Control: "text"},
		{Name: "intro", Label: "Intro", Control: "textarea"},
		{Name: "sort", Label: "Default sort", Control: "select", Options: []string{"manual", "newest", "oldest", "title"}},
		{Name: "layout", Label: "Layout", Control: "select", Options: []string{"grid", "list"}},
		{Name: "showSort", Label: "Allow visitors to sort", Control: "boolean"},
		{Name: "showFilters", Label: "Show filter chips", Control: "boolean"},
	}
	return BlockDefinition{
		Type:        "collection_index",
		Version:     BlockVersionV1,
		DisplayName: "Collection index",
		Category:    BlockCategoryContent,
		Tagline:     "Index page listing all entries.",
		DefaultProps: map[string]any{
			"heading":  "All entries",
			"sort":     "manual",
			"showSort": false,
		},
		EditorSchema:  editorSchema,
		PropSchema:    generationPropsSchema(editorSchema),
		MigrateProps:  migrateBlockPropsPassthrough,
		ValidateProps: validateCollectionIndexProps,
	}
}

func collectionDetailBlockDefinition() BlockDefinition {
	editorSchema := []EditorField{
		{Name: "heading", Label: "Heading", Control: "text", Description: "Optional override; defaults to the entry title"},
		{Name: "layout", Label: "Layout", Control: "select", Options: []string{"default", "narrow", "wide"}},
	}
	return BlockDefinition{
		Type:        "collection_detail",
		Version:     BlockVersionV1,
		DisplayName: "Collection detail",
		Category:    BlockCategoryContent,
		Tagline:     "Renders a single entry's content.",
		DefaultProps: map[string]any{
			"layout": "default",
		},
		EditorSchema:  editorSchema,
		PropSchema:    generationPropsSchema(editorSchema),
		MigrateProps:  migrateBlockPropsPassthrough,
		ValidateProps: validateCollectionDetailProps,
	}
}

func footerBlockDefinition() BlockDefinition {
	editorSchema := []EditorField{
		{Name: "showBrand", Label: "Show business name and logo", Control: "checkbox"},
		{Name: "tagline", Label: "Tagline", Control: "textarea"},
		{Name: "showMadeWith", Label: "Show \"Made with Snælda\" link", Control: "checkbox"},
		{
			Name:    "contact",
			Label:   "Contact details",
			Control: "object",
			Fields: []EditorField{
				{
					Name:    "address",
					Label:   "Address",
					Control: "object",
					Fields: []EditorField{
						{Name: "street", Label: "Street", Control: "text"},
						{Name: "city", Label: "City", Control: "text"},
						{Name: "postalCode", Label: "Postal code", Control: "text"},
						{Name: "region", Label: "Region", Control: "text"},
						{Name: "country", Label: "Country", Control: "text"},
					},
				},
				{Name: "phone", Label: "Phone", Control: "text", Placeholder: "+354 555 1234"},
				{Name: "email", Label: "Email", Control: "text"},
				{
					Name:        "hours",
					Label:       "Opening hours",
					Control:     "repeater",
					Description: "One entry per open (or closed) day of the week.",
					ItemFields: []EditorField{
						{Name: "day", Label: "Day", Control: "select", Options: footerWeekdays},
						{Name: "opens", Label: "Opens", Control: "text", Placeholder: "09:00"},
						{Name: "closes", Label: "Closes", Control: "text", Placeholder: "17:00"},
						{Name: "closed", Label: "Closed", Control: "checkbox"},
					},
				},
			},
		},
		{Name: "copyright", Label: "Copyright", Control: "text"},
		{
			Name:    "socialLinks",
			Label:   "Social links",
			Control: "repeater",
			ItemFields: []EditorField{
				{Name: "label", Label: "Label", Control: "text"},
				{Name: "href", Label: "Link", Control: "text", Placeholder: "https://instagram.com/example"},
			},
		},
	}
	return BlockDefinition{
		Type:        "footer",
		Version:     BlockVersionV1,
		DisplayName: "Footer",
		Category:    BlockCategoryContent,
		Tagline:     "Site-wide footer with link columns.",
		DefaultProps: map[string]any{
			"showBrand":    true,
			"showMadeWith": true,
			"tagline":      "A short closing line that reinforces the tone of the site.",
			"contact": map[string]any{
				"email": "hello@example.com",
			},
			"copyright": "Copyright 2026 Site name",
		},
		EditorSchema:  editorSchema,
		PropSchema:    generationPropsSchema(editorSchema, "copyright"),
		MigrateProps:  migrateBlockPropsPassthrough,
		ValidateProps: validateFooterProps,
	}
}

func generationPropsSchema(fields []EditorField, _ ...string) map[string]any {
	// OpenAI Structured Outputs (strict mode) requires every property in
	// `properties` to be listed in `required`. We include every field name so
	// the schema is acceptable; the model can still emit empty strings or empty
	// arrays for fields that are semantically optional in the editor.
	properties := make(map[string]any, len(fields))
	required := make([]string, 0, len(fields))
	for _, field := range fields {
		properties[field.Name] = generationFieldSchema(field)
		required = append(required, field.Name)
	}
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func generationFieldSchema(field EditorField) map[string]any {
	switch field.Control {
	case "textarea", "text", "collection-picker":
		return map[string]any{"type": "string"}
	case "select":
		if field.ValueType == "integer" {
			values := make([]int, 0, len(field.Options))
			for _, option := range field.Options {
				if parsed, err := strconv.Atoi(option); err == nil {
					values = append(values, parsed)
				}
			}
			schema := map[string]any{"type": "integer"}
			if len(values) > 0 {
				schema["enum"] = values
			}
			return schema
		}
		schema := map[string]any{"type": "string"}
		if len(field.Options) > 0 {
			schema["enum"] = field.Options
		}
		return schema
	case "number":
		return map[string]any{"type": "integer"}
	case "boolean", "checkbox":
		return map[string]any{"type": "boolean"}
	case "string_list":
		return map[string]any{
			"type":  "array",
			"items": map[string]any{"type": "string"},
		}
	case "asset":
		return map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"assetId": map[string]any{"type": "string"},
				"alt":     map[string]any{"type": "string"},
			},
			"required": []string{"assetId", "alt"},
		}
	case "link":
		fields := field.Fields
		if len(fields) == 0 {
			fields = []EditorField{
				{Name: "label", Control: "text"},
				{Name: "href", Control: "text"},
			}
		}
		return generationPropsSchema(fields, "label", "href")
	case "object":
		return generationPropsSchema(field.Fields)
	case "repeater":
		return map[string]any{
			"type":  "array",
			"items": generationPropsSchema(field.ItemFields),
		}
	default:
		return map[string]any{}
	}
}

func migrateBlockPropsPassthrough(_ string, previousProps map[string]any) map[string]any {
	return cloneBlockProps(previousProps)
}

func cloneBlockProps(previousProps map[string]any) map[string]any {
	if previousProps == nil {
		return map[string]any{}
	}
	clone := make(map[string]any, len(previousProps))
	for key, value := range previousProps {
		clone[key] = cloneBlockPropValue(value)
	}
	return clone
}

func cloneBlockPropValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneBlockProps(typed)
	case []any:
		cloned := make([]any, len(typed))
		for index, item := range typed {
			cloned[index] = cloneBlockPropValue(item)
		}
		return cloned
	default:
		return typed
	}
}

func validateHeroProps(path string, props map[string]any, c *collector) {
	requireKnownProps(path, props, c, "variant", "eyebrow", "headline", "subheadline", "primaryCta", "secondaryCta", "image", "layout")
	optionalEnum(path, props, "variant", c, "standard", "full-page", "statement")
	optionalString(path, props, "eyebrow", 80, c)
	requireString(path, props, "headline", 1, 120, c)
	optionalString(path, props, "subheadline", 280, c)
	optionalCTA(path, props, "primaryCta", c)
	optionalCTA(path, props, "secondaryCta", c)
	optionalImage(path, props, "image", c)
	optionalEnum(path, props, "layout", c, "centered", "split-left", "split-right")
}

func validateTextSectionProps(path string, props map[string]any, c *collector) {
	requireKnownProps(path, props, c, "heading", "body", "alignment", "width")
	requireString(path, props, "heading", 1, 120, c)
	requireString(path, props, "body", 1, 4000, c)
	optionalEnum(path, props, "alignment", c, "left", "center", "right")
	optionalEnum(path, props, "width", c, "narrow", "default", "wide")
}

func validateImageTextProps(path string, props map[string]any, c *collector) {
	requireKnownProps(path, props, c, "heading", "body", "image", "imagePosition", "cta")
	requireString(path, props, "heading", 1, 120, c)
	requireString(path, props, "body", 1, 2500, c)
	optionalImage(path, props, "image", c)
	optionalEnum(path, props, "imagePosition", c, "left", "right")
	optionalCTA(path, props, "cta", c)
}

func validateFeaturesGridProps(path string, props map[string]any, c *collector) {
	requireKnownProps(path, props, c, "heading", "intro", "items", "columns")
	requireString(path, props, "heading", 1, 120, c)
	optionalString(path, props, "intro", 500, c)
	optionalIntEnum(path, props, "columns", c, 2, 3, 4)

	itemsPath := child(path, "items")
	items, ok := props["items"]
	if !ok {
		c.add(itemsPath, "required", "items is required")
		return
	}
	values, ok := asSlice(items)
	if !ok {
		c.add(itemsPath, "invalid_type", "items must be an array")
		return
	}
	if len(values) == 0 || len(values) > 12 {
		c.add(itemsPath, "invalid_length", "items must include between 1 and 12 entries")
	}
	for index, value := range values {
		itemPath := fmt.Sprintf("%s[%d]", itemsPath, index)
		item, ok := asObject(value)
		if !ok {
			c.add(itemPath, "invalid_type", "feature item must be an object")
			continue
		}
		requireKnownProps(itemPath, item, c, "icon", "title", "body")
		optionalString(itemPath, item, "icon", 40, c)
		requireString(itemPath, item, "title", 1, 80, c)
		requireString(itemPath, item, "body", 1, 280, c)
	}
}

func validateCTABandProps(path string, props map[string]any, c *collector) {
	requireKnownProps(path, props, c, "heading", "body", "cta", "variant")
	requireString(path, props, "heading", 1, 120, c)
	requireString(path, props, "body", 1, 600, c)
	optionalCTA(path, props, "cta", c)
	optionalEnum(path, props, "variant", c, "primary", "secondary", "accent")
}

func validateGalleryProps(path string, props map[string]any, c *collector) {
	requireKnownProps(path, props, c, "heading", "intro", "images", "layout")
	requireString(path, props, "heading", 1, 120, c)
	optionalString(path, props, "intro", 500, c)
	optionalEnum(path, props, "layout", c, "grid", "masonry", "spotlight")

	imagesPath := child(path, "images")
	images, ok := props["images"]
	if !ok {
		c.add(imagesPath, "required", "images is required")
		return
	}
	values, ok := asSlice(images)
	if !ok {
		c.add(imagesPath, "invalid_type", "images must be an array")
		return
	}
	if len(values) == 0 || len(values) > 12 {
		c.add(imagesPath, "invalid_length", "images must include between 1 and 12 entries")
	}
	for index, value := range values {
		itemPath := fmt.Sprintf("%s[%d]", imagesPath, index)
		item, ok := asObject(value)
		if !ok {
			c.add(itemPath, "invalid_type", "gallery image must be an object")
			continue
		}
		requireKnownProps(itemPath, item, c, "title", "caption", "image")
		requireString(itemPath, item, "title", 1, 80, c)
		optionalString(itemPath, item, "caption", 240, c)
		optionalImage(itemPath, item, "image", c)
	}
}

func validateContactFormProps(path string, props map[string]any, c *collector) {
	requireKnownProps(path, props, c, "heading", "intro", "submitLabel", "fields", "successMessage", "notificationEmail")
	requireString(path, props, "heading", 1, 120, c)
	optionalString(path, props, "intro", 600, c)
	requireString(path, props, "submitLabel", 1, 40, c)
	optionalString(path, props, "successMessage", 200, c)
	optionalString(path, props, "notificationEmail", 160, c)

	definition, err := FormDefinitionFromProps(props)
	if validationErr, ok := err.(ValidationError); ok {
		for _, issue := range validationErr.Issues {
			message := issue.Message
			if issue.Path == "form" {
				c.add(path, issue.Code, message)
				continue
			}
			c.add(strings.Replace(issue.Path, "form", path, 1), issue.Code, message)
		}
		return
	}
	if err != nil {
		c.add(path, "invalid_form", err.Error())
		return
	}

	for index, field := range definition.Fields {
		if field.Type == "select" && len(field.Options) == 0 {
			c.add(fmt.Sprintf("%s.fields[%d].options", path, index), "required", "select fields must include options")
		}
	}
}

func validateTestimonialsProps(path string, props map[string]any, c *collector) {
	requireKnownProps(path, props, c, "heading", "intro", "items")
	requireString(path, props, "heading", 1, 120, c)
	optionalString(path, props, "intro", 500, c)

	itemsPath := child(path, "items")
	items, ok := props["items"]
	if !ok {
		c.add(itemsPath, "required", "items is required")
		return
	}
	values, ok := asSlice(items)
	if !ok {
		c.add(itemsPath, "invalid_type", "items must be an array")
		return
	}
	if len(values) == 0 || len(values) > 6 {
		c.add(itemsPath, "invalid_length", "items must include between 1 and 6 entries")
	}
	for index, value := range values {
		itemPath := fmt.Sprintf("%s[%d]", itemsPath, index)
		item, ok := asObject(value)
		if !ok {
			c.add(itemPath, "invalid_type", "testimonial must be an object")
			continue
		}
		requireKnownProps(itemPath, item, c, "quote", "name", "role", "avatar")
		requireString(itemPath, item, "quote", 1, 320, c)
		requireString(itemPath, item, "name", 1, 80, c)
		optionalString(itemPath, item, "role", 120, c)
		optionalImage(itemPath, item, "avatar", c)
	}
}

func validatePricingPackagesProps(path string, props map[string]any, c *collector) {
	requireKnownProps(path, props, c, "heading", "intro", "plans")
	requireString(path, props, "heading", 1, 120, c)
	optionalString(path, props, "intro", 500, c)

	plansPath := child(path, "plans")
	plans, ok := props["plans"]
	if !ok {
		c.add(plansPath, "required", "plans is required")
		return
	}
	values, ok := asSlice(plans)
	if !ok {
		c.add(plansPath, "invalid_type", "plans must be an array")
		return
	}
	if len(values) == 0 || len(values) > 4 {
		c.add(plansPath, "invalid_length", "plans must include between 1 and 4 entries")
	}
	for index, value := range values {
		itemPath := fmt.Sprintf("%s[%d]", plansPath, index)
		item, ok := asObject(value)
		if !ok {
			c.add(itemPath, "invalid_type", "plan must be an object")
			continue
		}
		requireKnownProps(itemPath, item, c, "name", "price", "description", "features", "cta")
		requireString(itemPath, item, "name", 1, 80, c)
		requireString(itemPath, item, "price", 1, 40, c)
		requireString(itemPath, item, "description", 1, 240, c)
		optionalCTA(itemPath, item, "cta", c)

		featuresPath := child(itemPath, "features")
		features, ok := item["features"]
		if !ok {
			c.add(featuresPath, "required", "features is required")
			continue
		}
		featureValues, ok := asSlice(features)
		if !ok {
			c.add(featuresPath, "invalid_type", "features must be an array")
			continue
		}
		if len(featureValues) == 0 || len(featureValues) > 6 {
			c.add(featuresPath, "invalid_length", "features must include between 1 and 6 entries")
		}
		for featureIndex, featureValue := range featureValues {
			featurePath := fmt.Sprintf("%s[%d]", featuresPath, featureIndex)
			feature, ok := asObject(featureValue)
			if !ok {
				c.add(featurePath, "invalid_type", "feature must be an object")
				continue
			}
			requireKnownProps(featurePath, feature, c, "text")
			requireString(featurePath, feature, "text", 1, 120, c)
		}
	}
}

func validateFAQProps(path string, props map[string]any, c *collector) {
	requireKnownProps(path, props, c, "heading", "intro", "items")
	requireString(path, props, "heading", 1, 120, c)
	optionalString(path, props, "intro", 500, c)

	itemsPath := child(path, "items")
	items, ok := props["items"]
	if !ok {
		c.add(itemsPath, "required", "items is required")
		return
	}
	values, ok := asSlice(items)
	if !ok {
		c.add(itemsPath, "invalid_type", "items must be an array")
		return
	}
	if len(values) == 0 || len(values) > 10 {
		c.add(itemsPath, "invalid_length", "items must include between 1 and 10 entries")
	}
	for index, value := range values {
		itemPath := fmt.Sprintf("%s[%d]", itemsPath, index)
		item, ok := asObject(value)
		if !ok {
			c.add(itemPath, "invalid_type", "faq item must be an object")
			continue
		}
		requireKnownProps(itemPath, item, c, "question", "answer")
		requireString(itemPath, item, "question", 1, 140, c)
		requireString(itemPath, item, "answer", 1, 400, c)
	}
}

func validateTeamProfileCardsProps(path string, props map[string]any, c *collector) {
	requireKnownProps(path, props, c, "heading", "intro", "people")
	requireString(path, props, "heading", 1, 120, c)
	optionalString(path, props, "intro", 500, c)

	peoplePath := child(path, "people")
	people, ok := props["people"]
	if !ok {
		c.add(peoplePath, "required", "people is required")
		return
	}
	values, ok := asSlice(people)
	if !ok {
		c.add(peoplePath, "invalid_type", "people must be an array")
		return
	}
	if len(values) == 0 || len(values) > 8 {
		c.add(peoplePath, "invalid_length", "people must include between 1 and 8 entries")
	}
	for index, value := range values {
		itemPath := fmt.Sprintf("%s[%d]", peoplePath, index)
		item, ok := asObject(value)
		if !ok {
			c.add(itemPath, "invalid_type", "person must be an object")
			continue
		}
		requireKnownProps(itemPath, item, c, "name", "role", "bio", "photo", "links")
		requireString(itemPath, item, "name", 1, 80, c)
		requireString(itemPath, item, "role", 1, 80, c)
		requireString(itemPath, item, "bio", 1, 400, c)
		optionalImage(itemPath, item, "photo", c)
		optionalLinkList(itemPath, item, "links", 0, 3, c)
	}
}

func validateFooterProps(path string, props map[string]any, c *collector) {
	requireKnownProps(path, props, c, "showBrand", "showMadeWith", "tagline", "contact", "copyright", "socialLinks", "siteName", "contactLine", "navigationLinks")
	optionalBool(path, props, "showBrand", c)
	optionalBool(path, props, "showMadeWith", c)
	optionalString(path, props, "tagline", 240, c)
	validateFooterContact(path, props, "contact", c)
	optionalString(path, props, "siteName", 120, c)
	optionalString(path, props, "contactLine", 180, c)
	requireString(path, props, "copyright", 1, 120, c)
	optionalLinkList(path, props, "navigationLinks", 0, 6, c)
	optionalLinkList(path, props, "socialLinks", 0, 6, c)
}

func requireKnownProps(path string, props map[string]any, c *collector, names ...string) {
	allowed := map[string]bool{}
	for _, name := range names {
		allowed[name] = true
	}
	for key := range props {
		if !allowed[key] {
			c.add(child(path, key), "unknown_property", "property is not supported")
		}
	}
}

func requireString(path string, props map[string]any, key string, minLength int, maxLength int, c *collector) {
	value, ok := props[key]
	fieldPath := child(path, key)
	if !ok {
		c.add(fieldPath, "required", key+" is required")
		return
	}
	text, ok := value.(string)
	if !ok {
		c.add(fieldPath, "invalid_type", key+" must be a string")
		return
	}
	trimmed := strings.TrimSpace(text)
	validateStringLength(fieldPath, trimmed, minLength, maxLength, c)
	validatePlainText(fieldPath, trimmed, c)
}

func optionalString(path string, props map[string]any, key string, maxLength int, c *collector) {
	value, ok := props[key]
	if !ok || value == nil {
		return
	}
	text, ok := value.(string)
	fieldPath := child(path, key)
	if !ok {
		c.add(fieldPath, "invalid_type", key+" must be a string")
		return
	}
	trimmed := strings.TrimSpace(text)
	validateStringLength(fieldPath, trimmed, 0, maxLength, c)
	validatePlainText(fieldPath, trimmed, c)
}

func validateStringLength(path string, text string, minLength int, maxLength int, c *collector) {
	if len(text) < minLength {
		c.add(path, "invalid_length", "value is too short")
	}
	if len(text) > maxLength {
		c.add(path, "invalid_length", "value is too long")
	}
}

func optionalEnum(path string, props map[string]any, key string, c *collector, values ...string) {
	value, ok := props[key]
	if !ok || value == nil {
		return
	}
	text, ok := value.(string)
	fieldPath := child(path, key)
	if !ok {
		c.add(fieldPath, "invalid_type", key+" must be a string")
		return
	}
	for _, allowed := range values {
		if text == allowed {
			return
		}
	}
	c.add(fieldPath, "invalid_value", key+" is not supported")
}

func optionalIntEnum(path string, props map[string]any, key string, c *collector, values ...int) {
	value, ok := props[key]
	if !ok || value == nil {
		return
	}
	number, ok := asInt(value)
	fieldPath := child(path, key)
	if !ok {
		c.add(fieldPath, "invalid_type", key+" must be an integer")
		return
	}
	for _, allowed := range values {
		if number == allowed {
			return
		}
	}
	c.add(fieldPath, "invalid_value", key+" is not supported")
}

func optionalCTA(path string, props map[string]any, key string, c *collector) {
	value, ok := props[key]
	if !ok || value == nil {
		return
	}
	object, ok := asObject(value)
	fieldPath := child(path, key)
	if !ok {
		c.add(fieldPath, "invalid_type", key+" must be an object")
		return
	}
	requireKnownProps(fieldPath, object, c, "label", "href")
	requireString(fieldPath, object, "label", 1, 40, c)
	href, ok := object["href"].(string)
	if !ok {
		c.add(child(fieldPath, "href"), "invalid_type", "href must be a string")
		return
	}
	if err := ValidateURL(href); err != nil {
		c.add(child(fieldPath, "href"), "unsafe_url", err.Error())
	}
}

func optionalImage(path string, props map[string]any, key string, c *collector) {
	value, ok := props[key]
	if !ok || value == nil {
		return
	}
	object, ok := asObject(value)
	fieldPath := child(path, key)
	if !ok {
		c.add(fieldPath, "invalid_type", key+" must be an object")
		return
	}
	requireKnownProps(fieldPath, object, c, "assetId", "alt")
	requireString(fieldPath, object, "assetId", 1, 120, c)
	requireString(fieldPath, object, "alt", 1, 180, c)
}

func validateFooterContact(path string, props map[string]any, key string, c *collector) {
	value, ok := props[key]
	if !ok || value == nil {
		return
	}
	object, ok := asObject(value)
	fieldPath := child(path, key)
	if !ok {
		c.add(fieldPath, "invalid_type", key+" must be an object")
		return
	}
	requireKnownProps(fieldPath, object, c, "address", "phone", "email", "hours")
	validateFooterAddress(fieldPath, object, "address", c)
	optionalString(fieldPath, object, "phone", 40, c)
	optionalString(fieldPath, object, "email", 160, c)
	if email, _ := object["email"].(string); strings.TrimSpace(email) != "" && !validFooterEmail(email) {
		c.add(child(fieldPath, "email"), "invalid_email", "email must be a valid address")
	}
	validateFooterHours(fieldPath, object, "hours", c)
}

// footerWeekdays is the closed set of canonical day keys accepted by the
// structured opening-hours shape. English lowercase keys keep the stored props
// locale-agnostic; the renderer localizes the label and maps to schema.org
// dayOfWeek names for the LocalBusiness JSON-LD emission (Spec 04/09).
var footerWeekdays = []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}

func validateFooterAddress(path string, props map[string]any, key string, c *collector) {
	value, ok := props[key]
	if !ok || value == nil {
		return
	}
	fieldPath := child(path, key)
	// Drafts generated before the structured contact contract (5801843) stored
	// the address as one free-text line. The renderer folds a string address
	// into `street`, so validation must keep those drafts loadable — rejecting
	// them here bricked every pre-contract site on read (Kaffi Krús incident,
	// 2026-07-13). New writes use the structured shape below.
	if legacy, isString := value.(string); isString {
		if len(legacy) > 300 {
			c.add(fieldPath, "invalid_length", "address must be at most 300 characters")
		}
		return
	}
	object, ok := asObject(value)
	if !ok {
		c.add(fieldPath, "invalid_type", key+" must be an object")
		return
	}
	requireKnownProps(fieldPath, object, c, "street", "city", "postalCode", "region", "country")
	optionalString(fieldPath, object, "street", 160, c)
	optionalString(fieldPath, object, "city", 120, c)
	optionalString(fieldPath, object, "postalCode", 40, c)
	optionalString(fieldPath, object, "region", 120, c)
	optionalString(fieldPath, object, "country", 120, c)
}

func validateFooterHours(path string, props map[string]any, key string, c *collector) {
	value, ok := props[key]
	if !ok || value == nil {
		return
	}
	values, ok := asSlice(value)
	fieldPath := child(path, key)
	if !ok {
		c.add(fieldPath, "invalid_type", key+" must be an array")
		return
	}
	if len(values) > 7 {
		c.add(fieldPath, "invalid_length", "hours must include at most 7 entries")
	}
	for index, raw := range values {
		itemPath := fmt.Sprintf("%s[%d]", fieldPath, index)
		// Pre-contract drafts stored hours as free-text lines ("Mán–Fös
		// 08:00–17:00"); the renderer skips them, so validation tolerates them
		// too instead of bricking the draft. New writes use the day-keyed shape.
		if legacy, isString := raw.(string); isString {
			if len(legacy) > 120 {
				c.add(itemPath, "invalid_length", "hours entry must be at most 120 characters")
			}
			continue
		}
		entry, ok := asObject(raw)
		if !ok {
			c.add(itemPath, "invalid_type", "hours entry must be an object")
			continue
		}
		requireKnownProps(itemPath, entry, c, "day", "opens", "closes", "closed")
		day, _ := entry["day"].(string)
		if !isFooterWeekday(day) {
			c.add(child(itemPath, "day"), "invalid_value", "day must be a weekday")
		}
		optionalBool(itemPath, entry, "closed", c)
		validateFooterClockTime(itemPath, entry, "opens", c)
		validateFooterClockTime(itemPath, entry, "closes", c)
	}
}

func isFooterWeekday(value string) bool {
	for _, day := range footerWeekdays {
		if value == day {
			return true
		}
	}
	return false
}

func validateFooterClockTime(path string, props map[string]any, key string, c *collector) {
	value, ok := props[key]
	if !ok || value == nil {
		return
	}
	text, ok := value.(string)
	fieldPath := child(path, key)
	if !ok {
		c.add(fieldPath, "invalid_type", key+" must be a string")
		return
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return
	}
	if !validClockTime(trimmed) {
		c.add(fieldPath, "invalid_value", key+" must be a HH:MM time")
	}
}

func validClockTime(value string) bool {
	if len(value) != 5 || value[2] != ':' {
		return false
	}
	hh := value[0:2]
	mm := value[3:5]
	if !isDigits(hh) || !isDigits(mm) {
		return false
	}
	hours := int(hh[0]-'0')*10 + int(hh[1]-'0')
	minutes := int(mm[0]-'0')*10 + int(mm[1]-'0')
	return hours <= 23 && minutes <= 59
}

func isDigits(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return len(value) > 0
}

func optionalBool(path string, props map[string]any, key string, c *collector) {
	value, ok := props[key]
	if !ok || value == nil {
		return
	}
	if _, ok := value.(bool); !ok {
		c.add(child(path, key), "invalid_type", key+" must be a boolean")
	}
}

func validFooterEmail(value string) bool {
	_, err := mail.ParseAddress(value)
	return err == nil
}

func optionalLinkList(path string, props map[string]any, key string, minItems int, maxItems int, c *collector) {
	value, ok := props[key]
	if !ok || value == nil {
		return
	}
	values, ok := asSlice(value)
	fieldPath := child(path, key)
	if !ok {
		c.add(fieldPath, "invalid_type", key+" must be an array")
		return
	}
	if len(values) < minItems || len(values) > maxItems {
		c.add(fieldPath, "invalid_length", fmt.Sprintf("%s must include between %d and %d entries", key, minItems, maxItems))
	}
	for index, value := range values {
		itemPath := fmt.Sprintf("%s[%d]", fieldPath, index)
		item, ok := asObject(value)
		if !ok {
			c.add(itemPath, "invalid_type", key+" entry must be an object")
			continue
		}
		requireKnownProps(itemPath, item, c, "label", "href")
		requireString(itemPath, item, "label", 1, 40, c)
		href, ok := item["href"].(string)
		if !ok {
			c.add(child(itemPath, "href"), "invalid_type", "href must be a string")
			continue
		}
		if err := ValidateURL(href); err != nil {
			c.add(child(itemPath, "href"), "unsafe_url", err.Error())
		}
	}
}

func validateStatsProps(path string, props map[string]any, c *collector) {
	requireKnownProps(path, props, c, "heading", "intro", "items")
	requireString(path, props, "heading", 1, 120, c)
	optionalString(path, props, "intro", 500, c)

	itemsPath := child(path, "items")
	items, ok := props["items"]
	if !ok {
		c.add(itemsPath, "required", "items is required")
		return
	}
	values, ok := asSlice(items)
	if !ok {
		c.add(itemsPath, "invalid_type", "items must be an array")
		return
	}
	if len(values) < 2 || len(values) > 8 {
		c.add(itemsPath, "invalid_length", "items must include between 2 and 8 entries")
	}
	for index, value := range values {
		itemPath := fmt.Sprintf("%s[%d]", itemsPath, index)
		item, ok := asObject(value)
		if !ok {
			c.add(itemPath, "invalid_type", "stat item must be an object")
			continue
		}
		requireKnownProps(itemPath, item, c, "value", "label", "description")
		requireString(itemPath, item, "value", 1, 32, c)
		requireString(itemPath, item, "label", 1, 80, c)
		optionalString(itemPath, item, "description", 200, c)
	}
}

func validateCollectionListProps(path string, props map[string]any, c *collector) {
	requireKnownProps(path, props, c, "heading", "intro", "collection", "limit", "layout", "showFilters", "cta")
	optionalString(path, props, "heading", 120, c)
	optionalString(path, props, "intro", 500, c)
	if collection, ok := props["collection"].(string); ok {
		if strings.TrimSpace(collection) != "" {
			validateStableID(child(path, "collection"), collection, c)
		}
	} else if value, exists := props["collection"]; exists && value != nil {
		c.add(child(path, "collection"), "invalid_type", "collection must be a string")
	}
	if limit, exists := props["limit"]; exists && limit != nil {
		number, ok := asInt(limit)
		if !ok {
			c.add(child(path, "limit"), "invalid_type", "limit must be an integer")
		} else if number < 1 || number > 50 {
			c.add(child(path, "limit"), "invalid_value", "limit must be between 1 and 50")
		}
	}
	optionalEnum(path, props, "layout", c, "grid", "list")
	if showFilters, exists := props["showFilters"]; exists && showFilters != nil {
		if _, ok := showFilters.(bool); !ok {
			c.add(child(path, "showFilters"), "invalid_type", "showFilters must be true or false")
		}
	}
	optionalCTA(path, props, "cta", c)
}

func validateCollectionIndexProps(path string, props map[string]any, c *collector) {
	requireKnownProps(path, props, c, "heading", "intro", "sort", "layout", "showSort", "showFilters")
	optionalString(path, props, "heading", 120, c)
	optionalString(path, props, "intro", 500, c)
	optionalEnum(path, props, "sort", c, "manual", "newest", "oldest", "title")
	optionalEnum(path, props, "layout", c, "grid", "list")
	for _, key := range []string{"showSort", "showFilters"} {
		if value, exists := props[key]; exists && value != nil {
			if _, ok := value.(bool); !ok {
				c.add(child(path, key), "invalid_type", key+" must be true or false")
			}
		}
	}
}

func validateCollectionDetailProps(path string, props map[string]any, c *collector) {
	requireKnownProps(path, props, c, "heading", "layout")
	optionalString(path, props, "heading", 120, c)
	optionalEnum(path, props, "layout", c, "default", "narrow", "wide")
}

func asObject(value any) (map[string]any, bool) {
	if object, ok := value.(map[string]any); ok {
		return object, true
	}
	return nil, false
}

func asSlice(value any) ([]any, bool) {
	if values, ok := value.([]any); ok {
		return values, true
	}
	return nil, false
}

func asInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		if typed == float64(int(typed)) {
			return int(typed), true
		}
	}
	return 0, false
}
