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
			{Name: "image", Label: "Image", Control: "asset"},
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
			{Name: "items", Label: "Items", Control: "repeater"},
			{Name: "columns", Label: "Columns", Control: "select", Options: []string{"2", "3", "4"}},
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
			{Name: "cta", Label: "CTA", Control: "link"},
			{Name: "variant", Label: "Variant", Control: "select", Options: []string{"primary", "secondary", "accent"}},
		},
		ValidateProps: validateCTABandProps,
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
	validateStringLength(fieldPath, strings.TrimSpace(text), minLength, maxLength, c)
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
	validateStringLength(fieldPath, strings.TrimSpace(text), 0, maxLength, c)
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
