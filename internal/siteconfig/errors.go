package siteconfig

import (
	"fmt"
	"strings"
)

type Issue struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ValidationError struct {
	Issues []Issue `json:"issues"`
}

func (e ValidationError) Error() string {
	if len(e.Issues) == 0 {
		return "validation failed"
	}
	first := e.Issues[0]
	return fmt.Sprintf("validation failed at %s: %s", first.Path, first.Message)
}

func (e ValidationError) Has(code string) bool {
	for _, issue := range e.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

type collector struct {
	issues []Issue
}

func (c *collector) add(path string, code string, message string) {
	c.issues = append(c.issues, Issue{
		Path:    path,
		Code:    code,
		Message: message,
	})
}

func (c *collector) err() error {
	if len(c.issues) == 0 {
		return nil
	}
	return ValidationError{Issues: c.issues}
}

func child(path string, name string) string {
	if path == "" {
		return name
	}
	if strings.HasPrefix(name, "[") {
		return path + name
	}
	return path + "." + name
}
