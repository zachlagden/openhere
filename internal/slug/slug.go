package slug

import (
	"strings"

	"github.com/zachlagden/openhere/internal/validate"
)

const (
	MaxLength = 63
	idLength  = 32
)

func Check(s string) error {
	if s == "" || len(s) > MaxLength {
		return validate.Invalid("Slug must be 1 to 63 characters long.")
	}
	if strings.HasPrefix(s, "-") || strings.HasSuffix(s, "-") {
		return validate.Invalid("Slug must not start or end with a hyphen.")
	}
	for i := 0; i < len(s); i++ {
		if !isSlugByte(s[i]) {
			return validate.Invalid("Slug may contain only lowercase letters, digits and hyphens.")
		}
	}
	if LooksLikeID(s) {
		return validate.Invalid("Slug must not look like an identifier.")
	}
	return nil
}

func LooksLikeID(s string) bool {
	if len(s) != idLength {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func From(text string, maxLength int) string {
	var out strings.Builder
	pendingHyphen := false
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if pendingHyphen && out.Len() > 0 {
				out.WriteByte('-')
			}
			pendingHyphen = false
			out.WriteRune(r)
		default:
			pendingHyphen = true
		}
	}
	result := out.String()
	if len(result) > maxLength {
		result = result[:maxLength]
	}
	return strings.TrimRight(result, "-")
}

func isSlugByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '-'
}
