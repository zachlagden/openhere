package auth

import (
	"net/mail"
	"strings"
	"unicode/utf8"
)

const (
	MinPasswordLength = 12
	MaxPasswordLength = 128
	MaxEmailLength    = 254
	MaxLabelLength    = 64
)

func ValidatePassword(password string) error {
	length := utf8.RuneCountInString(password)
	if length < MinPasswordLength {
		return invalid("Password must be at least 12 characters long.")
	}
	if length > MaxPasswordLength {
		return invalid("Password must be at most 128 characters long.")
	}
	return nil
}

func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || len(email) > MaxEmailLength || !isASCII(email) {
		return "", invalid("Enter a valid email address.")
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email[strings.LastIndex(email, "@"):], ".") {
		return "", invalid("Enter a valid email address.")
	}
	return email, nil
}

func NormalizeLabel(raw string) (string, error) {
	label := strings.TrimSpace(raw)
	if label == "" || utf8.RuneCountInString(label) > MaxLabelLength {
		return "", invalid("Label must be 1 to 64 characters long.")
	}
	for _, r := range label {
		if r < 0x20 || r == 0x7f {
			return "", invalid("Label must not contain control characters.")
		}
	}
	return label, nil
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}
