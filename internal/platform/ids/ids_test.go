package ids

import (
	"errors"
	"testing"
)

func TestNewReturnsValidUUID(t *testing.T) {
	value, err := New()
	if err != nil {
		t.Fatalf("new id: %v", err)
	}

	if !IsValid(value) {
		t.Fatalf("expected valid uuid, got %q", value)
	}
}

func TestValidateRejectsInvalidID(t *testing.T) {
	err := Validate("site-1")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected invalid id error, got %v", err)
	}
}
