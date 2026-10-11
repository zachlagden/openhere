package hosting

import (
	"path"
	"strings"
)

const (
	CacheFile      = "no-cache"
	CacheImmutable = "public, max-age=31536000, immutable"

	minHashLength = 8
	maxHashLength = 64
)

var hashedAssetRoots = map[string]bool{"_next": true, "assets": true, "static": true, "build": true}

func cacheControlFor(contentType, filePath string, private bool) string {
	switch {
	case private:
		return cachePrivateFile
	case !isHTML(contentType) && isContentHashed(filePath):
		return CacheImmutable
	default:
		return CacheFile
	}
}

func isContentHashed(filePath string) bool {
	segments := strings.Split(strings.Trim(filePath, "/"), "/")
	root := -1
	for i, segment := range segments[:len(segments)-1] {
		if hashedAssetRoots[segment] {
			root = i
			break
		}
	}
	if root < 0 {
		return false
	}
	last := len(segments) - 1
	for i := root + 1; i < last; i++ {
		if hasHashToken(segments[i]) {
			return true
		}
	}
	file := segments[last]
	return hasHashToken(strings.TrimSuffix(file, path.Ext(file)))
}

func hasHashToken(name string) bool {
	tokens := strings.FieldsFunc(name, isHashSeparator)
	if len(tokens) < 2 {
		return false
	}
	for _, token := range tokens[1:] {
		if isHashToken(token) {
			return true
		}
	}
	return false
}

func isHashSeparator(r rune) bool {
	return r == '.' || r == '-' || r == '_'
}

func isHashToken(token string) bool {
	if len(token) < minHashLength || len(token) > maxHashLength {
		return false
	}
	letter, digit := false, false
	for _, r := range token {
		switch {
		case r >= '0' && r <= '9':
			digit = true
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			letter = true
		default:
			return false
		}
	}
	return letter && digit
}
