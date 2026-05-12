package siteconfig

import (
	"fmt"
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

func FormDefinitionFromProps(props map[string]any) (FormDefinition, error) {
	definition := FormDefinition{}
	if props == nil {
		return definition, ValidationError{Issues: []Issue{{
			Path:    "form.fields",
			Code:    "required",
			Message: "form must include at least one field",
		}}}
	}

	fieldsValue, ok := props["fields"]
	if !ok {
		return definition, ValidationError{Issues: []Issue{{
			Path:    "form.fields",
			Code:    "required",
			Message: "form must include at least one field",
		}}}
	}

	fields, ok := asSlice(fieldsValue)
	if !ok {
		return definition, ValidationError{Issues: []Issue{{
			Path:    "form.fields",
			Code:    "invalid_type",
			Message: "fields must be an array",
		}}}
	}

	definition.Fields = make([]FormField, 0, len(fields))
	for index, value := range fields {
		fieldObject, ok := asObject(value)
		if !ok {
			return definition, ValidationError{Issues: []Issue{{
				Path:    fmt.Sprintf("form.fields[%d]", index),
				Code:    "invalid_type",
				Message: "field must be an object",
			}}}
		}

		field := FormField{}
		if text, ok := fieldObject["name"].(string); ok {
			field.Name = text
		}
		if text, ok := fieldObject["label"].(string); ok {
			field.Label = text
		}
		if text, ok := fieldObject["type"].(string); ok {
			field.Type = text
		}
		if required, ok := fieldObject["required"].(bool); ok {
			field.Required = required
		}
		if optionsValue, ok := fieldObject["options"]; ok {
			options, ok := asSlice(optionsValue)
			if !ok {
				return definition, ValidationError{Issues: []Issue{{
					Path:    fmt.Sprintf("form.fields[%d].options", index),
					Code:    "invalid_type",
					Message: "options must be an array",
				}}}
			}
			field.Options = make([]string, 0, len(options))
			for optionIndex, optionValue := range options {
				text, ok := optionValue.(string)
				if !ok {
					return definition, ValidationError{Issues: []Issue{{
						Path:    fmt.Sprintf("form.fields[%d].options[%d]", index, optionIndex),
						Code:    "invalid_type",
						Message: "option must be a string",
					}}}
				}
				field.Options = append(field.Options, text)
			}
		}

		definition.Fields = append(definition.Fields, field)
	}

	if text, ok := props["successMessage"].(string); ok {
		definition.SuccessMessage = text
	}
	if text, ok := props["notificationEmail"].(string); ok {
		definition.NotificationEmail = text
	}

	if err := ValidateFormDefinition(definition); err != nil {
		return FormDefinition{}, err
	}

	return definition, nil
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
