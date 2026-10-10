package analytics

import (
	"strings"
	"unicode/utf8"
)

const (
	MaxPathLength = 200
	PathRoot      = "/"
	PathNotFound  = "/(not found)"
	PathOther     = "/(other)"
)

func NormalisePath(requestPath string) string {
	if cut := strings.IndexAny(requestPath, "?#"); cut >= 0 {
		requestPath = requestPath[:cut]
	}
	cleaned := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == utf8.RuneError {
			return -1
		}
		return r
	}, requestPath)
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	if len(cleaned) <= MaxPathLength {
		return cleaned
	}
	cut := MaxPathLength
	for cut > 0 && !utf8.RuneStart(cleaned[cut]) {
		cut--
	}
	return cleaned[:cut]
}
