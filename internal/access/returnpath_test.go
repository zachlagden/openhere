package access

import (
	"strings"
	"testing"
)

func TestSafeReturnPathAcceptsSameHostPaths(t *testing.T) {
	good := []string{
		"/", "/about", "/docs/", "/docs/index.html", "/a/b/c?x=1&y=2", "/?q=//host", "/search?next=/other",
		"/caf%C3%A9", "/a%20b", "/.well-known/thing", "/a..b", "/..hidden", "/file.tar.gz", "/@name", "/a:b",
	}
	for _, raw := range good {
		got, ok := SafeReturnPath(raw)
		if !ok || got != raw {
			t.Errorf("SafeReturnPath(%q) = %q, %v; want it accepted unchanged", raw, got, ok)
		}
	}
}

func TestSafeReturnPathRejectsEverythingElse(t *testing.T) {
	bad := map[string]string{
		"empty":                     "",
		"relative":                  "about",
		"absolute url":              "https://evil.example/",
		"absolute url upper":        "HTTPS://evil.example/",
		"scheme only":               "javascript:alert(1)",
		"data url":                  "data:text/html,x",
		"protocol relative":         "//evil.example",
		"protocol relative path":    "//evil.example/path",
		"triple slash":              "///evil.example",
		"backslash start":           "/\\evil.example",
		"backslash inside":          "/a\\b",
		"double backslash":          "\\\\evil.example",
		"encoded backslash":         "/%5Cevil.example",
		"encoded backslash lower":   "/%5cevil.example",
		"encoded slash pair":        "/%2F/evil.example",
		"encoded slash pair lower":  "/%2f%2fevil.example",
		"encoded second slash":      "/%2Fevil.example/%2F%2Fx",
		"double encoded slash":      "/%252F%252Fevil.example",
		"double encoded backslash":  "/%255Cevil.example",
		"triple encoded backslash":  "/%25255Cevil.example",
		"tab":                       "/\t/evil.example",
		"newline":                   "/\n/evil.example",
		"carriage return":           "/\r/evil.example",
		"encoded tab":               "/%09/evil.example",
		"encoded newline":           "/%0a/evil.example",
		"encoded carriage return":   "/%0d/evil.example",
		"nul":                       "/a\x00b",
		"encoded nul":               "/a%00b",
		"space then slash":          " //evil.example",
		"dot segment":               "/a/./b",
		"dotdot segment":            "/a/../b",
		"leading dotdot":            "/../x",
		"encoded dotdot":            "/a/%2e%2e/b",
		"encoded dotdot upper":      "/a/%2E%2E/b",
		"encoded dot":               "/a/%2e/b",
		"double encoded dotdot":     "/a/%252e%252e/b",
		"encoded dotdot then slash": "/%2e%2e%2f%2e%2e%2fapi",
		"dotdot at the end":         "/a/..",
		"fragment":                  "/a#frag",
		"encoded fragment":          "/a%23frag",
		"too long":                  "/" + strings.Repeat("a", maxReturnLength),
		"backslash in query":        "/a?x=\\evil",
		"control char":              "/a\x1fb",
		"delete char":               "/a\x7fb",
	}
	for name, raw := range bad {
		if got, ok := SafeReturnPath(raw); ok {
			t.Errorf("%s: SafeReturnPath(%q) = %q, want a rejection", name, raw, got)
		}
	}
}
