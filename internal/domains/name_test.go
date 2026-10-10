package domains

import (
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/validate"
)

func TestNormaliseAcceptsValidNames(t *testing.T) {
	cases := map[string]string{
		"example.com":                   "example.com",
		"www.example.com":               "www.example.com",
		"WWW.Example.COM":               "www.example.com",
		"  example.com  ":               "example.com",
		"example.com.":                  "example.com",
		"a-b.example.co.uk":             "a-b.example.co.uk",
		"xn--bcher-kva.example":         "",
		"bücher.example.com":            "xn--bcher-kva.example.com",
		"BÜCHER.example.com":            "xn--bcher-kva.example.com",
		"xn--bcher-kva.example.com":     "xn--bcher-kva.example.com",
		"1.example.com":                 "1.example.com",
		"example.xn--p1ai":              "example.xn--p1ai",
		strings.Repeat("a", 63) + ".co": strings.Repeat("a", 63) + ".co",
		"deep.a.b.c.d.example.com":      "deep.a.b.c.d.example.com",
		"site.example.technology":       "site.example.technology",
		"例え.テスト.jp":                     "xn--r8jz45g.xn--zckzah.jp",
	}
	for input, want := range cases {
		got, err := Normalise(input)
		if want == "" {
			if err == nil {
				t.Errorf("Normalise(%q) = %q, want an error", input, got)
			}
			continue
		}
		if err != nil || got != want {
			t.Errorf("Normalise(%q) = %q, %v, want %q", input, got, err, want)
		}
	}
}

func TestNormaliseRejectsInvalidNames(t *testing.T) {
	long := strings.Repeat("a", 64)
	cases := map[string]string{
		"empty":                  "",
		"spaces only":            "   ",
		"single label":           "localhost",
		"single label with dot":  "example.",
		"ip v4":                  "192.0.2.1",
		"ip v4 trailing dot":     "192.0.2.1.",
		"ip v6":                  "2001:db8::1",
		"ip v6 bracketed":        "[2001:db8::1]",
		"numeric tld":            "example.123",
		"wildcard":               "*.example.com",
		"wildcard inside":        "a*.example.com",
		"port":                   "example.com:8080",
		"scheme":                 "https://example.com",
		"path":                   "example.com/path",
		"query":                  "example.com?x=1",
		"user info":              "alex@example.com",
		"fragment":               "example.com#top",
		"backslash":              "example.com\\x",
		"double trailing dot":    "example.com..",
		"leading dot":            ".example.com",
		"empty label":            "a..example.com",
		"leading hyphen":         "-a.example.com",
		"trailing hyphen":        "a-.example.com",
		"underscore":             "my_site.example.com",
		"challenge label":        "_selfhost-challenge.example.com",
		"space inside":           "my site.example.com",
		"control character":      "a\x00.example.com",
		"label too long":         long + ".example.com",
		"name too long":          strings.Repeat("a.", 120) + "com",
		"too long for challenge": strings.Repeat(strings.Repeat("a", 58)+".", 4) + "com",
		"bad punycode":           "xn--a.example.com",
		"percent encoded":        "example%2ecom",
		"local tld":              "printer.local",
		"internal tld":           "app.internal",
		"localhost tld":          "app.localhost",
		"test tld":               "app.test",
		"example tld":            "app.example",
		"invalid tld":            "app.invalid",
		"onion tld":              "abc.onion",
		"home arpa":              "nas.home.arpa",
		"lan tld":                "nas.lan",
		"one letter tld":         "example.c",
		"tld with digit":         "example.c0m",
		"newline injection":      "example.com\nHost: evil.com",
		"fullwidth dot ip":       "192。0。2。1",
		"decimal ip lookalike":   "3221225985",
		"hex ip":                 "0x7f.0.0.1",
	}
	for name, input := range cases {
		got, err := Normalise(input)
		if err == nil {
			t.Errorf("%s: Normalise(%q) = %q, want an error", name, input, got)
			continue
		}
		if _, ok := err.(*validate.Error); !ok {
			t.Errorf("%s: error %T is not a validation error", name, err)
		}
	}
}

func TestNormaliseIsIdempotent(t *testing.T) {
	first, err := Normalise("BÜCHER.Example.com.")
	if err != nil {
		t.Fatal(err)
	}
	second, err := Normalise(first)
	if err != nil || second != first {
		t.Errorf("second pass = %q, %v, want %q", second, err, first)
	}
}

func TestChallengeNameFitsInDNS(t *testing.T) {
	name := strings.Repeat(strings.Repeat("a", 62)+".", 3) + strings.Repeat("b", 41) + ".co"
	if len(name) != maxDomainLength {
		t.Fatalf("test name has %d characters, want %d", len(name), maxDomainLength)
	}
	got, err := Normalise(name)
	if err != nil {
		t.Fatalf("Normalise: %v", err)
	}
	if len(ChallengeLabel+"."+got) > maxNameLength {
		t.Errorf("challenge name has %d characters, over %d", len(ChallengeLabel+"."+got), maxNameLength)
	}
	if _, err := Normalise("c" + name); err == nil {
		t.Error("a name one character too long was accepted")
	}
}
