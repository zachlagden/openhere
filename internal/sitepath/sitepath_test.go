package sitepath

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateAccepts(t *testing.T) {
	valid := []string{
		"index.html",
		"assets/app.js",
		".well-known/security.txt",
		".env.example",
		"a/b/c/d/e.txt",
		"dir.with.dots/file",
		"naïve/ファイル.html",
		"UPPER/lower",
		"name with spaces.txt",
		"...",
		"a..b",
		strings.Repeat("a", MaxSegmentBytes),
		strings.Repeat("a/", 100) + "z",
	}
	for _, p := range valid {
		if err := Validate(p); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", p, err)
		}
	}
}

func TestValidateRejects(t *testing.T) {
	cases := map[string]string{
		"empty":                  "",
		"absolute":               "/etc/passwd",
		"dot segment":            "a/./b",
		"leading dot segment":    "./a",
		"only dot":               ".",
		"dotdot segment":         "a/../b",
		"leading dotdot":         "../a",
		"only dotdot":            "..",
		"trailing dotdot":        "a/..",
		"empty middle":           "a//b",
		"trailing slash":         "a/",
		"only slash":             "/",
		"backslash":              `a\b`,
		"backslash traversal":    `..\..\x`,
		"nul":                    "a\x00b",
		"nul at end":             "a.html\x00.png",
		"newline":                "a\nb",
		"tab":                    "a\tb",
		"carriage return":        "a\rb",
		"escape":                 "a\x1bb",
		"delete":                 "a\x7fb",
		"c1 control":             "a\u0085b",
		"right to left override": "a\u202eb",
		"zero width space":       "a\u200bb",
		"invalid utf8":           "a\xffb",
		"too long":               strings.Repeat("a/", 600),
		"long segment":           strings.Repeat("a", MaxSegmentBytes+1),
		"long multibyte segment": strings.Repeat("é", 128),
	}
	for name, p := range cases {
		err := Validate(p)
		if err == nil {
			t.Errorf("%s: Validate(%q) = nil, want an error", name, p)
			continue
		}
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: error %v does not wrap ErrInvalid", name, err)
		}
	}
}

func TestValidatePathLengthBoundary(t *testing.T) {
	segment := strings.Repeat("b", 204)
	atLimit := strings.Join([]string{segment, segment, segment, segment, segment}, "/")
	if len(atLimit) != MaxPathBytes {
		t.Fatalf("test path is %d bytes, want %d", len(atLimit), MaxPathBytes)
	}
	if err := Validate(atLimit); err != nil {
		t.Errorf("path at the limit rejected: %v", err)
	}
	if err := Validate(atLimit + "c"); err == nil {
		t.Error("path one byte over the limit accepted")
	}
}

func TestCheckSetRejects(t *testing.T) {
	cases := map[string][]string{
		"exact duplicate":          {"a.html", "a.html"},
		"case collision":           {"Index.html", "index.html"},
		"case collision in dir":    {"Assets/app.js", "assets/APP.js"},
		"unicode case collision":   {"É.txt", "é.txt"},
		"sharp s fold":             {"Straße.txt", "STRAßE.txt"},
		"file and directory":       {"docs", "docs/index.html"},
		"directory then file":      {"docs/index.html", "docs"},
		"nested file and dir":      {"a/b", "a/b/c/d"},
		"case-folded file and dir": {"Docs", "docs/x.html"},
		"one bad path":             {"ok.html", "../x"},
	}
	for name, paths := range cases {
		if err := CheckSet(paths); err == nil {
			t.Errorf("%s: CheckSet(%v) = nil, want an error", name, paths)
		} else if !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: error %v does not wrap ErrInvalid", name, err)
		}
	}
}

func TestCheckSetAccepts(t *testing.T) {
	sets := [][]string{
		{"index.html"},
		{"index.html", "about/index.html", "assets/a.js", "assets/b.js"},
		{".well-known/a", ".well-known/b", ".hidden"},
		{"a", "ab", "a.b", "a-b"},
		{"docs.html", "docs/index.html"},
	}
	for _, paths := range sets {
		if err := CheckSet(paths); err != nil {
			t.Errorf("CheckSet(%v) = %v, want nil", paths, err)
		}
	}
}

func TestFoldKey(t *testing.T) {
	if FoldKey("A/B.Js") != FoldKey("a/b.JS") {
		t.Error("ASCII case variants have different keys")
	}
	if FoldKey("a/b") == FoldKey("a/c") {
		t.Error("different paths share a key")
	}
}
