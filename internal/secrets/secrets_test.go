package secrets

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testBox(t *testing.T) *Box {
	t.Helper()
	box, err := New(bytes.Repeat([]byte{7}, KeyBytes))
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func TestSealOpenRoundTrip(t *testing.T) {
	box := testBox(t)
	sealed, err := box.Seal("smtp-password-example")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, "smtp-password-example") {
		t.Fatal("the sealed value contains the plaintext")
	}
	plain, err := box.Open(sealed)
	if err != nil || plain != "smtp-password-example" {
		t.Fatalf("Open = %q, %v", plain, err)
	}
	again, err := box.Seal("smtp-password-example")
	if err != nil || again == sealed {
		t.Fatal("two seals of one value must differ")
	}
}

func TestOpenRejectsTamperingAndWrongKeys(t *testing.T) {
	box := testBox(t)
	sealed, err := box.Seal("value")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(sealed, sealPrefix))
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 1
	tampered := sealPrefix + base64.RawStdEncoding.EncodeToString(raw)
	if _, err := box.Open(tampered); !errors.Is(err, ErrOpen) {
		t.Errorf("tampered Open error = %v, want ErrOpen", err)
	}
	other, err := New(bytes.Repeat([]byte{9}, KeyBytes))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Open(sealed); !errors.Is(err, ErrOpen) {
		t.Errorf("wrong key error = %v, want ErrOpen", err)
	}
	for _, bad := range []string{"", "plain", "v1:", "v1:!!!", "v1:AAAA"} {
		if _, err := box.Open(bad); !errors.Is(err, ErrMalformed) {
			t.Errorf("Open(%q) error = %v, want ErrMalformed", bad, err)
		}
	}
}

func TestNewRejectsShortKeys(t *testing.T) {
	if _, err := New(make([]byte, 16)); err == nil {
		t.Fatal("a 16-byte key was accepted")
	}
}

func TestSignDependsOnPurposeAndMessage(t *testing.T) {
	box := testBox(t)
	a := box.Sign("one", "message")
	if !bytes.Equal(a, box.Sign("one", "message")) {
		t.Error("Sign is not deterministic")
	}
	if bytes.Equal(a, box.Sign("two", "message")) || bytes.Equal(a, box.Sign("one", "other")) {
		t.Error("Sign ignores its inputs")
	}
}

func TestParseKeyAcceptsBase64Forms(t *testing.T) {
	key := bytes.Repeat([]byte{0xfb}, KeyBytes)
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		parsed, err := ParseKey(" " + encoding.EncodeToString(key) + "\n")
		if err != nil || !bytes.Equal(parsed, key) {
			t.Errorf("ParseKey rejected a valid form: %v", err)
		}
	}
	for _, bad := range []string{"", "short", base64.StdEncoding.EncodeToString(make([]byte, 16))} {
		if _, err := ParseKey(bad); err == nil {
			t.Errorf("ParseKey(%q) succeeded", bad)
		}
	}
}

func TestResolveCreatesAndReusesKeyFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	first, err := Resolve(nil, dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, KeyFileName)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("key file mode = %v, want 0600", info.Mode().Perm())
	}
	sealed, err := first.Seal("value")
	if err != nil {
		t.Fatal(err)
	}
	second, err := Resolve(nil, dir)
	if err != nil {
		t.Fatal(err)
	}
	if plain, err := second.Open(sealed); err != nil || plain != "value" {
		t.Fatalf("the reloaded key cannot open earlier values: %v", err)
	}
}

func TestResolvePrefersTheEnvironmentKey(t *testing.T) {
	dir := t.TempDir()
	box, err := Resolve(bytes.Repeat([]byte{3}, KeyBytes), dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := box.Seal("x"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, KeyFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Error("a key file was created although SECRET_KEY was set")
	}
}

func TestResolveRejectsACorruptKeyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, KeyFileName), []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(nil, dir); err == nil {
		t.Fatal("a corrupt key file was accepted")
	}
}
