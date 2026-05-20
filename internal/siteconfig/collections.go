package siteconfig

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/MattiSig/snaelda/internal/platform/slugs"
)

// Collection field-type registry. The set is closed: AI and the editor may
// only choose from these types.
const (
	FieldTypeText      = "text"
	FieldTypeLongText  = "long_text"
	FieldTypeRichText  = "rich_text"
	FieldTypeNumber    = "number"
	FieldTypeBoolean   = "boolean"
	FieldTypeDate      = "date"
	FieldTypeURL       = "url"
	FieldTypeEmail     = "email"
	FieldTypePhone     = "phone"
	FieldTypeLocation  = "location"
	FieldTypeEnum      = "enum"
	FieldTypeEnumMulti = "enum_multi"
	FieldTypeAsset     = "asset"
	FieldTypeAssetList = "asset_list"
	FieldTypeReference = "reference"
)

const (
	EntryStatusDraft     = "draft"
	EntryStatusPublished = "published"
)

const (
	MaxCollectionFields  = 25
	MaxFieldOptions      = 25
	MaxCollectionEntries = 500
)

var supportedFieldTypes = set(
	FieldTypeText,
	FieldTypeLongText,
	FieldTypeRichText,
	FieldTypeNumber,
	FieldTypeBoolean,
	FieldTypeDate,
	FieldTypeURL,
	FieldTypeEmail,
	FieldTypePhone,
	FieldTypeLocation,
	FieldTypeEnum,
	FieldTypeEnumMulti,
	FieldTypeAsset,
	FieldTypeAssetList,
	FieldTypeReference,
)

// SupportedFieldTypes returns the registry of allowed collection field
// types, in declaration order.
func SupportedFieldTypes() []string {
	return []string{
		FieldTypeText,
		FieldTypeLongText,
		FieldTypeRichText,
		FieldTypeNumber,
		FieldTypeBoolean,
		FieldTypeDate,
		FieldTypeURL,
		FieldTypeEmail,
		FieldTypePhone,
		FieldTypeLocation,
		FieldTypeEnum,
		FieldTypeEnumMulti,
		FieldTypeAsset,
		FieldTypeAssetList,
		FieldTypeReference,
	}
}

var (
	fieldKeyPattern  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
	isoDatePattern   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	phonePattern     = regexp.MustCompile(`^[+0-9 ()\-\.]{3,32}$`)
	locationKeys     = set("name", "region", "country", "lat", "lng")
	textPropTypes    = set(FieldTypeText, FieldTypeLongText, FieldTypeRichText, FieldTypeURL, FieldTypeEmail, FieldTypePhone)
	bindablePropKeys = map[string]bool{}
)

// Collection is a user-defined typed list of structured entries (services,
// projects, menu items, etc.) owned by a site.
type Collection struct {
	ID            string              `json:"id"`
	Slug          string              `json:"slug"`
	SingularLabel string              `json:"singularLabel"`
	PluralLabel   string              `json:"pluralLabel"`
	Schema        []FieldDefinition   `json:"schema"`
	Settings      CollectionSettings  `json:"settings,omitempty"`
	SortOrder     int                 `json:"sortOrder"`
	Entries       []CollectionEntry   `json:"entries,omitempty"`
}

// CollectionSettings holds collection-level options. Kept narrow on purpose
// so the runtime contract is the single source of truth.
type CollectionSettings struct {
	DefaultSort      string `json:"defaultSort,omitempty"`
	ExposeDetailURLs bool   `json:"exposeDetailUrls,omitempty"`
}

// FieldDefinition describes a single typed field on a collection.
type FieldDefinition struct {
	Key          string           `json:"key"`
	Label        string           `json:"label"`
	Type         string           `json:"type"`
	Required     bool             `json:"required,omitempty"`
	Description  string           `json:"description,omitempty"`
	Options      []string         `json:"options,omitempty"`
	DefaultValue any              `json:"defaultValue,omitempty"`
	Validation   *FieldValidation `json:"validation,omitempty"`
}

// FieldValidation captures per-type constraints. Optional on every field;
// validators apply sensible defaults.
type FieldValidation struct {
	MinLength *int     `json:"minLength,omitempty"`
	MaxLength *int     `json:"maxLength,omitempty"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
}

// CollectionEntry is one row matching a collection's schema. Entries are
// normalized — they are not stored as opaque blobs on the collection.
type CollectionEntry struct {
	ID        string         `json:"id"`
	Slug      string         `json:"slug"`
	Fields    map[string]any `json:"fields"`
	SEO       SEOConfig      `json:"seo,omitempty"`
	Status    string         `json:"status,omitempty"`
	SortOrder int            `json:"sortOrder"`
}

// ValidateCollection validates a single collection definition and its
// entries against the field-type registry. Callers are responsible for
// validating uniqueness against other collections via the site-wide validator.
func ValidateCollection(collection Collection) error {
	var c collector
	validateCollection("collection", collection, &c)
	return c.err()
}

func validateCollections(path string, collections []Collection, c *collector) map[string]Collection {
	byID := map[string]Collection{}
	seenSlugs := map[string]bool{}
	for index, collection := range collections {
		colPath := fmt.Sprintf("%s[%d]", path, index)
		validateStableID(child(colPath, "id"), collection.ID, c)
		if collection.ID != "" {
			if _, exists := byID[collection.ID]; exists {
				c.add(child(colPath, "id"), "duplicate_id", "collection id must be unique")
			}
			byID[collection.ID] = collection
		}
		if collection.Slug == "" || !validCollectionSlug(collection.Slug) {
			c.add(child(colPath, "slug"), "invalid_slug", "collection slug must be lowercase words separated by hyphens")
		} else {
			if seenSlugs[collection.Slug] {
				c.add(child(colPath, "slug"), "duplicate_slug", "collection slug must be unique within the site")
			}
			seenSlugs[collection.Slug] = true
		}
		validateRequiredText(child(colPath, "singularLabel"), collection.SingularLabel, 1, 60, c)
		validateRequiredText(child(colPath, "pluralLabel"), collection.PluralLabel, 1, 60, c)
		validateCollection(colPath, collection, c)
	}
	return byID
}

func validateCollection(path string, collection Collection, c *collector) {
	if len(collection.Schema) == 0 {
		c.add(child(path, "schema"), "required", "collection schema must include at least one field")
	}
	if len(collection.Schema) > MaxCollectionFields {
		c.add(child(path, "schema"), "too_many_fields", fmt.Sprintf("collection schema cannot include more than %d fields", MaxCollectionFields))
	}

	fieldsByKey := map[string]FieldDefinition{}
	for index, field := range collection.Schema {
		fieldPath := fmt.Sprintf("%s.schema[%d]", path, index)
		validateFieldDefinition(fieldPath, field, c)
		if field.Key != "" {
			if _, dup := fieldsByKey[field.Key]; dup {
				c.add(child(fieldPath, "key"), "duplicate_key", "field key must be unique within a collection")
			}
			fieldsByKey[field.Key] = field
		}
	}

	if len(collection.Entries) > MaxCollectionEntries {
		c.add(child(path, "entries"), "too_many_entries", fmt.Sprintf("collection cannot include more than %d entries", MaxCollectionEntries))
	}
	seenEntrySlugs := map[string]bool{}
	seenEntryIDs := map[string]bool{}
	for index, entry := range collection.Entries {
		entryPath := fmt.Sprintf("%s.entries[%d]", path, index)
		validateStableID(child(entryPath, "id"), entry.ID, c)
		if entry.ID != "" {
			if seenEntryIDs[entry.ID] {
				c.add(child(entryPath, "id"), "duplicate_id", "entry id must be unique within a collection")
			}
			seenEntryIDs[entry.ID] = true
		}
		if entry.Slug == "" || !slugs.IsValid(entry.Slug) {
			c.add(child(entryPath, "slug"), "invalid_slug", "entry slug must be lowercase words separated by hyphens")
		} else {
			if seenEntrySlugs[entry.Slug] {
				c.add(child(entryPath, "slug"), "duplicate_slug", "entry slug must be unique within a collection")
			}
			seenEntrySlugs[entry.Slug] = true
		}
		switch entry.Status {
		case "", EntryStatusDraft, EntryStatusPublished:
		default:
			c.add(child(entryPath, "status"), "invalid_value", "entry status must be draft or published")
		}
		validateSEO(child(entryPath, "seo"), entry.SEO, false, c)
		validateEntryFields(child(entryPath, "fields"), entry.Fields, collection.Schema, c)
	}
}

func validateFieldDefinition(path string, field FieldDefinition, c *collector) {
	if field.Key == "" {
		c.add(child(path, "key"), "required", "field key is required")
	} else if !fieldKeyPattern.MatchString(field.Key) {
		c.add(child(path, "key"), "invalid_key", "field key must be snake_case starting with a letter")
	}
	validateRequiredText(child(path, "label"), field.Label, 1, 60, c)
	if !supportedFieldTypes[field.Type] {
		c.add(child(path, "type"), "unknown_field_type", "field type is not supported")
	}
	if field.Description != "" {
		validateStringLength(child(path, "description"), strings.TrimSpace(field.Description), 0, 200, c)
		validatePlainText(child(path, "description"), field.Description, c)
	}
	switch field.Type {
	case FieldTypeEnum, FieldTypeEnumMulti:
		if len(field.Options) == 0 {
			c.add(child(path, "options"), "required", "enum field must declare its options up front")
		}
		if len(field.Options) > MaxFieldOptions {
			c.add(child(path, "options"), "too_many_options", fmt.Sprintf("enum field cannot include more than %d options", MaxFieldOptions))
		}
		seen := map[string]bool{}
		for index, option := range field.Options {
			optionPath := fmt.Sprintf("%s.options[%d]", path, index)
			trimmed := strings.TrimSpace(option)
			if trimmed == "" {
				c.add(optionPath, "required", "enum option cannot be empty")
				continue
			}
			if len(trimmed) > 60 {
				c.add(optionPath, "invalid_length", "enum option is too long")
			}
			validatePlainText(optionPath, trimmed, c)
			if seen[trimmed] {
				c.add(optionPath, "duplicate_value", "enum options must be unique")
			}
			seen[trimmed] = true
		}
	default:
		if len(field.Options) > 0 {
			c.add(child(path, "options"), "invalid_value", "options are only valid on enum fields")
		}
	}
}

func validateEntryFields(path string, fields map[string]any, schema []FieldDefinition, c *collector) {
	known := map[string]FieldDefinition{}
	for _, field := range schema {
		known[field.Key] = field
	}
	for key := range fields {
		if _, ok := known[key]; !ok {
			c.add(child(path, key), "unknown_property", "field is not part of the collection schema")
		}
	}
	for _, field := range schema {
		value, ok := fields[field.Key]
		fieldPath := child(path, field.Key)
		if !ok || value == nil {
			if field.Required {
				c.add(fieldPath, "required", "field is required")
			}
			continue
		}
		validateFieldValue(fieldPath, value, field, c)
	}
}

func validateFieldValue(path string, value any, field FieldDefinition, c *collector) {
	switch field.Type {
	case FieldTypeText, FieldTypeLongText:
		text, ok := value.(string)
		if !ok {
			c.add(path, "invalid_type", "value must be a string")
			return
		}
		maxLen := 240
		if field.Type == FieldTypeLongText {
			maxLen = 4000
		}
		validateStringLength(path, strings.TrimSpace(text), 0, applyMax(field, maxLen), c)
		validatePlainText(path, text, c)
	case FieldTypeRichText:
		text, ok := value.(string)
		if !ok {
			c.add(path, "invalid_type", "value must be a string")
			return
		}
		validateStringLength(path, strings.TrimSpace(text), 0, applyMax(field, 8000), c)
	case FieldTypeNumber:
		number, ok := numericValue(value)
		if !ok {
			c.add(path, "invalid_type", "value must be a number")
			return
		}
		if field.Validation != nil {
			if field.Validation.Min != nil && number < *field.Validation.Min {
				c.add(path, "invalid_value", "value is below minimum")
			}
			if field.Validation.Max != nil && number > *field.Validation.Max {
				c.add(path, "invalid_value", "value is above maximum")
			}
		}
	case FieldTypeBoolean:
		if _, ok := value.(bool); !ok {
			c.add(path, "invalid_type", "value must be true or false")
		}
	case FieldTypeDate:
		text, ok := value.(string)
		if !ok {
			c.add(path, "invalid_type", "value must be an ISO date string")
			return
		}
		if !isoDatePattern.MatchString(text) {
			c.add(path, "invalid_value", "value must be a YYYY-MM-DD date")
		}
	case FieldTypeURL:
		text, ok := value.(string)
		if !ok {
			c.add(path, "invalid_type", "value must be a string")
			return
		}
		if err := ValidateURL(text); err != nil {
			c.add(path, "unsafe_url", err.Error())
		}
	case FieldTypeEmail:
		text, ok := value.(string)
		if !ok {
			c.add(path, "invalid_type", "value must be a string")
			return
		}
		if err := ValidateURL("mailto:" + text); err != nil {
			c.add(path, "invalid_value", "value must be a valid email address")
		}
	case FieldTypePhone:
		text, ok := value.(string)
		if !ok {
			c.add(path, "invalid_type", "value must be a string")
			return
		}
		if !phonePattern.MatchString(strings.TrimSpace(text)) {
			c.add(path, "invalid_value", "value must be a phone number")
		}
	case FieldTypeLocation:
		object, ok := asObject(value)
		if !ok {
			c.add(path, "invalid_type", "value must be a location object")
			return
		}
		for key := range object {
			if !locationKeys[key] {
				c.add(child(path, key), "unknown_property", "location key is not supported")
			}
		}
		name, _ := object["name"].(string)
		if strings.TrimSpace(name) == "" {
			c.add(child(path, "name"), "required", "location name is required")
		}
	case FieldTypeEnum:
		text, ok := value.(string)
		if !ok {
			c.add(path, "invalid_type", "value must be a string")
			return
		}
		if !containsOption(field.Options, text) {
			c.add(path, "invalid_value", "value is not in the declared options")
		}
	case FieldTypeEnumMulti:
		values, ok := asSlice(value)
		if !ok {
			c.add(path, "invalid_type", "value must be an array")
			return
		}
		seen := map[string]bool{}
		for index, item := range values {
			text, ok := item.(string)
			if !ok {
				c.add(fmt.Sprintf("%s[%d]", path, index), "invalid_type", "value must be a string")
				continue
			}
			if !containsOption(field.Options, text) {
				c.add(fmt.Sprintf("%s[%d]", path, index), "invalid_value", "value is not in the declared options")
			}
			if seen[text] {
				c.add(fmt.Sprintf("%s[%d]", path, index), "duplicate_value", "value must not repeat")
			}
			seen[text] = true
		}
	case FieldTypeAsset:
		validateAssetValue(path, value, c)
	case FieldTypeAssetList:
		values, ok := asSlice(value)
		if !ok {
			c.add(path, "invalid_type", "value must be an array of assets")
			return
		}
		for index, item := range values {
			validateAssetValue(fmt.Sprintf("%s[%d]", path, index), item, c)
		}
	case FieldTypeReference:
		object, ok := asObject(value)
		if !ok {
			c.add(path, "invalid_type", "value must be a reference object")
			return
		}
		collectionID, _ := object["collectionId"].(string)
		entryID, _ := object["entryId"].(string)
		if collectionID == "" {
			c.add(child(path, "collectionId"), "required", "reference collectionId is required")
		}
		if entryID == "" {
			c.add(child(path, "entryId"), "required", "reference entryId is required")
		}
	default:
		c.add(path, "unknown_field_type", "field type is not supported")
	}
}

func validateAssetValue(path string, value any, c *collector) {
	object, ok := asObject(value)
	if !ok {
		c.add(path, "invalid_type", "value must be an asset reference")
		return
	}
	for key := range object {
		switch key {
		case "assetId", "alt":
		default:
			c.add(child(path, key), "unknown_property", "asset key is not supported")
		}
	}
	assetID, ok := object["assetId"].(string)
	if !ok || strings.TrimSpace(assetID) == "" {
		c.add(child(path, "assetId"), "required", "assetId is required")
	}
	if alt, ok := object["alt"].(string); ok {
		validateStringLength(child(path, "alt"), strings.TrimSpace(alt), 0, 180, c)
		validatePlainText(child(path, "alt"), alt, c)
	}
}

// validateBindings asserts that any binding on a block targets a real entry
// field and that the field type is compatible with the bound prop.
func validateBindings(path string, block BlockInstance, page PageDraft, collectionsByID map[string]Collection, c *collector) {
	if len(block.Bindings) == 0 {
		return
	}
	if page.Type != PageTypeCollectionDetail {
		c.add(child(path, "bindings"), "invalid_binding", "bindings are only valid in collection_detail templates")
		return
	}
	collection, ok := collectionsByID[page.CollectionID]
	if !ok {
		c.add(child(path, "bindings"), "invalid_binding", "binding target collection does not exist")
		return
	}
	fieldsByKey := map[string]FieldDefinition{}
	for _, field := range collection.Schema {
		fieldsByKey[field.Key] = field
	}
	for propKey, binding := range block.Bindings {
		bindingPath := child(child(path, "bindings"), propKey)
		if binding.Source != BlockBindingSourceEntry {
			c.add(child(bindingPath, "source"), "invalid_value", "binding source must be entry")
			continue
		}
		field, ok := fieldsByKey[binding.Field]
		if !ok {
			c.add(child(bindingPath, "field"), "unresolved_reference", "binding targets a field that does not exist on the collection")
			continue
		}
		if !propKeyIsBindable(propKey) {
			c.add(bindingPath, "invalid_binding", "this block prop cannot be bound")
			continue
		}
		if !bindingTypeMatches(propKey, field.Type) {
			c.add(bindingPath, "type_mismatch", "binding target field type is incompatible with this prop")
		}
	}
}

// propKeyIsBindable allows a conservative starter set of bindable prop
// names. Block authors opt prop names into this list as they ship binding
// support for their block.
func propKeyIsBindable(prop string) bool {
	if bindablePropKeys[prop] {
		return true
	}
	switch prop {
	case "headline", "subheadline", "eyebrow", "heading", "body", "title", "summary", "description", "image", "cover", "gallery", "items", "href", "url", "phone", "email":
		return true
	}
	return false
}

func bindingTypeMatches(propKey string, fieldType string) bool {
	switch propKey {
	case "image", "cover":
		return fieldType == FieldTypeAsset
	case "gallery":
		return fieldType == FieldTypeAssetList
	case "items":
		return fieldType == FieldTypeAssetList || fieldType == FieldTypeEnumMulti
	case "href", "url":
		return fieldType == FieldTypeURL
	case "phone":
		return fieldType == FieldTypePhone
	case "email":
		return fieldType == FieldTypeEmail
	default:
		return textPropTypes[fieldType]
	}
}

func validCollectionSlug(value string) bool {
	if value == "" {
		return false
	}
	return slugs.IsValid(value)
}

func containsOption(options []string, value string) bool {
	for _, option := range options {
		if option == value {
			return true
		}
	}
	return false
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	}
	return 0, false
}

func applyMax(field FieldDefinition, fallback int) int {
	if field.Validation != nil && field.Validation.MaxLength != nil && *field.Validation.MaxLength > 0 {
		return *field.Validation.MaxLength
	}
	return fallback
}
