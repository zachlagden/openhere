package validate

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type Error struct {
	Message string
}

func (e *Error) Error() string { return e.Message }

func Invalid(message string) error { return &Error{Message: message} }

func Line(field, raw string, max int) (string, error) {
	value := strings.TrimSpace(raw)
	length := utf8.RuneCountInString(value)
	if length == 0 || length > max {
		return "", Invalid(fmt.Sprintf("%s must be 1 to %d characters long.", field, max))
	}
	for _, r := range value {
		if isControl(r) {
			return "", Invalid(field + " must not contain control characters.")
		}
	}
	return value, nil
}

func Text(field, raw string, max int) (string, error) {
	value := strings.TrimSpace(raw)
	if utf8.RuneCountInString(value) > max {
		return "", Invalid(fmt.Sprintf("%s must be at most %d characters long.", field, max))
	}
	for _, r := range value {
		if isControl(r) && r != '\n' && r != '\t' {
			return "", Invalid(field + " must not contain control characters.")
		}
	}
	return value, nil
}

func isControl(r rune) bool {
	return r < 0x20 || r == 0x7f
}
