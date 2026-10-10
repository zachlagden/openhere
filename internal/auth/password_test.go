package auth

import (
	"context"
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword(context.Background(), testPassword, testParams)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=8,t=1,p=1$") {
		t.Errorf("hash %q does not carry its parameters", hash)
	}
	ok, err := VerifyPassword(context.Background(), testPassword, hash)
	if err != nil || !ok {
		t.Errorf("VerifyPassword(correct) = %v, %v; want true, nil", ok, err)
	}
	ok, err = VerifyPassword(context.Background(), "wrong password!!", hash)
	if err != nil || ok {
		t.Errorf("VerifyPassword(wrong) = %v, %v; want false, nil", ok, err)
	}
}

func TestHashesAreSalted(t *testing.T) {
	a, err := HashPassword(context.Background(), testPassword, testParams)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	b, err := HashPassword(context.Background(), testPassword, testParams)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if a == b {
		t.Error("two hashes of one password are identical")
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	cases := []string{
		"",
		"plain",
		"$argon2i$v=19$m=8,t=1,p=1$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA",
		"$argon2id$v=18$m=8,t=1,p=1$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA",
		"$argon2id$v=19$m=8,t=1,p=0$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA",
		"$argon2id$v=19$m=99999999,t=1,p=1$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA",
		"$argon2id$v=19$m=8,t=1,p=1$!!$aGFzaGhhc2hoYXNoaGFzaA",
	}
	for _, encoded := range cases {
		if ok, err := VerifyPassword(context.Background(), testPassword, encoded); err == nil || ok {
			t.Errorf("VerifyPassword(%q) = %v, %v; want an error", encoded, ok, err)
		}
	}
}

func TestNeedsRehash(t *testing.T) {
	hash, err := HashPassword(context.Background(), testPassword, testParams)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if NeedsRehash(hash, testParams) {
		t.Error("NeedsRehash reported current parameters as outdated")
	}
	stronger := testParams
	stronger.Time = 2
	if !NeedsRehash(hash, stronger) {
		t.Error("NeedsRehash did not notice changed parameters")
	}
	if !NeedsRehash("garbage", testParams) {
		t.Error("NeedsRehash did not flag a malformed hash")
	}
}

func TestPasswordPolicy(t *testing.T) {
	cases := map[string]bool{
		strings.Repeat("a", 11):  false,
		strings.Repeat("a", 12):  true,
		strings.Repeat("é", 12):  true,
		strings.Repeat("a", 128): true,
		strings.Repeat("a", 129): false,
		"":                       false,
	}
	for password, wantOK := range cases {
		err := ValidatePassword(password)
		if (err == nil) != wantOK {
			t.Errorf("ValidatePassword(%d chars) error = %v, want ok = %v", len([]rune(password)), err, wantOK)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	good, err := NormalizeEmail("  Alex@Example.COM ")
	if err != nil || good != "alex@example.com" {
		t.Errorf("NormalizeEmail = %q, %v", good, err)
	}
	for _, bad := range []string{"", "alex", "alex@", "@example.com", "alex@example", "Alex <alex@example.com>", "ålex@example.com", strings.Repeat("a", 250) + "@example.com"} {
		if _, err := NormalizeEmail(bad); err == nil {
			t.Errorf("NormalizeEmail(%q) accepted an invalid address", bad)
		}
	}
}

func TestNormalizeLabel(t *testing.T) {
	if label, err := NormalizeLabel("  laptop  "); err != nil || label != "laptop" {
		t.Errorf("NormalizeLabel = %q, %v", label, err)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("x", 65), "bad\x00label", "tab\tlabel"} {
		if _, err := NormalizeLabel(bad); err == nil {
			t.Errorf("NormalizeLabel(%q) accepted an invalid label", bad)
		}
	}
}

func TestSetupGate(t *testing.T) {
	gate, err := NewSetupGate()
	if err != nil {
		t.Fatalf("NewSetupGate: %v", err)
	}
	token := gate.Token()
	if token == "" || !gate.Check(token) {
		t.Fatal("gate rejects its own token")
	}
	if gate.Check("") || gate.Check(token+"x") || gate.Check("nope") {
		t.Error("gate accepted a wrong token")
	}
	gate.Discard()
	if gate.Check(token) || gate.Check("") {
		t.Error("gate accepted a token after Discard")
	}
	if ClosedSetupGate().Check("") {
		t.Error("closed gate accepted an empty token")
	}
}
