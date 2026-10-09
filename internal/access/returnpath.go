package access

import (
	"net/url"
	"strings"
)

const (
	maxReturnLength = 2048
	maxDecodeRounds = 3
)

func SafeReturnPath(raw string) (string, bool) {
	if raw == "" || len(raw) > maxReturnLength || raw[0] != '/' {
		return "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.User != nil || parsed.Opaque != "" || parsed.Fragment != "" {
		return "", false
	}
	for _, form := range decodedForms(raw) {
		if !plainPath(form) {
			return "", false
		}
	}
	return raw, true
}

func decodedForms(raw string) []string {
	forms := []string{raw}
	current := raw
	for range maxDecodeRounds {
		next, err := url.PathUnescape(current)
		if err != nil || next == current {
			break
		}
		forms = append(forms, next)
		current = next
	}
	return forms
}

func plainPath(path string) bool {
	if strings.HasPrefix(path, "//") || strings.Contains(path, "\\") || strings.Contains(path, "#") {
		return false
	}
	for _, r := range path {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	pathPart, _, _ := strings.Cut(path, "?")
	for _, segment := range strings.Split(pathPart, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}
