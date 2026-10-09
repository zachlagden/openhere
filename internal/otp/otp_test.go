package otp

import "testing"

func TestNewProducesDigitsOfTheRequestedLength(t *testing.T) {
	for range 200 {
		code, err := New(8)
		if err != nil {
			t.Fatal(err)
		}
		if !ValidShape(code, 8) {
			t.Fatalf("code %q has the wrong shape", code)
		}
	}
}

func TestValidShape(t *testing.T) {
	for code, want := range map[string]bool{"12345678": true, "00000000": true, "1234567": false, "123456789": false, "1234567a": false, "": false, "1234 678": false} {
		if got := ValidShape(code, 8); got != want {
			t.Errorf("ValidShape(%q) = %v, want %v", code, got, want)
		}
	}
}
