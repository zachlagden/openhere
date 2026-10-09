package validate

import (
	"errors"
	"strings"
	"testing"
)

func TestLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
		fails bool
	}{
		{name: "trims", input: "  Docs  ", max: 10, want: "Docs"},
		{name: "empty", input: "   ", max: 10, fails: true},
		{name: "too long", input: strings.Repeat("a", 11), max: 10, fails: true},
		{name: "counts runes", input: strings.Repeat("é", 10), max: 10, want: strings.Repeat("é", 10)},
		{name: "newline", input: "a\nb", max: 10, fails: true},
		{name: "delete character", input: "a\x7fb", max: 10, fails: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Line("Name", tt.input, tt.max)
			var invalid *Error
			if tt.fails {
				if !errors.As(err, &invalid) {
					t.Fatalf("err = %v, want *Error", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("Line = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestText(t *testing.T) {
	if got, err := Text("Description", "", 5); err != nil || got != "" {
		t.Errorf("empty text = %q, %v; want empty and no error", got, err)
	}
	if got, err := Text("Description", "a\nb\tc", 10); err != nil || got != "a\nb\tc" {
		t.Errorf("multiline = %q, %v", got, err)
	}
	if _, err := Text("Description", "a\x00", 10); err == nil {
		t.Error("control character accepted")
	}
	if _, err := Text("Description", "abcdef", 5); err == nil {
		t.Error("overlong text accepted")
	}
}
