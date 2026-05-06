package siteconfig

import (
	"net/mail"
	"regexp"
)

var (
	supportedFormFieldTypes = set("name", "email", "phone", "message", "select")
	formFieldNamePattern    = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
)

func ValidateFormDefinition(definition FormDefinition) error {
	var c collector
	validateFormDefinition("form", definition, &c)
	return c.err()
}

func validateFormDefinition(path string, definition FormDefinition, c *collector) {
	if len(definition.Fields) == 0 {
		c.add(child(path, "fields"), "required", "form must include at least one field")
	}
	if len(definition.Fields) > 8 {
		c.add(child(path, "fields"), "invalid_length", "form cannot include more than 8 fields")
	}
	if definition.SuccessMessage != "" {
		validateRequiredText(child(path, "successMessage"), definition.SuccessMessage, 1, 200, c)
	}
	if definition.NotificationEmail != "" {
		if _, err := mail.ParseAddress(definition.NotificationEmail); err != nil {
			c.add(child(path, "notificationEmail"), "invalid_email", "notification email is invalid")
		}
	}

	seen := map[string]bool{}
	for index, field := range definition.Fields {
		fieldPath := child(child(path, "fields"), "["+itoa(index)+"]")
		validateRequiredText(child(fieldPath, "label"), field.Label, 1, 80, c)
		if !formFieldNamePattern.MatchString(field.Name) {
			c.add(child(fieldPath, "name"), "invalid_name", "field name must use lowercase letters, numbers, and underscores")
		}
		if seen[field.Name] {
			c.add(child(fieldPath, "name"), "duplicate_name", "field names must be unique")
		}
		seen[field.Name] = true
		if !supportedFormFieldTypes[field.Type] {
			c.add(child(fieldPath, "type"), "unsupported_field", "field type is not supported")
		}
		if field.Type == "select" && len(field.Options) == 0 {
			c.add(child(fieldPath, "options"), "required", "select fields must include options")
		}
		if len(field.Options) > 20 {
			c.add(child(fieldPath, "options"), "invalid_length", "select fields cannot include more than 20 options")
		}
		for optionIndex, option := range field.Options {
			validateRequiredText(child(child(fieldPath, "options"), "["+itoa(optionIndex)+"]"), option, 1, 80, c)
		}
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}
