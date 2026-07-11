package respin

import (
	"errors"
	"fmt"
)

// FetchStatusError is returned when the source responds with an HTTP error
// status. The pipeline treats it as a degradation signal.
type FetchStatusError struct {
	StatusCode int
	URL        string
}

func (e *FetchStatusError) Error() string {
	return fmt.Sprintf("respin: source returned status %d for %s", e.StatusCode, e.URL)
}

// ContentTypeError is returned when a page fetch yields a non-HTML body.
type ContentTypeError struct {
	ContentType string
	URL         string
}

func (e *ContentTypeError) Error() string {
	return fmt.Sprintf("respin: unexpected content type %q for %s", e.ContentType, e.URL)
}

// ErrInsufficientContent signals that a plain fetch did not yield enough
// readable content to re-spin from; the caller degrades to the prompt flow.
var ErrInsufficientContent = errors.New("respin: source content is insufficient for a plain fetch")
