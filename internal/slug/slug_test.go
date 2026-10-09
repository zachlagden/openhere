package slug

import (
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	valid := []string{"a", "7", "my-site", "site-2", "a--b", strings.Repeat("a", 63), "abc123"}
	invalid := []string{
		"", "-a", "a-", "My-Site", "my_site", "my.site", "my site", "é", strings.Repeat("a", 64),
		"0123456789abcdef0123456789abcdef",
	}
	for _, s := range valid {
		if err := Check(s); err != nil {
			t.Errorf("Check(%q) = %v, want nil", s, err)
		}
	}
	for _, s := range invalid {
		if err := Check(s); err == nil {
			t.Errorf("Check(%q) = nil, want error", s)
		}
	}
}

func TestFrom(t *testing.T) {
	tests := []struct {
		text string
		max  int
		want string
	}{
		{"Acme Ltd", 63, "acme-ltd"},
		{"  --Acme!!  Docs--  ", 63, "acme-docs"},
		{"Ünïcode", 63, "n-code"},
		{"!!!", 63, ""},
		{"abcdefghij", 5, "abcde"},
		{"ab cdefgh", 3, "ab"},
	}
	for _, tt := range tests {
		if got := From(tt.text, tt.max); got != tt.want {
			t.Errorf("From(%q, %d) = %q, want %q", tt.text, tt.max, got, tt.want)
		}
	}
}
