package sitepath

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxPathBytes    = 1024
	MaxSegmentBytes = 255
)

var ErrInvalid = errors.New("invalid site path")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

func Validate(p string) error {
	if p == "" {
		return invalid("path is empty")
	}
	if len(p) > MaxPathBytes {
		return invalid("path is longer than %d bytes", MaxPathBytes)
	}
	if !utf8.ValidString(p) {
		return invalid("path is not valid UTF-8")
	}
	if p[0] == '/' {
		return invalid("path is absolute")
	}
	if err := checkRunes(p); err != nil {
		return err
	}
	for _, segment := range strings.Split(p, "/") {
		if err := checkSegment(segment); err != nil {
			return err
		}
	}
	return nil
}

func checkRunes(p string) error {
	for _, r := range p {
		switch {
		case r == '\\':
			return invalid("path contains a backslash")
		case r == 0:
			return invalid("path contains a NUL character")
		case unicode.IsControl(r):
			return invalid("path contains a control character")
		case unicode.Is(unicode.Cf, r):
			return invalid("path contains an invisible formatting character")
		}
	}
	return nil
}

func checkSegment(segment string) error {
	switch {
	case segment == "":
		return invalid("path contains an empty segment")
	case segment == ".":
		return invalid("path contains a . segment")
	case segment == "..":
		return invalid("path contains a .. segment")
	case len(segment) > MaxSegmentBytes:
		return invalid("a path segment is longer than %d bytes", MaxSegmentBytes)
	}
	return nil
}
