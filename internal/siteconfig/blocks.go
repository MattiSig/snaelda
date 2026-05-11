package siteconfig

import (
	"fmt"
	"strings"
)

const BlockVersionV1 = "1.0.0"

func heroBlockDefinition() BlockDefinition {
	return BlockDefinition{
		Type:        "hero",
		Version:     BlockVersionV1,
		DisplayName: "Hero",
		Category:    BlockCategoryHero,
		DefaultProps: map[string]any{
			"headline": "A focused website starts here",
			"layout":   "centered",
		},
		EditorSchema: []EditorField{
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
			{Name: "layout", Label: "Layout", Control: "select", Options: []string{"centered", "split-left", "split-right"}},
		},
		ValidateProps: validateHeroProps,
	}
}

func textSectionBlockDefinition() BlockDefinition {
	return BlockDefinition{
		Type:        "text_section",
		Version:     BlockVersionV1,
		DisplayName: "Text section",
		Category:    BlockCategoryContent,
		DefaultProps: map[string]any{
			"heading":   "About",
			"body":      "Add focused supporting copy here.",
			"alignment": "left",
			"width":     "default",
		},
		EditorSchema: []EditorField{
			{Name: "heading", Label: "Heading", Control: "text"},
			{Name: "body", Label: "Body", Control: "textarea"},
			{Name: "alignment", Label: "Alignment", Control: "select", Options: []string{"left", "center", "right"}},
			{Name: "width", Label: "Width", Control: "select", Options: []string{"narrow", "default", "wide"}},
		},
		ValidateProps: validateTextSectionProps,
	}
}

func imageTextBlockDefinition() BlockDefinition {
	return BlockDefinition{
		Type:        "image_text",
		Version:     BlockVersionV1,
		DisplayName: "Image and text",
		Category:    BlockCategoryMedia,
		DefaultProps: map[string]any{
			"heading":       "Built with structure",
			"body":          "Pair a short message with a supporting image.",
			"imagePosition": "right",
		},
		EditorSchema: []EditorField{
			{Name: "heading", Label: "Heading", Control: "text"},
			{Name: "body", Label: "Body", Control: "textarea"},
			{
				Name:        "image",
				Label:       "Image",
				Control:     "asset",
				Description: "Assets are stored by id once uploads are wired in.",
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
		},
		ValidateProps: validateImageTextProps,
	}
}

func featuresGridBlockDefinition() BlockDefinition {
	return BlockDefinition{
		Type:        "features_grid",
		Version:     BlockVersionV1,
		DisplayName: "Features grid",
		Category:    BlockCategoryContent,
		DefaultProps: map[string]any{
			"heading": "What you get",
			"items": []any{
				map[string]any{"title": "Fast", "body": "A concise benefit statement."},
			},
			"columns": 3,
		},
		EditorSchema: []EditorField{
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
		},
		ValidateProps: validateFeaturesGridProps,
	}
}

func ctaBandBlockDefinition() BlockDefinition {
	return BlockDefinition{
		Type:        "cta_band",
		Version:     BlockVersionV1,
		DisplayName: "CTA band",
		Category:    BlockCategoryConversion,
		DefaultProps: map[string]any{
			"heading": "Ready to begin?",
			"body":    "Invite visitors into the next step.",
			"variant": "primary",
		},
		EditorSchema: []EditorField{
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
		},
		ValidateProps: validateCTABandProps,
	}
}

func galleryBlockDefinition() BlockDefinition {
	return BlockDefinition{
		Type:        "gallery",
		Version:     BlockVersionV1,
		DisplayName: "Gallery",
		Category:    BlockCategoryMedia,
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
		EditorSchema: []EditorField{
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
						Description: "Optional for now. Preview falls back to a crafted placeholder until uploads are wired in.",
					},
				},
			},
		},
		ValidateProps: validateGalleryProps,
	}
}

func testimonialsBlockDefinition() BlockDefinition {
	return BlockDefinition{
		Type:        "testimonials",
		Version:     BlockVersionV1,
		DisplayName: "Testimonials",
		Category:    BlockCategoryContent,
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
		EditorSchema: []EditorField{
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
		},
		ValidateProps: validateTestimonialsProps,
	}
}

func pricingPackagesBlockDefinition() BlockDefinition {
	return BlockDefinition{
		Type:        "pricing_packages",
		Version:     BlockVersionV1,
		DisplayName: "Pricing packages",
		Category:    BlockCategoryConversion,
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
		EditorSchema: []EditorField{
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
		},
		ValidateProps: validatePricingPackagesProps,
	}
}

func faqBlockDefinition() BlockDefinition {
	return BlockDefinition{
		Type:        "faq",
		Version:     BlockVersionV1,
		DisplayName: "FAQ",
		Category:    BlockCategoryContent,
		DefaultProps: map[string]any{
			"heading": "Questions people ask first",
			"items": []any{
				map[string]any{
					"question": "What should someone know before they reach out?",
					"answer":   "Use this answer to reduce hesitation and set expectations clearly.",
				},
			},
		},
		EditorSchema: []EditorField{
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
		},
		ValidateProps: validateFAQProps,
	}
}

func teamProfileCardsBlockDefinition() BlockDefinition {
	return BlockDefinition{
		Type:        "team_profile_cards",
		Version:     BlockVersionV1,
		DisplayName: "Team profile cards",
		Category:    BlockCategoryContent,
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
		EditorSchema: []EditorField{
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
		},
		ValidateProps: validateTeamProfileCardsProps,
	}
}

func footerBlockDefinition() BlockDefinition {
	return BlockDefinition{
		Type:        "footer",
		Version:     BlockVersionV1,
		DisplayName: "Footer",
		Category:    BlockCategoryContent,
		DefaultProps: map[string]any{
			"siteName":  "Site name",
			"tagline":   "A short closing line that reinforces the tone of the site.",
			"copyright": "Copyright 2026 Site name",
			"navigationLinks": []any{
				map[string]any{
					"label": "Contact",
					"href":  "/contact",
				},
			},
		},
		EditorSchema: []EditorField{
			{Name: "siteName", Label: "Site name", Control: "text"},
			{Name: "tagline", Label: "Tagline", Control: "textarea"},
			{Name: "contactLine", Label: "Contact line", Control: "text"},
			{Name: "copyright", Label: "Copyright", Control: "text"},
			{
				Name:    "navigationLinks",
				Label:   "Navigation links",
				Control: "repeater",
				ItemFields: []EditorField{
					{Name: "label", Label: "Label", Control: "text"},
					{Name: "href", Label: "Link", Control: "text", Placeholder: "/contact"},
				},
			},
			{
				Name:    "socialLinks",
				Label:   "Social links",
				Control: "repeater",
				ItemFields: []EditorField{
					{Name: "label", Label: "Label", Control: "text"},
					{Name: "href", Label: "Link", Control: "text", Placeholder: "https://instagram.com/example"},
				},
			},
		},
		ValidateProps: validateFooterProps,
	}
}

func validateHeroProps(path string, props map[string]any, c *collector) {
	requireKnownProps(path, props, c, "eyebrow", "headline", "subheadline", "primaryCta", "secondaryCta", "image", "layout")
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
	requireKnownProps(path, props, c, "siteName", "tagline", "contactLine", "copyright", "navigationLinks", "socialLinks")
	requireString(path, props, "siteName", 1, 120, c)
	optionalString(path, props, "tagline", 240, c)
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
	optionalString(fieldPath, object, "alt", 180, c)
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
