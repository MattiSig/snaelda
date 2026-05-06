package ids

import (
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
)

var (
	ErrInvalid = errors.New("id must be a valid UUID")

	uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

func New() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}

	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4],
		value[4:6],
		value[6:8],
		value[8:10],
		value[10:16],
	), nil
}

func MustNew() string {
	value, err := New()
	if err != nil {
		panic(err)
	}
	return value
}

func Validate(value string) error {
	if !IsValid(value) {
		return ErrInvalid
	}
	return nil
}

func IsValid(value string) bool {
	return uuidPattern.MatchString(value)
}
