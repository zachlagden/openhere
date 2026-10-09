package sitepath

import (
	"strings"
	"unicode"
)

func FoldKey(p string) string {
	return strings.Map(canonicalCase, p)
}

func canonicalCase(r rune) rune {
	lowest := r
	for folded := unicode.SimpleFold(r); folded != r; folded = unicode.SimpleFold(folded) {
		if folded < lowest {
			lowest = folded
		}
	}
	return lowest
}

func CheckSet(paths []string) error {
	seen := make(map[string]string, len(paths))
	directories := make(map[string]string)
	for _, p := range paths {
		if err := Validate(p); err != nil {
			return err
		}
		key := FoldKey(p)
		if earlier, ok := seen[key]; ok {
			if earlier == p {
				return invalid("path %q appears twice", p)
			}
			return invalid("paths %q and %q differ only in letter case", earlier, p)
		}
		seen[key] = p
		for _, dir := range parentKeys(key) {
			directories[dir] = p
		}
	}
	for key, p := range seen {
		if other, ok := directories[key]; ok {
			return invalid("path %q is both a file and a directory of %q", p, other)
		}
	}
	return nil
}

func parentKeys(key string) []string {
	var parents []string
	for i := 0; i < len(key); i++ {
		if key[i] == '/' {
			parents = append(parents, key[:i])
		}
	}
	return parents
}
